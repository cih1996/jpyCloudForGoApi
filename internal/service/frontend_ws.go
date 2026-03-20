package service

import (
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"port-mapping-demo/internal/rpa"
	"port-mapping-demo/pkg/logger"
	"sync"
	"time"

	"github.com/ghp3000/logs"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// FrontendWSManager 前端 WebSocket 管理器
type FrontendWSManager struct {
	clients    map[*websocket.Conn]*FrontendClient
	lock       sync.RWMutex
	lastHashes map[string][32]byte // 每种数据类型的上次 hash
	hashLock   sync.Mutex
}

// FrontendClient 前端客户端信息
type FrontendClient struct {
	mu         *sync.Mutex
	subscribes map[string]bool // 订阅的数据类型
}

// FrontendMessage 前端 WS 消息格式
type FrontendMessage struct {
	Type string      `json:"type"` // subscribe, unsubscribe, request
	Data interface{} `json:"data"`
}

// FrontendPush 推送给前端的数据格式
type FrontendPush struct {
	Type string      `json:"type"` // devices, rpa_status, etc.
	Data interface{} `json:"data"`
	Time int64       `json:"time"` // 时间戳
}

// DeviceWithLocal 带本地状态的设备信息
type DeviceWithLocal struct {
	Raw   interface{}       `json:"raw"`
	Local *LocalDeviceState `json:"_local,omitempty"`
}

// LocalDeviceState 本地设备状态
type LocalDeviceState struct {
	RpaRunning     bool    `json:"rpaRunning"`
	RpaID          uint    `json:"rpaId,omitempty"`
	RpaName        string  `json:"rpaName,omitempty"`
	RpaStatus      string  `json:"rpaStatus,omitempty"`
	RpaStep        int     `json:"rpaStep,omitempty"`
	RpaStepName    string  `json:"rpaStepName,omitempty"`
	RpaTotalSteps  int     `json:"rpaTotalSteps,omitempty"`
	RpaLoopCount   int     `json:"rpaLoopCount,omitempty"`
	RpaLastError   string  `json:"rpaLastError,omitempty"`
	RpaSubStep     int     `json:"rpaSubStep,omitempty"`
	RpaSubStepName string  `json:"rpaSubStepName,omitempty"`
	RpaTotalTime   int64   `json:"rpaTotalTime"`
	RpaLoopStartAt *string `json:"rpaLoopStartAt,omitempty"`
}

var frontendWSManager = &FrontendWSManager{
	clients:    make(map[*websocket.Conn]*FrontendClient),
	lastHashes: make(map[string][32]byte),
}

var frontendUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// FrontendWSHandler 前端统一 WebSocket 处理器
func FrontendWSHandler(c *gin.Context) {
	ws, err := frontendUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logs.Error("[FrontendWS] 升级失败: %v", err)
		return
	}
	defer ws.Close()

	mu := &sync.Mutex{}
	client := &FrontendClient{
		mu:         mu,
		subscribes: make(map[string]bool),
	}

	frontendWSManager.register(ws, client)
	defer frontendWSManager.unregister(ws)

	logs.Info("[FrontendWS] 客户端连接")

	ws.SetReadDeadline(time.Now().Add(wsPongWait))
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	go frontendPingLoop(ws, mu)

	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logs.Error("[FrontendWS] 读取错误: %v", err)
			}
			break
		}

		var msg FrontendMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			logs.Error("[FrontendWS] 解析消息失败: %v", err)
			continue
		}

		frontendWSManager.handleMessage(ws, client, &msg)
	}

	logs.Info("[FrontendWS] 客户端断开")
}

func frontendPingLoop(ws *websocket.Conn, mu *sync.Mutex) {
	ticker := time.NewTicker(wsPingPeriod)
	defer ticker.Stop()

	for range ticker.C {
		mu.Lock()
		err := ws.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(wsWriteWait))
		mu.Unlock()
		if err != nil {
			return
		}
	}
}

func (m *FrontendWSManager) register(ws *websocket.Conn, client *FrontendClient) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.clients[ws] = client
}

func (m *FrontendWSManager) unregister(ws *websocket.Conn) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.clients, ws)
}

