package middleAgentRtc

import (
	"port-mapping-demo/third_party/JpyApiAgent/table/coreClass/publicStruct"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	publicStruct.MiddleAgentTypeInfo
	WsClients sync.Map // key: string(uuid), value: *WsClient
}

// WsClient 封装 WS 连接，加写锁防止并发写
type WsClient struct {
	Conn  *websocket.Conn
	mu    sync.Mutex
}

// WriteMessage 线程安全的 WS 写操作
func (w *WsClient) WriteMessage(messageType int, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Conn.WriteMessage(messageType, data)
}

// WriteJSON 线程安全的 WS JSON 写操作
func (w *WsClient) WriteJSON(v interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Conn.WriteJSON(v)
}

// Close 关闭 WS 连接
func (w *WsClient) Close() error {
	return w.Conn.Close()
}

// AddWsClient 注册一个 WS 客户端
func (s *TypeInfo) AddWsClient(id string, conn *websocket.Conn) {
	s.WsClients.Store(id, &WsClient{Conn: conn})
	logs.Info("中间件[%d] WS客户端已注册: %s", s.MiddlewareId, id)
}

// RemoveWsClient 移除一个 WS 客户端
func (s *TypeInfo) RemoveWsClient(id string) {
	s.WsClients.Delete(id)
	logs.Info("中间件[%d] WS客户端已移除: %s", s.MiddlewareId, id)
}

// BroadcastToWsClients 向所有活跃 WS 客户端广播消息
func (s *TypeInfo) BroadcastToWsClients(messageType int, data []byte) {
	s.WsClients.Range(func(key, value any) bool {
		client := value.(*WsClient)
		if err := client.WriteMessage(messageType, data); err != nil {
			logs.Error("中间件[%d] WS广播失败[%s]: %v", s.MiddlewareId, key.(string), err)
			client.Close()
			s.WsClients.Delete(key)
		}
		return true
	})
}

func (s *TypeInfo) DeviceIdGetMiddleId(deviceId uint64) uint64 {
	return deviceId >> 8
}

func save(middleId uint64, middleAgent *TypeInfo) {
	server.Store(middleId, middleAgent)
}
func get(middleId uint64) *TypeInfo {
	middleAgent, ok := server.Load(middleId)
	if !ok {
		return nil
	}
	return middleAgent.(*TypeInfo)
}
func del(middleId uint64) {
	server.Delete(middleId)
}

// ClearAll 清理所有中间件对象（用于重新登录时）
func ClearAll() {
	server.Range(func(key, value any) bool {
		if agent, ok := value.(*TypeInfo); ok && agent.Rtc != nil {
			agent.Rtc.Close()
		}
		server.Delete(key)
		return true
	})
	logs.Info("[middleAgentRtc] 已清理所有中间件对象")
}

func getAll() []*TypeInfo {
	middleAgentList := make([]*TypeInfo, 0)
	server.Range(func(key, value any) bool {
		middleAgentList = append(middleAgentList, value.(*TypeInfo))
		return true
	})
	return middleAgentList
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

func WriteBytesToFile(data []byte, path string) bool {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logs.Error("创建目录失败: %s, 错误: %v", dir, err)
		return false
	}

	// 写入数据到文件
	err := os.WriteFile(path, data, 0644)
	if err != nil {
		logs.Error("写入文件失败: %s, 错误: %v", path, err)
		return false
	}

	logs.Info("成功写入文件: %s, 数据大小: %d 字节", path, len(data))
	return true
}
