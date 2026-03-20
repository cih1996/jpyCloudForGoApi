package devicews

import (
	"encoding/json"
	"net/http"
	"port-mapping-demo/pkg/logger"
	"time"

	"github.com/gorilla/websocket"
)

// Server 设备 WebSocket 服务器
type Server struct {
	manager  *DeviceManager
	upgrader websocket.Upgrader
	addr     string

	// 消息处理器
	handlers map[uint8]MessageHandler

	// SerialnoResolver 通过 DeviceID 反查设备序列号（由外部注入，避免循环依赖）
	SerialnoResolver func(deviceID uint32) string
}

// MessageHandler 消息处理器类型
type MessageHandler func(dc *DeviceConn, packet *Packet)

// NewServer 创建服务器
func NewServer(addr string) *Server {
	s := &Server{
		manager: NewDeviceManager(),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
		},
		addr:     addr,
		handlers: make(map[uint8]MessageHandler),
	}

	// 注册默认处理器
	s.registerDefaultHandlers()

	return s
}

// registerDefaultHandlers 注册默认消息处理器
func (s *Server) registerDefaultHandlers() {
	// 心跳处理（新协议：心跳携带 JSON 设备身份信息）
	s.handlers[MsgHeartbeat] = func(dc *DeviceConn, packet *Packet) {
		// 解析心跳 payload（兼容旧协议空 payload）
		if packet.Header.DataFormat == FormatJSON && len(packet.Payload) > 0 {
			var hb HeartbeatPayload
			if err := json.Unmarshal(packet.Payload, &hb); err == nil {
				// 如果设备尚未通过 INIT 注册（兼容模式），用心跳补充身份
				if dc.Serialno == "" && hb.Serialno != "" {
					dc.Serialno = hb.Serialno
					logger.DeviceWSInfo("设备身份已通过心跳补充: %08X (%s)", dc.DeviceID, dc.Serialno)
				}
			}
		}
		if err := dc.SendHeartbeatAck(packet.Header.SeqNo); err != nil {
			logger.DeviceWSWarn("发送心跳ACK失败: %08X, err: %v", dc.DeviceID, err)
		}
	}

	// INIT 处理（兼容：心跳首包注册后，APK 补发 INIT 更新设备信息）
	s.handlers[MsgInit] = func(dc *DeviceConn, packet *Packet) {
		if err := dc.HandleInit(packet); err != nil {
			logger.DeviceWSWarn("处理延迟 INIT 失败: %08X, err: %v", dc.DeviceID, err)
			return
		}
		logger.DeviceWSInfo("设备信息已更新(延迟INIT): %08X (%s), brand=%s, model=%s",
			dc.DeviceID, dc.Serialno, dc.Info.Brand, dc.Info.Model)
		// 回复 INIT_ACK
		config := map[string]interface{}{
			"heartbeatInterval": int(HeartbeatInterval / time.Millisecond),
			"taskTimeout":       300000,
		}
		dc.SendInitAck(true, config)
	}

	// 状态上报
	s.handlers[MsgStatusReport] = func(dc *DeviceConn, packet *Packet) {
		if packet.Header.DataFormat == FormatJSON {
			var payload StatusReportPayload
			if err := json.Unmarshal(packet.Payload, &payload); err == nil {
				dc.State = DeviceState(payload.State)
				logger.DeviceWSDebug("设备 %08X 状态: %s, 任务: %s", dc.DeviceID, payload.State, payload.CurrentTask)
			}
		}
	}

	// 日志上报
	s.handlers[MsgLogReport] = func(dc *DeviceConn, packet *Packet) {
		if packet.Header.DataFormat == FormatJSON {
			var payload LogReportPayload
			if err := json.Unmarshal(packet.Payload, &payload); err == nil {
				logger.DeviceWSDebug("设备 %08X 日志 [%s][%s]: %s", dc.DeviceID, payload.Level, payload.Tag, payload.Message)
			}
		}
	}

	// 任务结果
	s.handlers[MsgTaskResult] = func(dc *DeviceConn, packet *Packet) {
		if packet.Header.DataFormat == FormatJSON {
			var payload TaskResultPayload
			if err := json.Unmarshal(packet.Payload, &payload); err == nil {
				logger.DeviceWSInfo("设备 %08X 任务结果: taskId=%s, success=%v, duration=%dms",
					dc.DeviceID, payload.TaskID, payload.Success, payload.Duration)
				// 存储任务结果
				dc.StoreTaskResult(&payload)
			}
		}
	}

	// 进度上报
	s.handlers[MsgProgressReport] = func(dc *DeviceConn, packet *Packet) {
		// 可以转发给前端或记录
		logger.DeviceWSDebug("设备 %08X 进度上报: %s", dc.DeviceID, string(packet.Payload))
	}

	// 脚本拉取请求
	s.handlers[MsgScriptPull] = func(dc *DeviceConn, packet *Packet) {
		logger.DeviceWSDebug("设备 %08X 请求拉取脚本: %s", dc.DeviceID, string(packet.Payload))
		// TODO: 从脚本库获取脚本并发送
	}

	// 资源拉取请求
	s.handlers[MsgResourcePull] = func(dc *DeviceConn, packet *Packet) {
		logger.DeviceWSDebug("设备 %08X 请求拉取资源: %s", dc.DeviceID, string(packet.Payload))
		// TODO: 从资源库获取资源并发送
	}

	// 截图数据
	s.handlers[MsgScreenshotData] = func(dc *DeviceConn, packet *Packet) {
		if packet.Header.DataFormat == FormatJSON {
			var payload ScreenshotDataPayload
			if err := json.Unmarshal(packet.Payload, &payload); err == nil {
				logger.DeviceWSDebug("设备 %08X 上传截图: %dx%d, 数据大小: %d bytes",
					dc.DeviceID, payload.Width, payload.Height, len(payload.Data))
				dc.StoreScreenshot(&payload)
			} else {
				logger.DeviceWSWarn("设备 %08X 截图数据解析失败: %v", dc.DeviceID, err)
			}
		} else {
			logger.DeviceWSWarn("设备 %08X 截图数据格式错误, 期望 JSON", dc.DeviceID)
		}
	}

	// 节点信息
	s.handlers[MsgNodesData] = func(dc *DeviceConn, packet *Packet) {
		if packet.Header.DataFormat == FormatJSON {
			var payload NodesDataPayload
			if err := json.Unmarshal(packet.Payload, &payload); err == nil {
				logger.DeviceWSDebug("设备 %08X 上传节点信息: requestId=%s",
					dc.DeviceID, payload.RequestID)
				dc.StoreNodes(&payload)
			} else {
				logger.DeviceWSWarn("设备 %08X 节点数据解析失败: %v", dc.DeviceID, err)
			}
		} else {
			logger.DeviceWSWarn("设备 %08X 节点数据格式错误, 期望 JSON", dc.DeviceID)
		}
	}

	// 调试执行结果
	s.handlers[MsgDebugResult] = func(dc *DeviceConn, packet *Packet) {
		if packet.Header.DataFormat == FormatJSON {
			var payload DebugResultPayload
			if err := json.Unmarshal(packet.Payload, &payload); err == nil {
				logger.DeviceWSInfo("设备 %08X 调试结果: debugId=%s, success=%v, duration=%dms",
					dc.DeviceID, payload.DebugID, payload.Success, payload.Duration)
				// 存储调试结果
				dc.StoreDebugResult(&payload)
			}
		}
	}
}

