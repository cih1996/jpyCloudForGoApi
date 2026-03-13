package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"port-mapping-demo/pkg/logger"
	"strings"
	"sync"
	"time"

	"adminApi/changeOsCtl"
	"adminApi/tbFileCtl"
	"adminApi/userDeviceCtl"

	"github.com/ghp3000/logs"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var unifiedKey string
var deviceCache sync.Map // map[uint64]*DeviceCommandInfo

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

func handleGetDeviceList() (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}

	// 获取全部设备列表（分页设大值）
	ret, err := GetGlobalApi().UserDeviceCtl.GetUserDeviceList(&userDeviceCtl.GetUserDeviceListReq{
		PageNum:  1,
		PageSize: 999999,
	})

	if err != nil {
		return nil, fmt.Errorf(err.Msg)
	}
	return ret, nil
}

// CheckDeviceOnline 检查设备是否在线（通过 API 接口）
func CheckDeviceOnline(deviceID int) (bool, error) {
	if err := ensureGlobalApi(); err != nil {
		return false, err
	}

	ret, err := GetGlobalApi().UserDeviceCtl.GetUserDeviceList(&userDeviceCtl.GetUserDeviceListReq{
		PageNum:  1,
		PageSize: 999999,
	})

	if err != nil {
		return false, fmt.Errorf(err.Msg)
	}

	for _, d := range ret.Records {
		if int(d.DeviceInfo.DeviceId) == deviceID {
			return d.DeviceInfo.Online, nil
		}
	}

	return false, fmt.Errorf("设备 %d 未找到", deviceID)
}

// Helper to convert map to struct if needed, or just pass through
// Since SendGenericCommandToDevice takes interface{}, we might need to process data
// specific to each command type if necessary.

// For Changephones (Type 3)
// Data: [{"deviceId":..., "type":"changeDevice", "func":1, "paramsAll":...}]
func handleChangePhones(data interface{}) (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}

	var reqs []*changeOsCtl.ChangeOsReq
	if err := json.Unmarshal(dataBytes, &reqs); err != nil {
		return nil, fmt.Errorf("invalid data format for ChangeOs: %v", err)
	}

	res, errPkg := GetGlobalApi().ChangeOsCtl.ChangeOs(reqs)
	if errPkg != nil {
		return nil, fmt.Errorf("%s", errPkg.Msg)
	}

	return parseChangeOsRes(res), nil
}

func handleGetAppList(data interface{}) (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}

	type TempReq struct {
		DeviceId uint64 `json:"deviceId"`
	}
	var tempReq TempReq
	if err := json.Unmarshal(dataBytes, &tempReq); err != nil {
		return nil, fmt.Errorf("invalid data format for GetAppList: %v", err)
	}

	if tempReq.DeviceId == 0 {
		return nil, fmt.Errorf("deviceId is required")
	}

	info, err := findDeviceInfoWithCache(tempReq.DeviceId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	// F=290 for Get App List
	payload := map[string]interface{}{}

	res, err := SendGenericCommandToDevice(unifiedKey, []DeviceCommandInfo{*info}, 290, payload, true, true, 15*time.Second)
	if err != nil {
		return nil, err
	}
	return processResponse(res), nil
}

func handleGetTaskStatus(data interface{}) (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}

	var req changeOsCtl.GetChangeOsStatusReq
	if err := json.Unmarshal(dataBytes, &req); err != nil {
		return nil, fmt.Errorf("invalid data format for GetTaskStatus: %v", err)
	}

	res, errPkg := GetGlobalApi().ChangeOsCtl.GetChangeOsStatus(req)
	if errPkg != nil {
		return nil, fmt.Errorf("%s", errPkg.Msg)
	}
	return parseGetChangeOsStatusRes(res), nil
}

