package coreClass

import (
	"portmap"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ghp3000/logs"
	"github.com/ghp3000/netclient/BufferRTC"
	"github.com/ghp3000/netclient/bufferPool"
	"github.com/ghp3000/netclient/netclient"
	"github.com/ghp3000/public"
)

func (c *Core) PortMapConnect(Url string, Token string, p *PortMapInfo) {
	s := &PortMapInfo{
		Wait:          sync.Map{},
		DeviceId:      p.DeviceId,
		PhonePort:     p.PhonePort,
		LocalPort:     p.LocalPort,
		PortMapRtc:    nil,
		Portforwarder: nil,
		LetOpen:       p.LetOpen,
		IsOpen:        p.IsOpen,
	}
	logs.Info("[PortMaprtc]开始连接,deviceId=%d,GuestUrl=%s,Token=%s==================================", p.DeviceId, Url, Token)
	rtc, err := BufferRTC.NewStreamClient(s.DeviceId, Url, Token, true, nil, s.portMapOpen, s.portMapOnClose, s.portMapData)
	if err != nil {
		logs.Error("rtc err:%s,TokenInfo=%v", err.Error(), Token)
	} else {
		s.PortMapRtc = rtc
		logs.Info("PortMapRtc连接开始,connIc=%d,user=%d", rtc.Extra(), s.DeviceId)
	}
}

func (s *PortMapInfo) PortMap重连() {
	logs.Info("<PortMap重连函数>")
}
func (s *PortMapInfo) portMapOnClose(_ netclient.NetClient) {
	logs.Info("PortMapClose连接断开")
	if s.PortMapRtc != nil {
		err := s.PortMapRtc.Close()
		if err != nil {
			logs.Error("PortMapClose关闭Rtc连接失败:%s", err.Error())
		}
	}
	if s.Portforwarder != nil {
		err := s.Portforwarder.Close()
		if err != nil {
			logs.Error("PortMapClose关闭本地断开服务失败:%s", err.Error())
		}
	}
	s.PortMapRtc = nil
	s.Portforwarder = nil
	atomic.StoreInt32(&s.IsOpen, 0)
	S.LocalPortSavePortMapInfo(s.LocalPort, s)
	if atomic.LoadInt32(&S.Callback端口映射界面打开) == 1 {
		S.CallBack端口映射表(s.LocalPort, s.DeviceId)
	}

	if atomic.LoadInt32(&s.LetOpen) == 1 {
		logs.Info("PortMapRtc连接断开,重连")
		go s.PortMap重连()
	}

}
func (s *PortMapInfo) portMapData(p *bufferPool.Packet, _ netclient.NetClient) bool {
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
func (s *PortMapInfo) portMapOpen(rtcConn netclient.NetClient) {
	logs.Info("portMapOpen======================================连接建立")
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
		return
	}
	conn, err := rtcConn.GetConnWithDeadline()
	if err != nil {
		logs.Info(err)
		return
	}

	ret := s.SyncCall(conn, 0, msg, time.Second*10)
	if err := ret.Error(); err != nil {
		logs.Error(err)
		if atomic.LoadInt32(&s.LetOpen) == 0 {
			logs.Info("[PortMap重连]关闭连接")
			return
		}
		time.Sleep(1 * time.Second)
		go s.PortMap重连()
		return
	}
	logs.Info(ret)
	//本地工作模式
	s.Portforwarder, err = portmap.NewForwarder(conn, 0, "tcp", s.LocalPort)
	//forwarder.Close()  关闭
	if err != nil {
		logs.Info(err)
		return
	}

	atomic.StoreInt32(&s.IsOpen, 1)
	S.LocalPortSavePortMapInfo(s.LocalPort, s)
	logs.Info("本地端口映射成功", s.LocalPort)
	if atomic.LoadInt32(&S.Callback端口映射界面打开) == 1 {
		S.CallBack端口映射表(s.LocalPort, s.DeviceId)
	}
	go func() {
		if err = s.Portforwarder.Start(); err != nil {
			logs.Info(err)
		}
	}()

}
