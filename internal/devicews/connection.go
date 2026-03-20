package devicews

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"port-mapping-demo/pkg/logger"
	"github.com/gorilla/websocket"
)

// DeviceState 设备状态
type DeviceState string

const (
	StateIdle    DeviceState = "idle"
	StateRunning DeviceState = "running"
	StateError   DeviceState = "error"
	StateDone    DeviceState = "done"
)

// DeviceInfo 设备信息
type DeviceInfo struct {
	Serialno       string `json:"serialno"`
	Version        string `json:"version"`
	SdkVersion     string `json:"sdkVersion"`
	ScreenWidth    int    `json:"screenWidth"`
	ScreenHeight   int    `json:"screenHeight"`
	Brand          string `json:"brand"`
	Model          string `json:"model"`
	AndroidVersion string `json:"androidVersion"`
}

// DeviceConn 设备连接
type DeviceConn struct {
	DeviceID   uint32          // 设备ID
	Serialno   string          // 设备序列号
	Conn       *websocket.Conn // WebSocket 连接
	Info       *DeviceInfo     // 设备信息
	State      DeviceState     // 当前状态
	ConnectedAt time.Time      // 连接时间
	LastSeen   time.Time       // 最后活跃时间
	SeqNo      uint32          // 消息序号（原子操作）

	writeMu    sync.Mutex      // 写锁（WebSocket 不支持并发写）
	closed     int32           // 是否已关闭（原子操作）
	closeChan  chan struct{}   // 关闭信号

	// 任务结果缓存
	taskResults   map[string]*TaskResultPayload
	taskResultsMu sync.RWMutex

	// 截图数据缓存
	lastScreenshot   *ScreenshotDataPayload
	lastScreenshotAt time.Time
	screenshotMu     sync.RWMutex

	// 节点信息缓存
	lastNodes     *NodesDataPayload
	lastNodesAt   time.Time
	nodesMu       sync.RWMutex

	// 调试执行结果缓存
	debugResults   map[string]*DebugResultPayload
	debugResultsMu sync.RWMutex
}

// DebugResultPayload 调试执行结果
type DebugResultPayload struct {
	DebugID   string      `json:"debugId"`
	Success   bool        `json:"success"`
	Result    interface{} `json:"result,omitempty"`
	Error     string      `json:"error,omitempty"`
	Logs      []string    `json:"logs,omitempty"`
	Duration  int64       `json:"duration"`
	Timestamp int64       `json:"timestamp"`
}

// NewDeviceConn 创建设备连接
func NewDeviceConn(conn *websocket.Conn) *DeviceConn {
	now := time.Now()
	return &DeviceConn{
		Conn:         conn,
		State:        StateIdle,
		ConnectedAt:  now,
		LastSeen:     now,
		closeChan:    make(chan struct{}),
		taskResults:  make(map[string]*TaskResultPayload),
		debugResults: make(map[string]*DebugResultPayload),
	}
}

// NextSeqNo 获取下一个消息序号
func (dc *DeviceConn) NextSeqNo() uint32 {
	return atomic.AddUint32(&dc.SeqNo, 1)
}

// UpdateLastSeen 更新最后活跃时间
func (dc *DeviceConn) UpdateLastSeen() {
	dc.LastSeen = time.Now()
}

// IsClosed 检查连接是否已关闭
func (dc *DeviceConn) IsClosed() bool {
	return atomic.LoadInt32(&dc.closed) == 1
}

// Close 关闭连接
func (dc *DeviceConn) Close() {
	if atomic.CompareAndSwapInt32(&dc.closed, 0, 1) {
		close(dc.closeChan)
		dc.Conn.Close()
	}
}

// WritePacket 发送数据包（线程安全）
func (dc *DeviceConn) WritePacket(packet []byte) error {
	if dc.IsClosed() {
		return websocket.ErrCloseSent
	}
	dc.writeMu.Lock()
	defer dc.writeMu.Unlock()
	return dc.Conn.WriteMessage(websocket.BinaryMessage, packet)
}

// SendHeartbeatAck 发送心跳响应
func (dc *DeviceConn) SendHeartbeatAck(seqNo uint32) error {
	packet := BuildPacket(MsgHeartbeatAck, FormatNone, dc.DeviceID, seqNo, nil)
	return dc.WritePacket(packet)
}