// Helper to parse Data field in responses
func parseChangeOsRes(res []*changeOsCtl.ChangeOsRes) []map[string]interface{} {
	var finalRes []map[string]interface{}
	for _, s := range res {
		sMap := make(map[string]interface{})
		sBytes, _ := json.Marshal(s)
		_ = json.Unmarshal(sBytes, &sMap)

		if s.Data != "" {
			var dataObj interface{}
			if err := json.Unmarshal([]byte(s.Data), &dataObj); err == nil {
				sMap["dataObj"] = dataObj
			}
		}
		finalRes = append(finalRes, sMap)
	}
	return finalRes
}

func parseGetChangeOsStatusRes(res []*changeOsCtl.GetChangeOsStatusRes) []map[string]interface{} {
	var finalRes []map[string]interface{}
	for _, s := range res {
		sMap := make(map[string]interface{})
		sBytes, _ := json.Marshal(s)
		_ = json.Unmarshal(sBytes, &sMap)

		if s.Data != "" {
			var dataObj interface{}
			if err := json.Unmarshal([]byte(s.Data), &dataObj); err == nil {
				sMap["dataObj"] = dataObj
			}
		}
		finalRes = append(finalRes, sMap)
	}
	return finalRes
}

func handleDownLoadInstallApp(data interface{}) (interface{}, error) {
	// "data":{"devices":[21323,21043],"url":"...","install":true,...}
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid data format")
	}

	devicesInterface, ok := m["devices"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("devices list missing")
	}

	var deviceIds []DeviceCommandInfo
	for _, d := range devicesInterface {
		if did, ok := d.(float64); ok {
			info, err := findDeviceInfoWithCache(uint64(did))
			if err == nil {
				deviceIds = append(deviceIds, *info)
			}
		}
	}

	// Payload for F=293
	// Extract other fields from data
	payload := make(map[string]interface{})
	for k, v := range m {
		if k != "devices" {
			payload[k] = v
		}
	}
	// Ensure mandatory fields
	payload["receive"] = true

	// Use F=293 for Download/Install task
	res, err := SendGenericCommandToDevice(unifiedKey, deviceIds, 293, payload, true, true, 30*time.Second)
	if err != nil {
		return nil, err
	}
	return processResponse(res), nil
}

func handleGetDownloadProgress(data interface{}) (interface{}, error) {
	// Expected data: { "deviceId": 123, "id": "task_id" }
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid data format")
	}

	deviceIdVal, ok := m["deviceId"].(float64)
	if !ok {
		return nil, fmt.Errorf("deviceId missing or invalid")
	}
	deviceId := uint64(deviceIdVal)

	id, ok := m["id"].(string)
	if !ok || id == "" {
		return nil, fmt.Errorf("id (task id) missing or empty")
	}

	info, err := findDeviceInfoWithCache(deviceId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	payload := map[string]interface{}{
		"id": id,
	}

	// F=294 for Get Download Progress
	res, err := SendGenericCommandToDevice(unifiedKey, []DeviceCommandInfo{*info}, 294, payload, true, true, 15*time.Second)
	if err != nil {
		return nil, err
	}
	return processResponse(res), nil
}

func handleHideApp(data interface{}) (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}

	type TempReq struct {
		DeviceId    uint64 `json:"deviceId"`
		PackageName string `json:"packageName"`
		IsHide      bool   `json:"isHide"`
	}
	var tempReq TempReq
	if err := json.Unmarshal(dataBytes, &tempReq); err != nil {
		return nil, fmt.Errorf("invalid data format for HideApp: %v", err)
	}

	if tempReq.DeviceId == 0 {
		return nil, fmt.Errorf("deviceId is required")
	}
	if tempReq.PackageName == "" {
		return nil, fmt.Errorf("packageName is required")
	}

	info, err := findDeviceInfoWithCache(tempReq.DeviceId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	// Use DeviceId (int64)
	did := int64(info.DeviceId)
	req := changeOsCtl.HideAppReq{
		TbDeviceId:  &did,
		PackageName: &tempReq.PackageName,
		IsHide:      &tempReq.IsHide,
	}

	if errPkg := GetGlobalApi().ChangeOsCtl.HideApp(req); errPkg != nil {
		return nil, fmt.Errorf("%s", errPkg.Msg)
	}
	return "Success", nil
}

