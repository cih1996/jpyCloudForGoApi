package service

import (
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
	clients map[*websocket.Conn]*FrontendClient
	lock    sync.RWMutex
}

// FrontendClient 前端客户端信息
type FrontendClient struct {
	mu           *sync.Mutex
	subscribes   map[string]bool // 订阅的数据类型
	lastPushTime map[string]time.Time
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
	// 集控平台原生字段（保持原样透传）
	Raw interface{} `json:"raw"`
	// 本地增强字段
	Local *LocalDeviceState `json:"_local,omitempty"`
}

// LocalDeviceState 本地设备状态
type LocalDeviceState struct {
	RpaRunning     bool   `json:"rpaRunning"`
	RpaID          uint   `json:"rpaId,omitempty"`
	RpaName        string `json:"rpaName,omitempty"`
	RpaStatus      string `json:"rpaStatus,omitempty"`
	RpaStep        int    `json:"rpaStep,omitempty"`
	RpaStepName    string `json:"rpaStepName,omitempty"`
	RpaTotalSteps  int    `json:"rpaTotalSteps,omitempty"`
	RpaLoopCount   int    `json:"rpaLoopCount,omitempty"`
	RpaLastError   string `json:"rpaLastError,omitempty"`
	RpaSubStep     int    `json:"rpaSubStep,omitempty"`
	RpaSubStepName string `json:"rpaSubStepName,omitempty"`
}

var frontendWSManager = &FrontendWSManager{
	clients: make(map[*websocket.Conn]*FrontendClient),
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
		mu:           mu,
		subscribes:   make(map[string]bool),
		lastPushTime: make(map[string]time.Time),
	}

	frontendWSManager.register(ws, client)
	defer frontendWSManager.unregister(ws)

	logs.Info("[FrontendWS] 客户端连接")

	// 设置 pong 处理
	ws.SetReadDeadline(time.Now().Add(wsPongWait))
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	// 启动 ping 协程
	go frontendPingLoop(ws, mu)

	// 读取消息循环
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
		// 订阅数据类型
		if dataType, ok := msg.Data.(string); ok {
			client.subscribes[dataType] = true
			logs.Info("[FrontendWS] 订阅: %s", dataType)
			// 立即推送一次数据
			m.pushDataToClient(ws, client, dataType)
		}

	case "unsubscribe":
		// 取消订阅
		if dataType, ok := msg.Data.(string); ok {
			delete(client.subscribes, dataType)
			logs.Info("[FrontendWS] 取消订阅: %s", dataType)
		}

	case "request":
		// 一次性请求数据
		if dataType, ok := msg.Data.(string); ok {
			m.pushDataToClient(ws, client, dataType)
		}

	default:
		logs.Warn("[FrontendWS] 未知消息类型: %s", msg.Type)
	}
}

func (m *FrontendWSManager) pushDataToClient(ws *websocket.Conn, client *FrontendClient, dataType string) {
	var data interface{}
	var err error

	switch dataType {
	case "devices":
		data, err = m.getDevicesWithLocal()
	case "rpa_status":
		data, err = m.getRpaStatus()
	case "rpa_flows":
		data, err = m.getRpaFlows()
	case "server_status":
		data, err = m.getServerStatus()
	default:
		logs.Warn("[FrontendWS] 未知数据类型: %s", dataType)
		return
	}

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

// getDevicesWithLocal 获取带本地状态的设备列表
func (m *FrontendWSManager) getDevicesWithLocal() (interface{}, error) {
	core := GetJpyCore()
	if core == nil {
		return []interface{}{}, nil
	}

	// 获取集控平台设备列表
	devices := core.GetAllDevice()

	// 获取所有 RPA 状态
	rpaStatuses, _ := rpa.GetEngine().GetAllDeviceStatus()
	rpaMap := make(map[int]*rpa.EngineStatus)
	for i := range rpaStatuses {
		rpaMap[rpaStatuses[i].DeviceID] = &rpaStatuses[i]
	}

	// 合并数据
	result := make([]map[string]interface{}, 0, len(devices))
	for _, d := range devices {
		item := map[string]interface{}{
			// 集控平台原生字段
			"deviceId":            d.DeviceId,
			"serialno":            d.MiddleAgentDevice.Uuid,
			"online":              d.MiddleAgentDevice.Online,
			"deviceInfo":          d.DeviceInfo,
			"tbYunJiUserDeviceId": d.TBYunJiUserDeviceId,
			"middleAgentDevice":   d.MiddleAgentDevice,
		}

		// 注入本地 RPA 状态
		if status, ok := rpaMap[int(d.DeviceId)]; ok {
			item["_local"] = &LocalDeviceState{
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
			}
		}

		result = append(result, item)
	}

	return result, nil
}

// getRpaStatus 获取所有 RPA 状态
func (m *FrontendWSManager) getRpaStatus() (interface{}, error) {
	return rpa.GetEngine().GetAllDeviceStatus()
}

// getRpaFlows 获取所有 RPA 流程
func (m *FrontendWSManager) getRpaFlows() (interface{}, error) {
	// 这里需要调用 database 获取
	// 暂时返回空，后续补充
	return []interface{}{}, nil
}

// getServerStatus 获取服务器连接状态
func (m *FrontendWSManager) getServerStatus() (interface{}, error) {
	host := GetCurrentHost()
	isConnected := GetJpyCore() != nil && GetGlobalApi() != nil

	return map[string]interface{}{
		"host":      host,
		"connected": isConnected,
	}, nil
}

// BroadcastToFrontend 广播数据到所有前端客户端
func BroadcastToFrontend(dataType string, data interface{}) {
	frontendWSManager.broadcast(dataType, data)
}

func (m *FrontendWSManager) broadcast(dataType string, data interface{}) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	push := FrontendPush{
		Type: dataType,
		Data: data,
		Time: time.Now().UnixMilli(),
	}

	for ws, client := range m.clients {
		// 只推送给订阅了该类型的客户端
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

// StartFrontendPushLoop 启动前端数据推送循环
func StartFrontendPushLoop() {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			frontendWSManager.pushToSubscribers()
		}
	}()

	logger.LogInfo("[FrontendWS] 推送循环已启动")
}

func (m *FrontendWSManager) pushToSubscribers() {
	m.lock.RLock()
	defer m.lock.RUnlock()

	if len(m.clients) == 0 {
		return
	}

	// 收集需要推送的数据类型
	needPush := make(map[string]bool)
	for _, client := range m.clients {
		for dataType := range client.subscribes {
			needPush[dataType] = true
		}
	}

	// 获取数据并推送
	for dataType := range needPush {
		var data interface{}
		var err error

		switch dataType {
		case "devices":
			data, err = m.getDevicesWithLocal()
		case "rpa_status":
			data, err = m.getRpaStatus()
		default:
			continue
		}

		if err != nil {
			continue
		}

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
				w.WriteJSON(push)
			}(ws, client)
		}
	}
}
