package deviceRtc

import (
	"port-mapping-demo/third_party/JpyApiAgent/table/coreClass/publicStruct"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ghp3000/logs"
	"github.com/ghp3000/netclient/BufferRTC"
	"github.com/ghp3000/netclient/NetClient"
	"github.com/ghp3000/netclient/bufferPool"
	"github.com/ghp3000/public"
	"github.com/gorilla/websocket"
)

func New(p *publicStruct.Device) (*TypeInfo, error) {
	deviceList := getAll()
	//检查是否已存在
	for n := 0; n < len(deviceList); n++ {
		if deviceList[n].DeviceId == p.DeviceId {
			return nil, errors.New(fmt.Sprintf("设备对象已存在,deviceId=%d", p.DeviceId))
		}
	}

	var a TypeInfo
	a.DeviceId = p.DeviceId
	a.TokenInfo = p.TokenInfo
	a.Reconnect = p.Reconnect
	//传入的函数
	a.GetToken = p.GetToken
	//传入的回调
	a.FuncOpen = p.FuncOpen
	a.FuncClose = p.FuncClose
	a.FuncCallBack = p.FuncCallBack
	save(a.DeviceId, &a)
	return &a, nil
}

func (s *TypeInfo) Connect() {
	var err error
	s.Rtc, err = BufferRTC.New(s.DeviceId, s.TokenInfo.Url, s.TokenInfo.Token, true, bufferPool.Buffer, s.onOpen, s.onClose, s.onData)
	if err != nil {
		s.SetCode(0, CodeDeviceRtc连接失败, MsgDeviceRtc连接失败)
		return
	}
	s.SetCode(0, CodeDeviceRtc连接建立, MsgDevice连接建立)
}

func (s *TypeInfo) onOpen(conn NetClient.NetClient) {
	s.SetCode(0, CodeDeviceRtc连接成功, MsgDeviceRtc连接成功)
	s.reConnectTodo()
	//调用外部回调
	if s.FuncOpen != nil {
		go s.FuncOpen(conn)
	}
}
func (s *TypeInfo) onClose(conn NetClient.NetClient) {
	s.SetCode(0, CodeDeviceRtc断开连接, MsgDeviceRtc断开连接)
	if s.Rtc != nil {
		err := s.Rtc.Close()
		if err != nil {
			return
		}
	}
	//如果需要重连，就重连
	if s.Reconnect == 0 {
		rec, err := s.GetToken(s.DeviceId)
		if err == nil {
			s.TokenInfo = rec
			s.Connect()
		} else {
			logs.Error("设备[%d]重连获取RtcToken失败,%s", s.DeviceId, err.Msg)
			time.Sleep(5 * time.Second)
			s.onClose(conn)
		}
	} else {
		del(s.DeviceId)
	}

	//调用外部回调
	if s.FuncClose != nil {
		go s.FuncClose(conn)
	}
}

func (s *TypeInfo) reConnectTodo() {
	if s.Code == CodeDeviceRtc连接成功 {
		if s.VideoStatus.Status == 1 && s.VideoStatus.ReConnect == 0 {
			s.AsyncStartVideo(s.VideoStatus.ReConnect)
		}
		if s.AudioStatus.Status == 1 && s.AudioStatus.ReConnect == 0 {
			s.AsyncStartAudio(s.AudioStatus.ReConnect)
		}
		return
	}
}

