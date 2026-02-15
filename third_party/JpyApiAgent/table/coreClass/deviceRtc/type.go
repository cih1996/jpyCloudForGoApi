package deviceRtc

import (
	"port-mapping-demo/third_party/JpyApiAgent/table/coreClass/publicStruct"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ghp3000/logs"
	"github.com/ghp3000/public"
	"github.com/gorilla/websocket"
)

var (
	server  sync.Map
	timeNow int64
)

type TypeInfo struct {
	publicStruct.Device
	WsConn *websocket.Conn // 设备 WS 连接（一对一）
	wsMu   sync.Mutex      // WS 写操作锁
}

// WsWriteMessage 线程安全的 WS 写操作
func (s *TypeInfo) WsWriteMessage(messageType int, data []byte) error {
	s.wsMu.Lock()
	defer s.wsMu.Unlock()
	if s.WsConn == nil {
		return nil
	}
	return s.WsConn.WriteMessage(messageType, data)
}

// WsWriteJSON 线程安全的 WS JSON 写操作
func (s *TypeInfo) WsWriteJSON(v interface{}) error {
	s.wsMu.Lock()
	defer s.wsMu.Unlock()
	if s.WsConn == nil {
		return nil
	}
	return s.WsConn.WriteJSON(v)
}

func save(deviceId uint64, middleAgent *TypeInfo) {
	server.Store(deviceId, middleAgent)
}
func get(deviceId uint64) *TypeInfo {
	middleAgent, ok := server.Load(deviceId)
	if !ok {
		return nil
	}
	return middleAgent.(*TypeInfo)
}
func del(deviceId uint64) {
	server.Delete(deviceId)
}
func getAll() []*TypeInfo {
	middleAgentList := make([]*TypeInfo, 0)
	server.Range(func(key, value any) bool {
		middleAgentList = append(middleAgentList, value.(*TypeInfo))
		return true
	})
	return middleAgentList
}
func (s *TypeInfo) SetCode(_ int, code int32, msg string) {
	s.Code = code
	s.Msg = msg
	logs.Info("[设备控制Rtc状态更新][deviceId=%d][code=%d]%s", s.DeviceId, s.Code, s.Msg)
}
func msgPackToJson(msg *public.Message, logMsg string) (string, error) {
	var a interface{}
	err := msg.Unmarshal(&a)
	if err != nil {
		return "", fmt.Errorf("[%s]消息解析错误,err=%s", logMsg, err.Error())
	}
	jsonData, err := json.Marshal(a)
	if err != nil {
		return "", fmt.Errorf("[%s]消息解析错误,err=%s", logMsg, err.Error())
	}
	return string(jsonData), nil
}
