package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"port-mapping-demo/pkg/logger"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"adminApi/userDeviceCtl"

	"github.com/ghp3000/logs"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var unifiedKey string
var deviceCache sync.Map // map[uint64]*DeviceCommandInfo

// seqCounter 全局原子递增计数器，用于自动生成请求 Seq
// 初始值基于当前秒级时间戳，保证重启后不重复
var seqCounter = func() *atomic.Int64 {
	c := &atomic.Int64{}
	c.Store(time.Now().Unix())
	return c
}()

// nextSeq 生成下一个唯一 Seq 值
func nextSeq() int {
	return int(seqCounter.Add(1))
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

const (
	wsWriteWait  = 10 * time.Second
	wsPongWait   = 120 * time.Second
	wsPingPeriod = (wsPongWait * 9) / 10
)

// ClientManager manages connected WebSocket clients
type ClientManager struct {
	clients map[*websocket.Conn]*ClientInfo
	lock    sync.RWMutex
}

type ClientInfo struct {
	mu            *sync.Mutex
	logSubscribed bool
}

var unifiedClientManager = ClientManager{
	clients: make(map[*websocket.Conn]*ClientInfo),
}

func (manager *ClientManager) register(ws *websocket.Conn, mu *sync.Mutex) {
	manager.lock.Lock()
	defer manager.lock.Unlock()
	manager.clients[ws] = &ClientInfo{
		mu:            mu,
		logSubscribed: false,
	}
}

func (manager *ClientManager) unregister(ws *websocket.Conn) {
	manager.lock.Lock()
	defer manager.lock.Unlock()
	if _, ok := manager.clients[ws]; ok {
		delete(manager.clients, ws)
	}
}

func (manager *ClientManager) broadcast(res *UnifiedResponse) {
	manager.lock.RLock()
	defer manager.lock.RUnlock()
	for ws, info := range manager.clients {
		// Filter log stream: only send if client is subscribed
		if res.Type == "LogStream" && !info.logSubscribed {
			continue
		}

		// Send asynchronously to avoid blocking
		go func(w *websocket.Conn, m *sync.Mutex) {
			sendWSResponse(w, m, res)
		}(ws, info.mu)
	}
}

func (manager *ClientManager) setLogSubscription(ws *websocket.Conn, subscribed bool) {
	manager.lock.Lock()
	defer manager.lock.Unlock()
	if info, ok := manager.clients[ws]; ok {
		info.logSubscribed = subscribed
	}
}

// BroadcastToUnifiedClients is the exported function to send messages to all clients
func BroadcastToUnifiedClients(res *UnifiedResponse) {
	unifiedClientManager.broadcast(res)
}

// UnifiedWSHandler handles WebSocket connections for unified requests
func UnifiedWSHandler(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logs.Error("Failed to upgrade websocket: %v", err)
		return
	}
	defer ws.Close()

	clientAddr := c.ClientIP()
	logger.LogInfo("[Unified] WS connected: client=%s", clientAddr)

	// Use a mutex to ensure thread-safe writing to the websocket
	var writeMutex sync.Mutex

	// Register client
	unifiedClientManager.register(ws, &writeMutex)
	defer unifiedClientManager.unregister(ws)

	// Keepalive: extend read deadline on pong, and periodically ping.
	ws.SetReadDeadline(time.Now().Add(wsPongWait))
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	pingTicker := time.NewTicker(wsPingPeriod)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-pingTicker.C:
				writeMutex.Lock()
				_ = ws.SetWriteDeadline(time.Now().Add(wsWriteWait))
				if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
					writeMutex.Unlock()
					logger.LogError("[Unified] WS ping error: client=%s err=%v", clientAddr, err)
					logs.Error("WebSocket ping error: %v", err)
					return
				}
				writeMutex.Unlock()
			case <-done:
				return
			}
		}
	}()
	defer func() {
		close(done)
		pingTicker.Stop()
	}()

	logger.SetLogBroadcaster(func(logLine string) {
		BroadcastToUnifiedClients(&UnifiedResponse{
			Type: "LogStream",
			Code: 200,
			Data: logLine,
		})
	})

	for {
		// Read message
		_, message, err := ws.ReadMessage()
		if err != nil {
			if ce, ok := err.(*websocket.CloseError); ok {
				logger.LogError("[Unified] WS closed: client=%s code=%d text=%s", clientAddr, ce.Code, ce.Text)
			} else {
				logger.LogError("[Unified] WS read error: client=%s err=%v", clientAddr, err)
			}
			logs.Error("WebSocket read error: %v", err)
			break
		}

		logger.LogInfo("[Unified] WS recv: client=%s bytes=%d payload=%s", clientAddr, len(message), previewPayload(message))

		go func(msg []byte) {
			var req UnifiedRequest
			if err := json.Unmarshal(msg, &req); err != nil {
				logger.LogError("[Unified] Invalid JSON: client=%s err=%v payload=%s", clientAddr, err, previewPayload(msg))
				sendWSResponse(ws, &writeMutex, &UnifiedResponse{
					Code: 400,
					Msg:  "Invalid JSON format",
				})
				return
			}

			// Log processing for non-heartbeat requests to avoid spam
			isHeartbeat := req.Type == "Ping" || req.Type == "ping" || req.Type == "Heartbeat" || req.Type == "heartbeat"
			if !isHeartbeat {
				logger.LogInfo("[Unified] Processing request: Type=%s, Seq=%d, Data=%+v", req.Type, req.Seq, req.Data)
			}
			res, err := HandleUnifiedRequest(context.Background(), &req, ws)
			if err != nil {
				logger.LogError("[Unified] Request failed: Type=%s, Seq=%d, Error=%v", req.Type, req.Seq, err)
				// Should have been handled inside, but just in case
				res = &UnifiedResponse{
					Type: req.Type,
					Seq:  req.Seq,
					Code: 500,
					Msg:  err.Error(),
				}
			} else {
				if !isHeartbeat {
					logger.LogInfo("[Unified] Request success: Type=%s, Seq=%d", req.Type, req.Seq)
				}
			}

			sendWSResponse(ws, &writeMutex, res)
		}(message)
	}
}