// SendInitAck 发送初始化响应
func (dc *DeviceConn) SendInitAck(success bool, config map[string]interface{}) error {
	ack := InitAckPayload{
		Success:    success,
		ServerTime: time.Now().UnixMilli(),
		Config:     config,
	}
	packet, err := BuildJSONPacket(MsgInitAck, dc.DeviceID, dc.NextSeqNo(), ack)
	if err != nil {
		return err
	}
	return dc.WritePacket(packet)
}

// SendTaskPush 下发任务
func (dc *DeviceConn) SendTaskPush(task *TaskPushPayload) error {
	packet, err := BuildJSONPacket(MsgTaskPush, dc.DeviceID, dc.NextSeqNo(), task)
	if err != nil {
		return err
	}
	return dc.WritePacket(packet)
}

// SendTaskCancel 取消任务
func (dc *DeviceConn) SendTaskCancel(taskID, reason string) error {
	payload := map[string]string{
		"taskId": taskID,
		"reason": reason,
	}
	packet, err := BuildJSONPacket(MsgTaskCancel, dc.DeviceID, dc.NextSeqNo(), payload)
	if err != nil {
		return err
	}
	return dc.WritePacket(packet)
}

// SendCommand 发送命令
func (dc *DeviceConn) SendCommand(cmd string, params map[string]interface{}) error {
	payload := CommandPayload{
		Cmd:    cmd,
		Params: params,
	}
	packet, err := BuildJSONPacket(MsgCommand, dc.DeviceID, dc.NextSeqNo(), payload)
	if err != nil {
		return err
	}
	// 【调试日志】打印发送的命令
	logger.DeviceWSInfo("→ 发送命令: deviceId=%08X, msgType=0x%02X (COMMAND), cmd=%s, params=%v",
		dc.DeviceID, MsgCommand, cmd, params)
	return dc.WritePacket(packet)
}

// SendDebugExec 发送调试执行
func (dc *DeviceConn) SendDebugExec(debugID, code string, timeout int64) error {
	payload := DebugExecPayload{
		DebugID: debugID,
		Code:    code,
		Timeout: timeout,
	}
	packet, err := BuildJSONPacket(MsgDebugExec, dc.DeviceID, dc.NextSeqNo(), payload)
	if err != nil {
		return err
	}
	// 【调试日志】打印发送的调试执行
	codePreview := code
	if len(codePreview) > 100 {
		codePreview = codePreview[:100] + "..."
	}
	logger.DeviceWSInfo("→ 发送调试执行: deviceId=%08X, msgType=0x%02X (DEBUG_EXEC), debugId=%s, code=%s",
		dc.DeviceID, MsgDebugExec, debugID, codePreview)
	return dc.WritePacket(packet)
}

// SendScriptData 发送脚本数据
func (dc *DeviceConn) SendScriptData(taskID, scriptHash, code string) error {
	payload := map[string]string{
		"taskId":     taskID,
		"scriptHash": scriptHash,
		"code":       code,
	}
	packet, err := BuildJSONPacket(MsgScriptData, dc.DeviceID, dc.NextSeqNo(), payload)
	if err != nil {
		return err
	}
	return dc.WritePacket(packet)
}

// SendResourceData 发送资源数据（JSON + Base64）
func (dc *DeviceConn) SendResourceData(name, hash, base64Data string) error {
	payload := map[string]string{
		"name": name,
		"hash": hash,
		"data": base64Data,
	}
	packet, err := BuildJSONPacket(MsgResourceData, dc.DeviceID, dc.NextSeqNo(), payload)
	if err != nil {
		return err
	}
	return dc.WritePacket(packet)
}

// SendResourcePush 主动推送资源
func (dc *DeviceConn) SendResourcePush(name, hash, base64Data string) error {
	payload := map[string]string{
		"name": name,
		"hash": hash,
		"data": base64Data,
	}
	packet, err := BuildJSONPacket(MsgResourcePush, dc.DeviceID, dc.NextSeqNo(), payload)
	if err != nil {
		return err
	}
	return dc.WritePacket(packet)
}

