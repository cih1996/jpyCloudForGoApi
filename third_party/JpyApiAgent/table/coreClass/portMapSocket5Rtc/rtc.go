package portMapSocket5Rtc

import (
	"port-mapping-demo/third_party/JpyApiAgent/table/coreClass/publicStruct"
	"encoding/json"
	"errors"
	"portmap"
	"sync/atomic"
	"time"

	"github.com/ghp3000/logs"
	"github.com/ghp3000/netclient/BufferRTC"
	"github.com/ghp3000/netclient/NetClient"
	"github.com/ghp3000/netclient/bufferPool"
	"github.com/ghp3000/public"
)

func New(p *publicStruct.PortMapTypeInfo) (*TypeInfo, error) {
	portMapList := getAll()
	//检查是否已存在
	for _, v := range portMapList {
		if v.DeviceId == p.DeviceId && v.PhonePort == p.PhonePort {
			return nil, errors.New("phonePort已占用")
		}
		if v.LocalPort == p.LocalPort {
			return nil, errors.New("localPort已占用")
		}
	}
	var a TypeInfo
	//传入的函数
	a.GetToken = p.GetToken
	a.DelPortMapId = p.DelPortMapId
	a.DeviceId = p.DeviceId
	a.TokenInfo = p.TokenInfo
	a.PhonePort = p.PhonePort
	a.LocalPort = p.LocalPort
	a.Mode = p.Mode
	a.Rtc = nil
	a.Forwarder = nil
	save(a.DeviceId, &a)
	return &a, nil

}
func (s *TypeInfo) Connect() {
	rtc, err := BufferRTC.NewStreamClient(s.DeviceId, s.TokenInfo.Url, s.TokenInfo.Token, true, nil, s.Open, s.OnClose, s.OnData)
	if err != nil {
		//logs.Error("rtc err:%s,TokenInfo=%v", err.Error(), s.Token)
		switch s.Mode {
		case ModePortMaps:
			s.SetCode(s.Mode, CodePortMaps断开连接, err.Error())
			break
		case ModeSocket5Server:
			s.SetCode(s.Mode, CodeSocket5Server断开连接, err.Error())
			break
		}
	} else {
		s.Rtc = rtc
		switch s.Mode {
		case ModePortMaps:
			s.SetCode(s.Mode, CodePortMaps开始Rtc连接, MsgPortMaps开始Rtc连接)
		case ModeSocket5Server:
			s.SetCode(s.Mode, CodeSocket5Server开始Rtc连接, MsgSocket5Server开始Rtc连接)
		}

	}
}
func (s *TypeInfo) GetCodeMsg() string {
	var msg string
	switch s.Code {
	case CodePortMaps开始Rtc连接:
		msg = MsgPortMaps开始Rtc连接
		break
	case CodePortMapsRtc连接建立:
		msg = MsgPortMapsRtc连接建立
		break
	case CodePortMapsRtc连接失败:
		msg = MsgPortMapsRtc连接失败
		break
	case CodePortMapsRtc连接成功:
		msg = MsgPortMapsRtc连接成功
		break
	case CodePortMaps本地端口开启开始:
		msg = MsgPortMaps本地端口开启开始
		break
	case CodePortMaps本地端口开启成功:
		msg = MsgPortMaps本地端口开启成功
		break
	case CodePortMaps本地端口开启失败:
		msg = MsgPortMaps本地端口开启失败
		break
	case CodePortMaps成功:
		msg = MsgPortMaps成功
		break

	case CodeSocket5Server开始Rtc连接:
		msg = MsgSocket5Server开始Rtc连接
		break
	case CodeSocket5ServerRtc连接建立:
		msg = MsgSocket5ServerRtc连接建立
		break
	case CodeSocket5ServerRtc连接失败:
		msg = MsgSocket5ServerRtc连接失败
		break
	case CodeSocket5ServerRtc连接成功:
		msg = MsgSocket5ServerRtc连接成功
		break
	case CodeSocket5Server本地端口开启开始:
		msg = MsgSocket5Server本地端口开启开始
		break
	case CodeSocket5Server本地端口开启成功:
		msg = MsgSocket5Server本地端口开启成功
		break
	case CodeSocket5Server本地端口开启失败:
		msg = MsgSocket5Server本地端口开启失败
		break
	case CodeSocket5Server成功:
		msg = MsgSocket5Server成功
		break
	}

	type jsonMsgInfo struct {
		Mode int    `json:"mode"`
		Code int32  `json:"code"`
		Msg  string `json:"msg"`
	}

	jsonMsg := jsonMsgInfo{
		Mode: s.Mode,
		Code: s.Code,
		Msg:  msg,
	}
	jsonMsgBytes, err := json.Marshal(jsonMsg)
	if err != nil {
		logs.Error("jsonMsgBytes err:%s", err.Error())
	}
	return string(jsonMsgBytes)
}
func (s *TypeInfo) OnClose(conn NetClient.NetClient) {
	//关闭连接时,设置code和msg
	s.SetCode(s.Mode, CodePortMaps断开连接, MsgPortMaps断开连接)
	if s.Rtc != nil {
		_ = s.Rtc.Close()
	}
	if s.Forwarder != nil {
		_ = s.Forwarder.Close()
	}

	//如果需要重连，就重连
	if s.Reconnect == 0 {
		rec, err := s.GetToken(s.DeviceId)
		if err == nil {
			s.TokenInfo = rec
			s.Connect()
		} else {
			logs.Error("映射[%d]重连获取Token失败,%s", s.DeviceId, err.Msg)
			time.Sleep(5 * time.Second)
			s.OnClose(conn)
		}
	} else {
		del(s.DeviceId)
		s.DelPortMapId(s.DeviceId)
	}

}
func (s *TypeInfo) SetCode(mode int, code int32, msg string) {
	switch mode {
	case ModePortMaps:
		atomic.StoreInt32(&s.Code, code)
		s.Msg = msg
		logs.Info("[端口映射状态打印:%s][deviceId=%d],url=%s,token=%s", msg, s.DeviceId, s.TokenInfo.Url, s.TokenInfo.Token)
	case ModeSocket5Server:
		atomic.StoreInt32(&s.Code, code)
		s.Msg = msg
		logs.Info("[Socket5服务状态打印:%s][deviceId=%d],url=%s,token=%s", msg, s.DeviceId, s.TokenInfo.Url, s.TokenInfo.Token)
	}
}
func (s *TypeInfo) OnData(p *bufferPool.Packet, _ NetClient.NetClient) bool {
	typ := p.Type()
	switch typ {
	case bufferPool.TypeMsgpack, bufferPool.TypeJson:
		var msg public.Message
		if err := p.Unmarshal(&msg); err != nil {
			logs.Error(err.Error())
			break
		}
		msg.Type = typ
		if !msg.Req { //是返回值
			value, ok := s.Wait.Load(msg.Seq)
			if ok {
				ch, ok := value.(chan *public.Message)
				if ok {
					ch <- &msg
				}
				return true
			}
		}
	default:
		break
	}
	return true
}
func (s *TypeInfo) Open(rtcConn NetClient.NetClient) {
	s.SetCode(s.Mode, CodePortMapsRtc连接建立, MsgPortMapsRtc连接建立)
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncPortMap, 1)
	req := public.ForwardRequest{
		Seat:    1,
		Mode:    0,
		Multi:   true, //是否端口复用
		Proto:   "tcp",
		DstPort: s.PhonePort, //手机的端口
	}
	if err := msg.Marshal(&req); err != nil {
		logs.Error(msg)
		s.SetCode(s.Mode, CodePortMaps断开连接, err.Error())
		return
	}
	conn, err := rtcConn.GetConnWithDeadline()
	if err != nil {
		logs.Info(err)
		s.SetCode(s.Mode, CodePortMaps断开连接, err.Error())
		return
	}

	ret := s.SyncCall(conn, 0, msg, time.Second*10)
	if err = ret.Error(); err != nil {
		time.Sleep(1 * time.Second)
		s.SetCode(s.Mode, CodePortMaps断开连接, err.Error())
		return
	}

	//logs.Info("执行状态：%d,执行结果文本%s:", ret.Code, ret.Msg)
	switch s.Mode {
	case ModePortMaps:
		if ret.Code != 0 {
			s.SetCode(s.Mode, CodePortMaps断开连接, MsgPortMaps断开连接)
		} else {
			s.SetCode(s.Mode, CodePortMapsRtc连接成功, MsgPortMapsRtc连接成功)
		}
		break
	case ModeSocket5Server:
		if ret.Code != 0 {
			s.SetCode(s.Mode, CodeSocket5Server断开连接, MsgSocket5Server断开连接)
		} else {
			s.SetCode(s.Mode, CodeSocket5ServerRtc连接成功, MsgSocket5ServerRtc连接成功)
		}
	}

	//本地工作模式
	s.Forwarder, err = portmap.NewForwarder(conn, s.Mode, "tcp", s.LocalPort)
	//forwarder.Close()  关闭
	if err != nil {
		logs.Info(err)
		switch s.Mode {
		case ModePortMaps:
			s.SetCode(s.Mode, CodePortMaps断开连接, MsgPortMaps断开连接)
			break
		case ModeSocket5Server:
			s.SetCode(s.Mode, CodeSocket5Server断开连接, MsgSocket5Server断开连接)
			break
		}
		return
	}

	switch s.Mode {
	case ModePortMaps:
		s.SetCode(s.Mode, CodePortMaps本地端口开启成功, MsgPortMaps本地端口开启成功)
		break
	case ModeSocket5Server:
		s.SetCode(s.Mode, CodeSocket5Server本地端口开启成功, MsgSocket5Server本地端口开启成功)
		break
	}

	go func() {
		if err = s.Forwarder.Start(); err != nil {
			logs.Info(err)
		}
	}()

	switch s.Mode {
	case ModePortMaps:
		s.SetCode(s.Mode, CodePortMaps成功, MsgPortMaps成功)
		break
	case ModeSocket5Server:
		s.SetCode(s.Mode, CodeSocket5Server成功, MsgSocket5Server成功)
		break
	}

}
