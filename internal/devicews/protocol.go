package devicews

import (
	"encoding/binary"
	"encoding/json"
	"errors"
)

// 协议头大小
const HeaderSize = 16

// 消息类型 - 客户端 → 服务端
const (
	MsgHeartbeat       uint8 = 0x01 // 心跳包
	MsgInit            uint8 = 0x02 // 初始化/注册
	MsgStatusReport    uint8 = 0x10 // 状态上报
	MsgLogReport       uint8 = 0x11 // 日志上报
	MsgTaskResult      uint8 = 0x12 // 任务执行结果
	MsgProgressReport  uint8 = 0x13 // 进度上报
	MsgScriptPull      uint8 = 0x14 // 请求拉取脚本
	MsgResourcePull    uint8 = 0x15 // 请求拉取资源
	MsgScreenshotData  uint8 = 0x16 // 截图数据上传
	MsgDebugResult     uint8 = 0x17 // 调试执行结果
	MsgNodesData       uint8 = 0x18 // 节点信息上报
)

// 消息类型 - 服务端 → 客户端
const (
	MsgHeartbeatAck  uint8 = 0x01 // 心跳响应
	MsgInitAck       uint8 = 0x02 // 初始化响应
	MsgTaskPush      uint8 = 0x20 // 下发脚本任务
	MsgTaskCancel    uint8 = 0x21 // 取消任务
	MsgConfigUpdate  uint8 = 0x22 // 配置更新
	MsgCommand       uint8 = 0x23 // 执行命令
	MsgScriptData    uint8 = 0x24 // 脚本内容响应
	MsgResourceData  uint8 = 0x25 // 资源内容响应
	MsgResourcePush  uint8 = 0x26 // 主动推送资源
	MsgDebugExec     uint8 = 0x27 // 调试模式直接执行
)

// 数据格式
const (
	FormatNone   uint8 = 0x00 // 无数据
	FormatJSON   uint8 = 0x01 // JSON 格式
	FormatText   uint8 = 0x02 // 纯文本
	FormatBinary uint8 = 0x03 // 二进制数据
)

// Header 协议头
type Header struct {
	MsgType    uint8  // 消息类型
	DataFormat uint8  // 数据格式
	DeviceID   uint32 // 设备ID
	SeqNo      uint32 // 消息序号
	DataLen    uint32 // 数据长度
	Reserved   uint16 // 保留字段
}

// Packet 完整数据包
type Packet struct {
	Header  Header
	Payload []byte
}

// ParseHeader 解析协议头
func ParseHeader(data []byte) (*Header, error) {
	if len(data) < HeaderSize {
		return nil, errors.New("data too short for header")
	}
	return &Header{
		MsgType:    data[0],
		DataFormat: data[1],
		DeviceID:   binary.BigEndian.Uint32(data[2:6]),
		SeqNo:      binary.BigEndian.Uint32(data[6:10]),
		DataLen:    binary.BigEndian.Uint32(data[10:14]),
		Reserved:   binary.BigEndian.Uint16(data[14:16]),
	}, nil
}

// ParsePacket 解析完整数据包
func ParsePacket(data []byte) (*Packet, error) {
	header, err := ParseHeader(data)
	if err != nil {
		return nil, err
	}
	expectedLen := HeaderSize + int(header.DataLen)
	if len(data) < expectedLen {
		return nil, errors.New("incomplete packet")
	}
	return &Packet{
		Header:  *header,
		Payload: data[HeaderSize:expectedLen],
	}, nil
}

// BuildPacket 构建数据包
func BuildPacket(msgType, dataFormat uint8, deviceID, seqNo uint32, payload []byte) []byte {
	dataLen := uint32(len(payload))
	buf := make([]byte, HeaderSize+dataLen)

	buf[0] = msgType
	buf[1] = dataFormat
	binary.BigEndian.PutUint32(buf[2:6], deviceID)
	binary.BigEndian.PutUint32(buf[6:10], seqNo)
	binary.BigEndian.PutUint32(buf[10:14], dataLen)
	binary.BigEndian.PutUint16(buf[14:16], 0) // reserved

	if dataLen > 0 {
		copy(buf[HeaderSize:], payload)
	}
	return buf
}

// BuildJSONPacket 构建 JSON 数据包
func BuildJSONPacket(msgType uint8, deviceID, seqNo uint32, data interface{}) ([]byte, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return BuildPacket(msgType, FormatJSON, deviceID, seqNo, payload), nil
}

// InitPayload 初始化包的 payload 结构
type InitPayload struct {
	Serialno       string `json:"serialno"`
	Version        string `json:"version"`
	SdkVersion     string `json:"sdkVersion"`
	ScreenWidth    int    `json:"screenWidth"`
	ScreenHeight   int    `json:"screenHeight"`
	Brand          string `json:"brand"`
	Model          string `json:"model"`
	AndroidVersion string `json:"androidVersion"`
	State          string `json:"state"`
}

// InitAckPayload 初始化响应的 payload 结构
type InitAckPayload struct {
	Success    bool                   `json:"success"`
	ServerTime int64                  `json:"serverTime"`
	Config     map[string]interface{} `json:"config,omitempty"`
}

// StatusReportPayload 状态上报的 payload 结构
type StatusReportPayload struct {
	State       string `json:"state"`
	CurrentTask string `json:"currentTask,omitempty"`
	Progress    string `json:"progress,omitempty"`
	Timestamp   int64  `json:"timestamp"`
}

// LogReportPayload 日志上报的 payload 结构
type LogReportPayload struct {
	Level     string `json:"level"`
	Tag       string `json:"tag"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// TaskResultPayload 任务结果的 payload 结构
type TaskResultPayload struct {
	TaskID    string      `json:"taskId"`
	TaskName  string      `json:"taskName"`
	Success   bool        `json:"success"`
	Result    interface{} `json:"result,omitempty"`
	Error     string      `json:"error,omitempty"`
	Duration  int64       `json:"duration"`
	Timestamp int64       `json:"timestamp"`
}

// TaskPushPayload 任务下发的 payload 结构
type TaskPushPayload struct {
	TaskID     string                   `json:"taskId"`
	TaskName   string                   `json:"taskName"`
	ScriptHash string                   `json:"scriptHash"`
	Code       string                   `json:"code,omitempty"`
	Timeout    int64                    `json:"timeout"`
	Priority   int                      `json:"priority"`
	Resources  []map[string]interface{} `json:"resources,omitempty"`
	Variables  map[string]interface{}   `json:"variables,omitempty"` // 流程变量，脚本中通过 context.variables 访问
}

// CommandPayload 命令的 payload 结构
type CommandPayload struct {
	Cmd    string                 `json:"cmd"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// DebugExecPayload 调试执行的 payload 结构
type DebugExecPayload struct {
	DebugID string `json:"debugId"`
	Code    string `json:"code"`
	Timeout int64  `json:"timeout"`
}

// ScreenshotDataPayload 截图数据的 payload 结构
type ScreenshotDataPayload struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Data   string `json:"data"` // base64 编码的图片数据
}

// NodesDataPayload 节点信息的 payload 结构
type NodesDataPayload struct {
	RequestID string          `json:"requestId,omitempty"`
	Nodes     json.RawMessage `json:"nodes"` // 可以是 JSON 对象或 JSON 字符串
	Timestamp int64           `json:"timestamp"`
}