func sendWSResponse(ws *websocket.Conn, mu *sync.Mutex, res *UnifiedResponse) {
	mu.Lock()
	defer mu.Unlock()
	// Prevent infinite loop: do not log LogStream responses
	// Also suppress Heartbeat/Ping responses to avoid spam
	isHeartbeat := res.Type == "Ping" || res.Type == "ping" || res.Type == "Heartbeat" || res.Type == "heartbeat"
	if res.Type != "LogStream" && !isHeartbeat {
		// Log detailed response data for debugging
		logger.LogInfo("[Unified] Sending response: Type=%s, Seq=%d, Code=%d, Data=%+v", res.Type, res.Seq, res.Code, res.Data)
	}
	_ = ws.SetWriteDeadline(time.Now().Add(wsWriteWait))
	if err := ws.WriteJSON(res); err != nil {
		logger.LogError("[Unified] WS write error: Type=%s, Seq=%d, err=%v", res.Type, res.Seq, err)
		logs.Error("WebSocket write error: %v", err)
	}
}

// UnifiedRequest defines the structure for all requests
type UnifiedRequest struct {
	Type   string      `json:"type"`
	Token  string      `json:"token,omitempty"`
	Host   string      `json:"host,omitempty"` // 云服务器地址，如 minio.accjs.cn
	Seq    int         `json:"seq"`
	Data   interface{} `json:"data,omitempty"`
	FuncId int         `json:"funcId,omitempty"` // For GetTaskStatus
	Req    bool        `json:"req,omitempty"`    // For Changephones
}

// UnifiedResponse defines the structure for all responses
type UnifiedResponse struct {
	Type string      `json:"type"`
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
	Seq  int         `json:"seq,omitempty"`
}

// HandleUnifiedRequestHTTP is a wrapper for HTTP requests (which don't have a WebSocket connection)
func HandleUnifiedRequestHTTP(ctx context.Context, req *UnifiedRequest) (*UnifiedResponse, error) {
	// For HTTP requests, ws is nil
	return HandleUnifiedRequest(ctx, req, nil)
}

