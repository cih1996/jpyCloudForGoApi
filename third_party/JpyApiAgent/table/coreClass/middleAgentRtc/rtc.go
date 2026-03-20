package middleAgentRtc

import (
	"port-mapping-demo/third_party/JpyApiAgent/table/coreClass/publicStruct"
	"fmt"
	"time"

	"github.com/ghp3000/logs"
	"github.com/ghp3000/netclient/BufferRTC"
	"github.com/ghp3000/netclient/NetClient"
	"github.com/ghp3000/netclient/bufferPool"
	"github.com/ghp3000/public"
)

//, parOpen NetClient.ConnectEvent, parClose NetClient.ConnectEvent, parCallBack NetClient.Callback, getToken func(mid uint64) (*rtcCtl.GetRtcTokenRes, *ErrPkg.Err)

// New 创建或获取中间件对象
// 返回值: (*TypeInfo, isNew bool, error)
// isNew 为 true 表示新创建，false 表示复用已有对象
func New(p *publicStruct.MiddleAgentTypeInfo) (*TypeInfo, bool, error) {
	middleAgentList := getAll()
	//检查是否已存在，如果存在则更新设备列表并返回已有对象
	for n := 0; n < len(middleAgentList); n++ {
		if middleAgentList[n].MiddlewareId == p.MiddlewareId {
			middleAgentList[n].Devices = p.Devices
			logs.Info("中间件[%d]对象已存在，复用现有对象并更新设备列表", p.MiddlewareId)
			return middleAgentList[n], false, nil
		}
	}
	var a TypeInfo
	a.MiddlewareId = p.MiddlewareId
	a.TokenInfo = p.TokenInfo
	a.Devices = p.Devices
	a.Reconnect = p.Reconnect
	//传入的函数
	a.GetToken = p.GetToken
	a.DelMiddleware = p.DelMiddleware
	//传入的回调
	a.FuncOpen = p.FuncOpen
	a.FuncClose = p.FuncClose
	a.FuncCallBack = p.FuncCallBack
	save(a.MiddlewareId, &a)
	return &a, true, nil
}

// Connect 中间件发起打洞连接
func (s *TypeInfo) Connect() {
	var err error
	s.Rtc, err = BufferRTC.New(s.MiddlewareId, s.TokenInfo.Url, s.TokenInfo.Token, true, bufferPool.Buffer, s.onOpen, s.onClose, s.onData)
	if err != nil {
		s.SetCode(0, CodeMiddleAgentRtc连接失败, MsgMiddleAgentRtc连接失败)
		return
	}
	s.SetCode(0, CodeMiddleAgentRtc连接建立, MsgMiddleAgent连接建立)
}
func (s *TypeInfo) onOpen(conn NetClient.NetClient) {
	s.SetCode(0, CodeMiddleAgentRtc连接成功, MsgMiddleAgentRtc连接成功)
	s.AsyncGetOnlineList()

	//调用外部回调
	if s.FuncOpen != nil {
		go s.FuncOpen(conn)
	}
}
func (s *TypeInfo) onClose(conn NetClient.NetClient) {
	s.SetCode(0, CodeMiddleAgentRtc断开连接, MsgMiddleAgentRtc断开连接)
	if s.Rtc != nil {
		err := s.Rtc.Close()
		if err != nil {
			return
		}
	}
	//如果需要重连，就重连
	if s.Reconnect == 0 {
		rec, err := s.GetToken(s.MiddlewareId)
		if err == nil {
			s.TokenInfo = rec
			s.Connect()
		} else {
			logs.Error("中间件[%d]重连获取RtcToken失败,%s", s.MiddlewareId, err.Msg)
			time.Sleep(5 * time.Second)
			s.onClose(conn)
		}
	} else {
		del(s.MiddlewareId)
		s.DelMiddleware(s.MiddlewareId)
	}

	//调用外部回调
	if s.FuncClose != nil {
		go s.FuncClose(conn)
	}
}
func (s *TypeInfo) onData(packet *bufferPool.Packet, conn NetClient.NetClient) bool {
	var header uint64
	var middleId uint64
	middleId = conn.Extra().(uint64)

	if err := packet.UnmarshalHeader(&header); err != nil {
		logs.Error("中间件[%d]解析包头内容失败", middleId)
		_ = s.Rtc.Close()
		return true
	}

	typ := packet.Type()
	switch typ {
	case bufferPool.TypePing:
		if err := conn.SendPong(); err != nil {
			_ = conn.Close()
		}
		return true
	case bufferPool.TypePong:
		logs.Info("中间件[%d]收到心跳", middleId)
		timeNow = time.Now().Unix()
		return true
	case bufferPool.TypeTestDelayResponse:
		var v publicStruct.DelayRequest
		if err := packet.Unmarshal(&v); err != nil {
			logs.Error("中间件[%d]异步测速返回的消息反序列化失败,", middleId, err)
			_ = s.Rtc.Close()
			return true
		}
		s.Delay = time.Now().UnixMicro() - v.Timestamp
		logs.Info("中间件[%d]异步测速的消息回来了,延迟=%d微秒", middleId, s.Delay)
		return true
	case bufferPool.TypeMsgpack:
		var msg public.Message
		if err := packet.Unmarshal(&msg); err != nil {
			logs.Error("中间件[%d]收到的msgpack数据反序列化失败,err=%s", middleId, err.Error())
			_ = s.Rtc.Close()
			return true
		}

		msg.Type = typ
		//异步转同步
		value, ok := s.Wait.Load(msg.Seq)
		if ok {
			logs.Info("异步转同步查询，发现seq=%d", msg.Seq)
			msg.Req = false
			ch, ok := value.(chan *public.Message)
			if ok {
				// 防止向已关闭的 channel 发送导致 panic
				select {
				case ch <- &msg:
				default:
					logs.Warn("异步转同步：channel 已关闭或满，seq=%d，丢弃响应", msg.Seq)
				}
			}
			return true
		}

		// 向所有 WS 客户端广播 RTC 消息
		s.broadcastMsgToWs(&msg, header)

		s.AsyncReceiveMessage(&msg, header, conn)
		return true

	default:
		logs.Error("中间件[%d]未处理的类型: %s", middleId, typ)
	}
	//调用外部回调
	if s.FuncCallBack != nil {
		go s.FuncCallBack(packet, conn)
	}
	return true
}

