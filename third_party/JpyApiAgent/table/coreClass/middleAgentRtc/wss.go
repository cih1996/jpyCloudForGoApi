package middleAgentRtc

import (
	"encoding/json"

	"github.com/ghp3000/logs"
	"github.com/ghp3000/public"
	"github.com/gorilla/websocket"
)

// WsBroadcastMessage 广播给 WS 客户端的消息结构
type WsBroadcastMessage struct {
	F      uint16      `json:"f"`
	Header uint64      `json:"header"`
	Code   int32       `json:"code"`
	Seq    uint32      `json:"seq"`
	Req    bool        `json:"req"`
	Data   interface{} `json:"data"`
}

// broadcastMsgToWs 将 RTC 收到的 msgpack 消息转 JSON 广播给所有 WS 客户端
func (s *TypeInfo) broadcastMsgToWs(msg *public.Message, header uint64) {
	// 如果没有 WS 客户端，直接返回
	hasClients := false
	s.WsClients.Range(func(_, _ any) bool {
		hasClients = true
		return false // 只需要知道有没有，不需要遍历全部
	})
	if !hasClients {
		return
	}

	// 解析 msgpack data 为通用 interface{}
	var data interface{}
	if err := msg.Unmarshal(&data); err != nil {
		logs.Error("中间件[%d] WS广播: 消息反序列化失败: %v", s.MiddlewareId, err)
		return
	}

	broadcastMsg := WsBroadcastMessage{
		F:      msg.F,
		Header: header,
		Code:   msg.Code,
		Seq:    msg.Seq,
		Req:    msg.Req,
		Data:   data,
	}

	jsonData, err := json.Marshal(broadcastMsg)
	if err != nil {
		logs.Error("中间件[%d] WS广播: JSON编码失败: %v", s.MiddlewareId, err)
		return
	}

	s.BroadcastToWsClients(websocket.TextMessage, jsonData)
}