func (m *FrontendWSManager) handleMessage(ws *websocket.Conn, client *FrontendClient, msg *FrontendMessage) {
	switch msg.Type {
	case "subscribe":
		if dataType, ok := msg.Data.(string); ok {
			client.subscribes[dataType] = true
			logs.Info("[FrontendWS] 订阅: %s", dataType)
			// 订阅时立即推一次全量
			m.pushDataToClient(ws, client, dataType)
		}

	case "unsubscribe":
		if dataType, ok := msg.Data.(string); ok {
			delete(client.subscribes, dataType)
			logs.Info("[FrontendWS] 取消订阅: %s", dataType)
		}

	case "request":
		// 一次性拉取（首次加载用）
		if dataType, ok := msg.Data.(string); ok {
			m.pushDataToClient(ws, client, dataType)
		}

	default:
		logs.Warn("[FrontendWS] 未知消息类型: %s", msg.Type)
	}
}

// pushDataToClient 推送数据给单个客户端（无条件推送）
func (m *FrontendWSManager) pushDataToClient(ws *websocket.Conn, client *FrontendClient, dataType string) {
	data, err := m.fetchData(dataType)
	if err != nil {
		logs.Error("[FrontendWS] 获取数据失败 [%s]: %v", dataType, err)
		return
	}

	push := FrontendPush{
		Type: dataType,
		Data: data,
		Time: time.Now().UnixMilli(),
	}

	client.mu.Lock()
	err = ws.WriteJSON(push)
	client.mu.Unlock()

	if err != nil {
		logs.Error("[FrontendWS] 推送失败: %v", err)
	}
}

// fetchData 根据类型获取数据
func (m *FrontendWSManager) fetchData(dataType string) (interface{}, error) {
	switch dataType {
	case "devices":
		return m.getDevicesWithLocal()
	case "rpa_status":
		return m.getRpaStatus()
	case "rpa_flows":
		return m.getRpaFlows()
	case "server_status":
		return m.getServerStatus()
	default:
		return nil, nil
	}
}

// computeHash 计算数据的 SHA256 hash
func computeHash(data interface{}) [32]byte {
	b, _ := json.Marshal(data)
	return sha256.Sum256(b)
}

// getDevicesWithLocal 获取带本地状态的设备列表
func (m *FrontendWSManager) getDevicesWithLocal() (interface{}, error) {
	core := GetJpyCore()
	if core == nil {
		return []interface{}{}, nil
	}

	devices := core.GetAllDevice()

	rpaStatuses, _ := rpa.GetEngine().GetAllDeviceStatus()
	rpaMap := make(map[int]*rpa.EngineStatus)
	for i := range rpaStatuses {
		rpaMap[rpaStatuses[i].DeviceID] = &rpaStatuses[i]
	}

	// 获取本地 WS 连接的设备列表，用于判断 wsConnected
	wsConnectedSet := make(map[string]bool)
	if wsServer := GetDeviceWSServer(); wsServer != nil {
		for _, dc := range wsServer.GetManager().GetAll() {
			if dc.Serialno != "" {
				wsConnectedSet[dc.Serialno] = true
			}
		}
	}

	result := make([]map[string]interface{}, 0, len(devices))
	for _, d := range devices {
		uuid := d.MiddleAgentDevice.Uuid
		item := map[string]interface{}{
			"deviceId":            d.DeviceId,
			"serialno":            uuid,
			"online":              d.MiddleAgentDevice.Online,
			"wsConnected":         wsConnectedSet[uuid],
			"deviceInfo":          d.DeviceInfo,
			"tbYunJiUserDeviceId": d.TBYunJiUserDeviceId,
			"middleAgentDevice":   d.MiddleAgentDevice,
		}

		if status, ok := rpaMap[int(d.DeviceId)]; ok {
			local := &LocalDeviceState{
				RpaRunning:     status.Status == "running",
				RpaID:          status.RpaID,
				RpaName:        status.RpaName,
				RpaStatus:      status.Status,
				RpaStep:        status.CurrentStep,
				RpaStepName:    status.StepName,
				RpaTotalSteps:  status.TotalSteps,
				RpaLoopCount:   status.LoopCount,
				RpaLastError:   status.LastError,
				RpaSubStep:     status.SubStep,
				RpaSubStepName: status.SubStepName,
				RpaTotalTime:   status.TotalTime,
			}
			if status.LoopStartAt != nil {
				t := status.LoopStartAt.Format(time.RFC3339)
				local.RpaLoopStartAt = &t
			}
			item["_local"] = local
		}

		result = append(result, item)
	}

	return result, nil
}