func handleSetSocket5(data interface{}) (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}

	type TempReq struct {
		DeviceId uint64 `json:"deviceId"`
		Id       uint64 `json:"id"`
		S5Url    string `json:"s5Url"`
		NOutSwID int    `json:"nOutSwID"`
		LineType int    `json:"lineType"`
	}
	var tempReq TempReq
	if err := json.Unmarshal(dataBytes, &tempReq); err != nil {
		return nil, fmt.Errorf("invalid data format for SetS5: %v", err)
	}

	if tempReq.NOutSwID == 0 {
		if tempReq.LineType == 1 {
			// If lineType is 1, default to 10006
			tempReq.NOutSwID = 11211
		} else if tempReq.LineType != 0 {
			// If lineType is not 0 and not 1, return error
			return nil, fmt.Errorf("unsupported line type: %d", tempReq.LineType)
		}
	}

	targetId := tempReq.DeviceId
	if targetId == 0 {
		targetId = tempReq.Id
	}
	if targetId == 0 {
		return nil, fmt.Errorf("deviceId is required")
	}

	info, err := findDeviceInfoWithCache(targetId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	tbId := int64(info.TbYunJiUserDeviceId)
	req := userDeviceCtl.SetS5Req{
		TbYunJiUserDeviceId: &tbId,
		S5Url:               &tempReq.S5Url,
		NOutSwID:            &tempReq.NOutSwID,
	}

	if errPkg := GetGlobalApi().UserDeviceCtl.SetS5(req); errPkg != nil {
		return nil, fmt.Errorf("%s", errPkg.Msg)
	}
	return nil, nil
}

func handleGetSocket5(data interface{}) (interface{}, error) {
	return nil, fmt.Errorf("use getS5outLine or check device list for S5 info")
}

func handleGetS5outLine(data interface{}) (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}

	type TempReq struct {
		DeviceId uint64 `json:"deviceId"`
		Id       uint64 `json:"id"`
	}
	var tempReq TempReq
	if err := json.Unmarshal(dataBytes, &tempReq); err != nil {
		return nil, fmt.Errorf("invalid data format for GetS5outLine: %v", err)
	}

	targetId := tempReq.DeviceId
	if targetId == 0 {
		targetId = tempReq.Id
	}
	if targetId == 0 {
		return nil, fmt.Errorf("deviceId is required")
	}

	info, err := findDeviceInfoWithCache(targetId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	tbId := int64(info.TbYunJiUserDeviceId)
	req := userDeviceCtl.GetOutLineReq{
		TbYunJiUserDeviceId: &tbId,
	}

	res, errPkg := GetGlobalApi().UserDeviceCtl.GetOutLine(req)
	if errPkg != nil {
		return nil, fmt.Errorf("%s", errPkg.Msg)
	}
	return res, nil
}

func handleGetUserFiles(data interface{}) (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}
	var req tbFileCtl.ListReq
	if err := json.Unmarshal(dataBytes, &req); err != nil {
		return nil, fmt.Errorf("invalid data format for GetUserFiles: %v", err)
	}

	// 1. 获取文件下载基础 URL
	baseUrl, errPkg := GetGlobalApi().TbFileCtl.GetDownloadUrl()
	if errPkg != nil {
		return nil, fmt.Errorf("failed to get download url: %s", errPkg.Msg)
	}

	// 2. 获取文件列表
	res, errPkg := GetGlobalApi().TbFileCtl.List(req)
	if errPkg != nil {
		return nil, fmt.Errorf("%s", errPkg.Msg)
	}

	// 3. Combine to form full URL
	var finalRes []map[string]interface{}
	for _, item := range res {
		itemMap := make(map[string]interface{})
		itemBytes, _ := json.Marshal(item)
		_ = json.Unmarshal(itemBytes, &itemMap)

		itemMap["url"] = baseUrl + "/" + item.Hash
		finalRes = append(finalRes, itemMap)
	}

	return finalRes, nil
}