func (s *TypeInfo) SetCode(_ int, code int32, msg string) {
	s.Code = code
	s.Msg = msg
	logs.Info("[中间件Rtc状态更新][mid=%d][code=%d]%s", s.MiddlewareId, s.Code, s.Msg)
}

func (s *TypeInfo) AsyncReceiveMessage(msg *public.Message, header uint64, conn NetClient.NetClient) {
	var middleId uint64
	middleId = conn.Extra().(uint64)
	switch msg.F {
	case public.FuncTestDelay:
		var v publicStruct.DelayRequest
		if err := msg.Unmarshal(&v); err != nil {
			logs.Error("中间件[%d]返回的消息反序列化失败,err=%v", middleId, err)
		}
	case public.FuncOnlineList:
		s.eventOnlineOfflineMessage(conn, msg, header)
		return
	case public.FuncScreenChange:
		s.eventScreenRotation(conn, msg, header)
		return
	case public.FuncDevices:
		s.eventReceiveDeviceList(msg)
	//case public.FuncDevice: //一台
	//	deviceId := GetDeviceIdFromMiddleIdAndSeat(middleId, uint8(Id))
	//	s.MiddleRtc收到单台设备信息(msg, deviceId, conn)
	//case public.FuncWakeUPAways: //常亮
	//	logs.Info("[MiddleRtcClient]设备常亮处理成功，设备Id=%d", Id)
	//	midId := conn.Extra().(uint64)
	//	MidRtc, ok := s.MidGetConn(midId)
	//	if ok {
	//		atomic.StoreInt32(&MidRtc.Let常亮, 1)
	//	}
	//case public.FuncFileDownload:
	//	s.MiddleRtc收到设备下载状态信息(GetDeviceIdFromMiddleIdAndSeat(conn.Extra().(uint64), uint8(Id)), msg)
	//case public.FuncGetAppList:
	//	s.MiddleRtc收到设备app列表信息(GetDeviceIdFromMiddleIdAndSeat(conn.Extra().(uint64), uint8(Id)), msg)
	//case public.FuncCMDWithResult:
	//	s.MiddleRtc收到设备shell命令返回(GetDeviceIdFromMiddleIdAndSeat(conn.Extra().(uint64), uint8(Id)), msg)
	default:
		jsonStr, err := msgPackToJson(msg, fmt.Sprintf("中间件[%d]未处理的函数", middleId))
		if err != nil {
			return
		}
		logs.Info("中间件[%d]未处理的函数 F=%d,seq=%v,json字符串 %s,", middleId, msg.F, msg.Seq, jsonStr)
	}
}