// RegisterHandler 注册自定义消息处理器
func (s *Server) RegisterHandler(msgType uint8, handler MessageHandler) {
	s.handlers[msgType] = handler
}

// GetManager 获取设备管理器
func (s *Server) GetManager() *DeviceManager {
	return s.manager
}

// Start 启动服务器
func (s *Server) Start() error {
	s.manager.Start()

	http.HandleFunc("/ws/device", s.handleConnection)

	logger.DeviceWSInfo("设备 WebSocket 服务启动: %s/ws/device", s.addr)
	return http.ListenAndServe(s.addr, nil)
}

// StartWithMux 使用自定义 mux 启动（集成到现有服务）
func (s *Server) StartWithMux(mux *http.ServeMux, path string) {
	s.manager.Start()
	mux.HandleFunc(path, s.handleConnection)
	logger.DeviceWSInfo("设备 WebSocket 服务注册: %s", path)
}

// Stop 停止服务器
func (s *Server) Stop() {
	s.manager.Stop()
}

// handleConnection 处理 WebSocket 连接
func (s *Server) handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.DeviceWSError("WebSocket 升级失败: %v", err)
		return
	}

	dc := NewDeviceConn(conn)
	clientIP := r.RemoteAddr

	logger.DeviceWSDebug("新连接: %s", clientIP)

	// 等待首包（3秒超时，避免垃圾连接）
	conn.SetReadDeadline(time.Now().Add(InitTimeout))
	_, data, err := conn.ReadMessage()
	if err != nil {
		logger.DeviceWSWarn("等待首包超时或读取失败: %s, err: %v", clientIP, err)
		conn.Close()
		return
	}

	// 解析数据包
	packet, err := ParsePacket(data)
	if err != nil {
		logger.DeviceWSWarn("解析数据包失败: %s, err: %v", clientIP, err)
		conn.Close()
		return
	}

	config := map[string]interface{}{
		"heartbeatInterval": int(HeartbeatInterval / time.Millisecond),
		"taskTimeout":       300000,
	}

	switch packet.Header.MsgType {
	case MsgInit:
		// 标准流程：首包是 INIT
		if err := dc.HandleInit(packet); err != nil {
				logger.DeviceWSWarn("处理 INIT 失败: %s, err: %v", clientIP, err)
			conn.Close()
			return
		}
		s.manager.Add(dc)
		logger.DeviceWSInfo("设备注册成功(INIT): %08X (%s), brand=%s, model=%s, version=%s",
			dc.DeviceID, dc.Serialno, dc.Info.Brand, dc.Info.Model, dc.Info.Version)

		if err := dc.SendInitAck(true, config); err != nil {
			logger.DeviceWSError("发送 INIT_ACK 失败: %v", err)
			s.manager.Remove(dc.DeviceID)
			return
		}

	case MsgHeartbeat:
		// 兼容模式：APK 首包是心跳（未实现 INIT 握手）
		// 从协议头提取 DeviceID
		dc.DeviceID = packet.Header.DeviceID
		dc.Info = &DeviceInfo{}

		// 新协议：心跳携带 JSON 身份信息，尝试提取 serialno
		if packet.Header.DataFormat == FormatJSON && len(packet.Payload) > 0 {
			var hb HeartbeatPayload
			if err := json.Unmarshal(packet.Payload, &hb); err == nil {
				dc.Serialno = hb.Serialno
				dc.Info.Serialno = hb.Serialno
			}
		}

		// Serialno 为空时，通过 DeviceID 反查云平台 UUID 补全
		if dc.Serialno == "" && s.SerialnoResolver != nil {
			if resolved := s.SerialnoResolver(dc.DeviceID); resolved != "" {
				dc.Serialno = resolved
				dc.Info.Serialno = resolved
				logger.DeviceWSInfo("设备身份已通过反查补全: %08X → %s", dc.DeviceID, resolved)
			}
		}

		s.manager.Add(dc)
		if dc.Serialno != "" {
			logger.DeviceWSInfo("设备注册成功(心跳兼容): %08X (%s), ip=%s", dc.DeviceID, dc.Serialno, clientIP)
		} else {
			logger.DeviceWSWarn("设备注册成功(心跳兼容): %08X, ip=%s (Serialno为空，WS状态可能异常)", dc.DeviceID, clientIP)
		}

		// 回复心跳 ACK
		if err := dc.SendHeartbeatAck(packet.Header.SeqNo); err != nil {
			logger.DeviceWSWarn("发送首包心跳ACK失败: %08X, err: %v", dc.DeviceID, err)
		}

	default:
		logger.DeviceWSWarn("首包类型不支持: %s, msgType: 0x%02X", clientIP, packet.Header.MsgType)
		conn.Close()
		return
	}

	// 清除读取超时，进入主循环
	conn.SetReadDeadline(time.Time{})

	// 主消息循环
	s.messageLoop(dc)

	// 连接断开，移除设备
	s.manager.Remove(dc.DeviceID)
	logger.DeviceWSInfo("设备断开: %08X (%s)", dc.DeviceID, dc.Serialno)
}