func handleSetLocation(data interface{}) (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}

	type SetLocationItem struct {
		DeviceId uint64      `json:"deviceId"`
		Lat      interface{} `json:"lat"`
		Lng      interface{} `json:"lng"`
	}

	var items []SetLocationItem
	// Support both array and single object
	if err := json.Unmarshal(dataBytes, &items); err != nil {
		var item SetLocationItem
		if err2 := json.Unmarshal(dataBytes, &item); err2 == nil {
			items = append(items, item)
		} else {
			return nil, fmt.Errorf("invalid data format: %v", err)
		}
	}

	var results []interface{}

	for _, item := range items {
		if item.DeviceId == 0 {
			results = append(results, map[string]interface{}{"deviceId": 0, "error": "deviceId is required"})
			continue
		}

		info, err := findDeviceInfoWithCache(item.DeviceId)
		if err != nil {
			results = append(results, map[string]interface{}{"deviceId": item.DeviceId, "error": fmt.Sprintf("device not found: %v", err)})
			continue
		}

		did := int64(info.DeviceId)
		latStr := fmt.Sprintf("%v", item.Lat)
		lngStr := fmt.Sprintf("%v", item.Lng)

		req := changeOsCtl.SetLocationReq{
			TbDeviceId: &did,
			Latitude:   &latStr,
			Longitude:  &lngStr,
		}

		errPkg := GetGlobalApi().ChangeOsCtl.SetLocation(req)
		if errPkg != nil {
			results = append(results, map[string]interface{}{"deviceId": item.DeviceId, "error": errPkg.Msg})
		} else {
			results = append(results, map[string]interface{}{"deviceId": item.DeviceId, "result": "success"})
		}
	}

	return results, nil
}

func handleGetRoot(data interface{}) (interface{}, error) {
	// Expected data: { "deviceId": 123, "pkg": "com.android.shell" }
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid data format")
	}

	deviceIdVal, ok := m["deviceId"].(float64)
	if !ok {
		return nil, fmt.Errorf("deviceId missing or invalid")
	}
	deviceId := uint64(deviceIdVal)

	// pkg is optional, defaults to "com.android.shell" if not present
	pkgName := "com.android.shell"
	if v, ok := m["pkg"].(string); ok && v != "" {
		pkgName = v
	}

	info, err := findDeviceInfoWithCache(deviceId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	payload := map[string]interface{}{
		"pkg": pkgName,
	}

	// F=516 for Get Root
	res, err := SendGenericCommandToDevice(unifiedKey, []DeviceCommandInfo{*info}, 516, payload, true, true, 15*time.Second)
	if err != nil {
		return nil, err
	}
	return processResponse(res), nil
}

func handleExecShell(data interface{}) (interface{}, error) {
	// Expected data: { "deviceId": 123, "shell": "ls -l" }
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid data format")
	}

	deviceIdVal, ok := m["deviceId"].(float64)
	if !ok {
		return nil, fmt.Errorf("deviceId missing or invalid")
	}
	deviceId := uint64(deviceIdVal)

	shellCmd, ok := m["shell"].(string)
	if !ok || shellCmd == "" {
		return nil, fmt.Errorf("shell command missing or empty")
	}

	info, err := findDeviceInfoWithCache(deviceId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	payload := map[string]interface{}{
		"shell": shellCmd,
	}

	// F=289 for Shell Execution
	res, err := SendGenericCommandToDevice(unifiedKey, []DeviceCommandInfo{*info}, 289, payload, true, true, 15*time.Second)
	if err != nil {
		return nil, err
	}
	return processResponse(res), nil
}