func (s *TypeInfo) onData(packet *bufferPool.Packet, conn NetClient.NetClient) bool {
	typ := packet.Type()
	switch typ {
	case bufferPool.TypePing:
		if err := conn.SendPong(); err != nil {
			_ = conn.Close()
		}
		return true
	case bufferPool.TypePong:
		logs.Info("设备[%d]收到心跳", s.DeviceId)
		timeNow = time.Now().Unix()
		return true
	case bufferPool.TypeTestDelayResponse:
		var v publicStruct.DelayRequest
		if err := packet.Unmarshal(&v); err != nil {
			logs.Error("中间件[%d]异步测速返回的消息反序列化失败,", s.DeviceId, err)
			_ = s.Rtc.Close()
			return true
		}
		s.Delay = time.Now().UnixMicro() - v.Timestamp
		logs.Info("中间件[%d]异步测速的消息回来了,延迟=%d微秒", s.DeviceId, s.Delay)
		return true
	case bufferPool.TypeMsgpack:
		var msg public.Message
		if err := packet.Unmarshal(&msg); err != nil {
			logs.Error("设备[%d]收到的msgpack数据反序列化失败,err=%s", s.DeviceId, err.Error())
			_ = s.Rtc.Close()
			return true
		}

		msg.Type = typ
		//异步转同步
		value, ok := s.Wait.Load(msg.Seq)
		if ok {
			msg.Req = false
			ch, ok := value.(chan *public.Message)
			if ok {
				ch <- &msg
			}
			return true
		}

		// 向 WS 客户端转发 msgpack 消息
		if s.WsConn != nil {
			var data interface{}
			if err := msg.Unmarshal(&data); err == nil {
				wsMsg := map[string]interface{}{
					"f":    msg.F,
					"code": msg.Code,
					"seq":  msg.Seq,
					"req":  msg.Req,
					"data": data,
				}
				if err := s.WsWriteJSON(wsMsg); err != nil {
					logs.Error("设备[%d] WS转发msgpack失败: %v", s.DeviceId, err)
				}
			}
		}

		s.AsyncReceiveMessage(&msg)
		return true
	case bufferPool.TypeVideo: //收到H264数据
		logs.Debug("收到H264数据,deviceId=", s.DeviceId)
		// 向 WS 客户端转发 H264 视频帧：首字节 0x01 + 原始数据
		if s.WsConn != nil {
			rawData := packet.Bytes()
			wsData := make([]byte, 1+len(rawData))
			wsData[0] = 0x01 // video 标识
			copy(wsData[1:], rawData)
			if err := s.WsWriteMessage(websocket.BinaryMessage, wsData); err != nil {
				logs.Error("设备[%d] WS转发H264失败: %v", s.DeviceId, err)
			}
		}
	case bufferPool.TypeAudio:
		logs.Debug("收到Audio数据,deviceId=", s.DeviceId)
		// 向 WS 客户端转发音频帧：首字节 0x02 + 原始数据
		if s.WsConn != nil {
			rawData := packet.Bytes()
			wsData := make([]byte, 1+len(rawData))
			wsData[0] = 0x02 // audio 标识
			copy(wsData[1:], rawData)
			if err := s.WsWriteMessage(websocket.BinaryMessage, wsData); err != nil {
				logs.Error("设备[%d] WS转发Audio失败: %v", s.DeviceId, err)
			}
		}
	default:
		logs.Error("设备[%d]未处理的Type类型: %s", s.DeviceId, typ)
	}
	//调用外部回调
	if s.FuncCallBack != nil {
		go s.FuncCallBack(packet, conn)
	}
	return true
}

func (s *TypeInfo) AsyncReceiveMessage(msg *public.Message) {
	switch msg.F {
	case public.FuncTestDelay:
		var v publicStruct.DelayRequest
		if err := msg.Unmarshal(&v); err != nil {
			logs.Error("设备[%d]返回的消息反序列化失败,", s.DeviceId, err)
		}
	case public.FuncStartVideo:
		atomic.StoreInt32(&s.VideoStatus.Status, 1)
	case public.FuncStopVideo:
		atomic.StoreInt32(&s.VideoStatus.Status, 0)
	case public.FuncStartAudio:
		atomic.StoreInt32(&s.AudioStatus.Status, 1)
	case public.FuncStopAudio:
		atomic.StoreInt32(&s.AudioStatus.Status, 0)
	default:
		jsonStr, err := msgPackToJson(msg, fmt.Sprintf("设备[%d]未处理的MsgPack函数", s.DeviceId))
		if err != nil {
			return
		}
		logs.Info("设备[%d]未处理的MsgPack函数 F=%d,json字符串 %s", s.DeviceId, msg.F, jsonStr)
	}
}