// HandleUnifiedRequest handles all incoming unified requests
func HandleUnifiedRequest(ctx context.Context, req *UnifiedRequest, ws *websocket.Conn) (*UnifiedResponse, error) {
	// SEQ 自动生成：调用方无需手动填 Seq，底层统一分配
	// Seq=0 时自动生成，非0保留（兼容外部调用）
	if req.Seq == 0 {
		req.Seq = nextSeq()
	}

	res := &UnifiedResponse{
		Type: req.Type,
		Seq:  req.Seq,
		Code: 200,
		Msg:  "Success",
	}

	var err error

	switch req.Type {
	case "Login":
		err = handleLogin(req.Token, req.Host)
	case "Ping", "ping", "Heartbeat", "heartbeat":
		res.Msg = "pong"
	case "GetDeviceList":
		res.Data, err = handleGetDeviceList()
	case "Changephones":
		res.Data, err = handleChangePhones(req.Data)
	case "getAppList":
		res.Data, err = handleGetAppList(req.Data)
	case "getTaskStatus":
		res.Data, err = handleGetTaskStatus(req.Data)
	case "downLoadInstallApp":
		res.Data, err = handleDownLoadInstallApp(req.Data)
	case "getDownloadProgress":
		res.Data, err = handleGetDownloadProgress(req.Data)
	case "hideApp":
		res.Data, err = handleHideApp(req.Data)
	case "setSocket5":
		res.Data, err = handleSetSocket5(req.Data)
	case "startLogStream":
		// Enable log streaming for this client
		unifiedClientManager.setLogSubscription(ws, true)
		res.Msg = "Log stream started"
	case "stopLogStream":
		// Disable log streaming for this client
		unifiedClientManager.setLogSubscription(ws, false)
		res.Msg = "Log stream stopped"
	case "getSocket5":
		res.Data, err = handleGetSocket5(req.Data)
	case "getS5outLine":
		res.Data, err = handleGetS5outLine(req.Data)
	case "getUserFiles":
		res.Data, err = handleGetUserFiles(req.Data)
	case "setLocation":
		res.Data, err = handleSetLocation(req.Data)
	case "execShell":
		res.Data, err = handleExecShell(req.Data)
	case "startApp":
		res.Data, err = handleStartApp(req.Data)
	case "getDeviceDetail":
		res.Data, err = handleGetDeviceDetail(req.Data)
	case "getDeviceStatus":
		res.Data, err = handleGetDeviceStatus(req.Data)
	case "getRoot":
		res.Data, err = handleGetRoot(req.Data)
	case "screenshot":
		res.Data, err = handleScreenshot(req.Data)
	case "backupApp":
		res.Data, err = handleBackupApp(req.Data)
	case "getBackupAppStatus":
		res.Data, err = handleGetBackupAppStatus(req.Data)
	case "getBackupList":
		res.Data, err = handleGetBackupList(req.Data)
	case "restoreApp":
		res.Data, err = handleRestoreApp(req.Data)
	case "getRestoreAppStatus":
		res.Data, err = handleGetRestoreAppStatus(req.Data)
	case "changeOldOsReq":
		res.Data, err = handleChangeOldOsReq(req.Data)
	case "getChangeOsList":
		res.Data, err = handleGetChangeOsList(req.Data)
	default:
		res.Code = 404
		res.Msg = "Unknown request type"
	}

	if err != nil {
		res.Code = 500
		res.Msg = err.Error()
	}

	return res, nil
}

func handleLogin(token string, host string) error {
	if token == "" {
		logger.LogError("[Unified] Login failed: empty token")
		return fmt.Errorf("token is required")
	}
	unifiedKey = token // Save token for later use
	return EnsureLogin(token, host)
}

func previewPayload(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	const maxLen = 512
	s := strings.TrimSpace(string(b))
	if len(s) > maxLen {
		return s[:maxLen] + "...(truncated)"
	}
	return s
}

// ensureGlobalApi 确保 JpyApiAgent 已登录且 adminApi 可用
func ensureGlobalApi() error {
	if GetGlobalApi() != nil {
		return nil
	}
	if unifiedKey == "" {
		return fmt.Errorf("not logged in")
	}
	// 尝试自动重新登录
	logger.LogInfo("[Unified] globalApi is nil, attempting auto-relogin with existing token")
	return handleLogin(unifiedKey, "")
}

// findDeviceInfoWithCache 带缓存的设备信息查找
// 优先从本地缓存获取，缓存未命中时先尝试 JpyApiAgent Core 缓存，
// 最后通过集控平台 API 刷新全量设备列表
func findDeviceInfoWithCache(deviceId uint64) (*DeviceCommandInfo, error) {
	// 1. 本地 sync.Map 缓存
	if val, ok := deviceCache.Load(deviceId); ok {
		return val.(*DeviceCommandInfo), nil
	}

	// 2. 尝试从 JpyApiAgent Core 内存缓存获取（无需 API 调用）
	if info, ok := findDeviceInfoFromCore(deviceId); ok {
		deviceCache.Store(deviceId, info)
		return info, nil
	}

	// 3. 缓存未命中，通过集控平台 API 刷新
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}

	api := GetGlobalApi()
	if api == nil {
		return nil, fmt.Errorf("globalApi 不可用")
	}

	res, err := api.UserDeviceCtl.GetUserDeviceList(&userDeviceCtl.GetUserDeviceListReq{
		PageNum:  1,
		PageSize: 999999,
	})
	if err != nil {
		return nil, fmt.Errorf("获取设备列表失败: %v", err)
	}

	var found *DeviceCommandInfo
	for _, d := range res.Records {
		info := &DeviceCommandInfo{
			DeviceId:            uint64(d.DeviceInfo.DeviceId),
			TbProxyId:           uint64(d.DeviceInfo.TbProxyId),
			TbYunJiUserDeviceId: uint64(d.TbYunJiUserDeviceId),
		}
		deviceCache.Store(info.DeviceId, info)

		if info.DeviceId == deviceId {
			found = info
		}
	}

	if found != nil {
		return found, nil
	}

	return nil, fmt.Errorf("设备 %d 未找到", deviceId)
}