// SendConfigUpdate 发送配置更新
func (dc *DeviceConn) SendConfigUpdate(config map[string]interface{}) error {
	packet, err := BuildJSONPacket(MsgConfigUpdate, dc.DeviceID, dc.NextSeqNo(), config)
	if err != nil {
		return err
	}
	return dc.WritePacket(packet)
}

// HandleInit 处理初始化包
func (dc *DeviceConn) HandleInit(packet *Packet) error {
	if packet.Header.DataFormat != FormatJSON {
		return nil
	}

	var payload InitPayload
	if err := json.Unmarshal(packet.Payload, &payload); err != nil {
		logger.DeviceWSError("解析初始化数据失败: %v", err)
		return err
	}

	dc.DeviceID = packet.Header.DeviceID
	dc.Serialno = payload.Serialno
	dc.Info = &DeviceInfo{
		Serialno:       payload.Serialno,
		Version:        payload.Version,
		SdkVersion:     payload.SdkVersion,
		ScreenWidth:    payload.ScreenWidth,
		ScreenHeight:   payload.ScreenHeight,
		Brand:          payload.Brand,
		Model:          payload.Model,
		AndroidVersion: payload.AndroidVersion,
	}
	dc.State = DeviceState(payload.State)

	return nil
}

// StoreTaskResult 存储任务结果
func (dc *DeviceConn) StoreTaskResult(result *TaskResultPayload) {
	dc.taskResultsMu.Lock()
	defer dc.taskResultsMu.Unlock()
	dc.taskResults[result.TaskID] = result
}

// GetTaskResult 获取任务结果
func (dc *DeviceConn) GetTaskResult(taskID string) (*TaskResultPayload, bool) {
	dc.taskResultsMu.RLock()
	defer dc.taskResultsMu.RUnlock()
	result, ok := dc.taskResults[taskID]
	return result, ok
}

// ClearTaskResult 清除任务结果
func (dc *DeviceConn) ClearTaskResult(taskID string) {
	dc.taskResultsMu.Lock()
	defer dc.taskResultsMu.Unlock()
	delete(dc.taskResults, taskID)
}

// StoreDebugResult 存储调试执行结果
func (dc *DeviceConn) StoreDebugResult(result *DebugResultPayload) {
	dc.debugResultsMu.Lock()
	defer dc.debugResultsMu.Unlock()
	dc.debugResults[result.DebugID] = result
}

// GetDebugResult 获取调试执行结果
func (dc *DeviceConn) GetDebugResult(debugID string) (*DebugResultPayload, bool) {
	dc.debugResultsMu.RLock()
	defer dc.debugResultsMu.RUnlock()
	result, ok := dc.debugResults[debugID]
	return result, ok
}

// ClearDebugResult 清除调试执行结果
func (dc *DeviceConn) ClearDebugResult(debugID string) {
	dc.debugResultsMu.Lock()
	defer dc.debugResultsMu.Unlock()
	delete(dc.debugResults, debugID)
}

// StoreScreenshot 存储截图数据
func (dc *DeviceConn) StoreScreenshot(data *ScreenshotDataPayload) {
	dc.screenshotMu.Lock()
	defer dc.screenshotMu.Unlock()
	dc.lastScreenshot = data
	dc.lastScreenshotAt = time.Now()
}

// GetScreenshot 获取截图数据
func (dc *DeviceConn) GetScreenshot() (*ScreenshotDataPayload, time.Time) {
	dc.screenshotMu.RLock()
	defer dc.screenshotMu.RUnlock()
	return dc.lastScreenshot, dc.lastScreenshotAt
}

// StoreNodes 存储节点信息
func (dc *DeviceConn) StoreNodes(data *NodesDataPayload) {
	dc.nodesMu.Lock()
	defer dc.nodesMu.Unlock()
	dc.lastNodes = data
	dc.lastNodesAt = time.Now()
}

// GetNodes 获取节点信息
func (dc *DeviceConn) GetNodes() (*NodesDataPayload, time.Time) {
	dc.nodesMu.RLock()
	defer dc.nodesMu.RUnlock()
	return dc.lastNodes, dc.lastNodesAt
}