func handleStartApp(data interface{}) (interface{}, error) {
	// Expected data: { "deviceId": 123, "packageName": "com.example" }
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid data format")
	}

	deviceIdVal, ok := m["deviceId"].(float64)
	if !ok {
		return nil, fmt.Errorf("deviceId missing or invalid")
	}
	deviceId := uint64(deviceIdVal)

	pkgName, ok := m["packageName"].(string)
	if !ok || pkgName == "" {
		return nil, fmt.Errorf("packageName missing or empty")
	}

	info, err := findDeviceInfoWithCache(deviceId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	payload := map[string]interface{}{
		"packageName": pkgName,
	}

	// F=291 for Start App
	res, err := SendGenericCommandToDevice(unifiedKey, []DeviceCommandInfo{*info}, 291, payload, true, true, 15*time.Second)
	if err != nil {
		return nil, err
	}
	return processResponse(res), nil
}

func handleGetDeviceDetail(data interface{}) (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}

	type TempReq struct {
		DeviceId uint64 `json:"deviceId"`
	}
	var tempReq TempReq
	if err := json.Unmarshal(dataBytes, &tempReq); err != nil {
		return nil, fmt.Errorf("invalid data format for GetDeviceDetail: %v", err)
	}

	if tempReq.DeviceId == 0 {
		return nil, fmt.Errorf("deviceId is required")
	}

	info, err := findDeviceInfoWithCache(tempReq.DeviceId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	// F=4 for Get Device Detail
	payload := map[string]interface{}{}

	res, err := SendGenericCommandToDevice(unifiedKey, []DeviceCommandInfo{*info}, 4, payload, true, true, 15*time.Second)
	if err != nil {
		return nil, err
	}
	return processResponse(res), nil
}

func handleGetDeviceStatus(data interface{}) (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}

	type TempReq struct {
		DeviceId uint64 `json:"deviceId"`
	}
	var tempReq TempReq
	if err := json.Unmarshal(dataBytes, &tempReq); err != nil {
		return nil, fmt.Errorf("invalid data format for GetDeviceStatus: %v", err)
	}

	if tempReq.DeviceId == 0 {
		return nil, fmt.Errorf("deviceId is required")
	}

	info, err := findDeviceInfoWithCache(tempReq.DeviceId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	// F=6 for Get Device Status
	payload := map[string]interface{}{}

	res, err := SendGenericCommandToDevice(unifiedKey, []DeviceCommandInfo{*info}, 6, payload, true, true, 15*time.Second)
	if err != nil {
		return nil, err
	}
	return processResponse(res), nil
}

type DeviceCommandRequest struct {
	DeviceId  uint64      `json:"deviceId"`
	Type      string      `json:"type"`
	Func      int         `json:"func"`
	ParamsAll interface{} `json:"paramsAll"`
}

func processDeviceCommands(data interface{}) (interface{}, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal failed: %v", err)
	}

	var commands []DeviceCommandRequest
	if err := json.Unmarshal(dataBytes, &commands); err != nil {
		return nil, fmt.Errorf("unmarshal failed: %v", err)
	}

	var results []interface{}

	for _, cmd := range commands {
		info, err := findDeviceInfoWithCache(cmd.DeviceId)
		if err != nil {
			logs.Error("Device %d not found: %v", cmd.DeviceId, err)
			results = append(results, map[string]interface{}{"deviceId": cmd.DeviceId, "error": err.Error()})
			continue
		}

		// Send command
		res, err := SendGenericCommandToDevice(unifiedKey, []DeviceCommandInfo{*info}, uint16(cmd.Func), cmd.ParamsAll, true, true, 15*time.Second)
		if err != nil {
			logs.Error("Failed to send command to device %d: %v", cmd.DeviceId, err)
			results = append(results, map[string]interface{}{"deviceId": cmd.DeviceId, "error": err.Error()})
		} else {
			results = append(results, map[string]interface{}{"deviceId": cmd.DeviceId, "result": processResponse(res)})
		}
	}

	return results, nil
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