// messageLoop 消息处理循环
func (s *Server) messageLoop(dc *DeviceConn) {
	for {
		_, data, err := dc.Conn.ReadMessage()
		if err != nil {
			if !dc.IsClosed() {
				logger.DeviceWSDebug("读取消息失败: %08X, err: %v", dc.DeviceID, err)
			}
			return
		}

		// 更新活跃时间
		dc.UpdateLastSeen()
		s.manager.IncrementMessages()

		// 解析数据包
		packet, err := ParsePacket(data)
		if err != nil {
				logger.DeviceWSWarn("解析数据包失败: %08X, err: %v", dc.DeviceID, err)
			continue
		}

		// 【调试日志】打印收到的所有消息
		logger.DeviceWSInfo("← 收到消息: deviceId=%08X, msgType=0x%02X, dataFormat=0x%02X, payloadLen=%d",
			packet.Header.DeviceID, packet.Header.MsgType, packet.Header.DataFormat, len(packet.Payload))

		// 调用处理器
		if handler, ok := s.handlers[packet.Header.MsgType]; ok {
			handler(dc, packet)
		} else {
			logger.DeviceWSDebug("未知消息类型: %08X, msgType: 0x%02X", dc.DeviceID, packet.Header.MsgType)
		}

		// 调用全局消息回调
		if s.manager.onMessage != nil {
			s.manager.onMessage(dc, packet)
		}
	}
}

// SendTaskToDevice 向指定设备发送任务
func (s *Server) SendTaskToDevice(deviceID uint32, task *TaskPushPayload) error {
	dc, ok := s.manager.Get(deviceID)
	if !ok {
		return ErrDeviceNotFound
	}
	return dc.SendTaskPush(task)
}

// SendCommandToDevice 向指定设备发送命令
func (s *Server) SendCommandToDevice(deviceID uint32, cmd string, params map[string]interface{}) error {
	dc, ok := s.manager.Get(deviceID)
	if !ok {
		return ErrDeviceNotFound
	}
	return dc.SendCommand(cmd, params)
}

// SendDebugToDevice 向指定设备发送调试执行
func (s *Server) SendDebugToDevice(deviceID uint32, debugID, code string, timeout int64) error {
	dc, ok := s.manager.Get(deviceID)
	if !ok {
		return ErrDeviceNotFound
	}
	return dc.SendDebugExec(debugID, code, timeout)
}

// BroadcastTask 广播任务到所有设备
func (s *Server) BroadcastTask(task *TaskPushPayload) int {
	return s.manager.BroadcastJSON(MsgTaskPush, task)
}

// BroadcastCommand 广播命令到所有设备
func (s *Server) BroadcastCommand(cmd string, params map[string]interface{}) int {
	payload := CommandPayload{Cmd: cmd, Params: params}
	return s.manager.BroadcastJSON(MsgCommand, payload)
}