func (m *FrontendWSManager) getRpaStatus() (interface{}, error) {
	return rpa.GetEngine().GetAllDeviceStatus()
}

func (m *FrontendWSManager) getRpaFlows() (interface{}, error) {
	return []interface{}{}, nil
}

func (m *FrontendWSManager) getServerStatus() (interface{}, error) {
	host := GetCurrentHost()
	isConnected := GetJpyCore() != nil && GetGlobalApi() != nil

	return map[string]interface{}{
		"host":      host,
		"connected": isConnected,
	}, nil
}

// BroadcastToFrontend 广播数据到所有前端客户端（外部主动触发）
func BroadcastToFrontend(dataType string, data interface{}) {
	frontendWSManager.broadcastData(dataType, data)
}

func (m *FrontendWSManager) broadcastData(dataType string, data interface{}) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	push := FrontendPush{
		Type: dataType,
		Data: data,
		Time: time.Now().UnixMilli(),
	}

	for ws, client := range m.clients {
		if !client.subscribes[dataType] {
			continue
		}

		go func(w *websocket.Conn, c *FrontendClient) {
			c.mu.Lock()
			defer c.mu.Unlock()
			if err := w.WriteJSON(push); err != nil {
				logs.Error("[FrontendWS] 广播失败: %v", err)
			}
		}(ws, client)
	}
}

// StartFrontendPushLoop 启动变化检测循环（按需推送）
func StartFrontendPushLoop() {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			frontendWSManager.checkAndPush()
		}
	}()

	logger.LogInfo("[FrontendWS] 变化检测循环已启动")
}

// NotifyDevicesChanged 设备上线/离线时主动推送设备列表
func NotifyDevicesChanged() {
	// 短暂延迟，等 manager 完成 Add/Remove
	time.Sleep(200 * time.Millisecond)
	data, err := frontendWSManager.getDevicesWithLocal()
	if err != nil {
		return
	}
	// 更新 hash 并广播
	newHash := computeHash(data)
	frontendWSManager.hashLock.Lock()
	frontendWSManager.lastHashes["devices"] = newHash
	frontendWSManager.hashLock.Unlock()

	frontendWSManager.broadcastData("devices", data)
}

// checkAndPush 检测数据变化，有变化才推送
func (m *FrontendWSManager) checkAndPush() {
	m.lock.RLock()
	if len(m.clients) == 0 {
		m.lock.RUnlock()
		return
	}

	// 收集所有订阅的数据类型
	needCheck := make(map[string]bool)
	for _, client := range m.clients {
		for dataType := range client.subscribes {
			needCheck[dataType] = true
		}
	}
	m.lock.RUnlock()

	// 逐个类型检测变化
	for dataType := range needCheck {
		data, err := m.fetchData(dataType)
		if err != nil {
			continue
		}

		newHash := computeHash(data)

		// 对比 hash
		m.hashLock.Lock()
		oldHash, exists := m.lastHashes[dataType]
		changed := !exists || newHash != oldHash
		if changed {
			m.lastHashes[dataType] = newHash
		}
		m.hashLock.Unlock()

		if !changed {
			continue
		}

		// 有变化，推送给所有订阅者
		m.broadcastData(dataType, data)
	}
}

// IsDeviceWSConnected 判断指定设备ID是否有本地WS连接
// 通过云平台设备列表拿到 UUID(serialno)，再到 DeviceWSServer 匹配
func IsDeviceWSConnected(deviceID int) bool {
	uuid := GetDeviceUUID(deviceID)
	if uuid == "" {
		return false
	}
	wsServer := GetDeviceWSServer()
	if wsServer == nil {
		return false
	}
	_, ok := wsServer.GetManager().GetBySerialNo(uuid)
	return ok
}

// GetDeviceUUID 通过设备ID获取设备的 UUID(serialno)
func GetDeviceUUID(deviceID int) string {
	core := GetJpyCore()
	if core == nil {
		return ""
	}
	for _, d := range core.GetAllDevice() {
		if int(d.DeviceId) == deviceID {
			return d.MiddleAgentDevice.Uuid
		}
	}
	return ""
}
