package BufferRTC

import (
	"errors"
	"fmt"
	"time"

	"github.com/ghp3000/netclient/bufferPool"
	"github.com/ghp3000/netclient/NetClient"
	"github.com/ghp3000/netclient/webRTC"
	"github.com/pion/webrtc/v4"
	"port-mapping-demo/pkg/logger"
)

const name = "webrtc"

type RtcClient struct {
	buf       *bufferPool.Pool // 收发数据的读写器
	callback  NetClient.Callback
	Client    *webRTC.Client
	channel   *webRTC.DataChannel
	online    bool
	extra     interface{} //附加数据，可以用来做标志，比如这条连接的用户Id，等
	sessionId int64
}

func New(id interface{}, urlString string, token string, isGuest bool, pool *bufferPool.Pool, onOpen, onClose NetClient.ConnectEvent, callback NetClient.Callback) (*RtcClient, error) {
	if pool == nil {
		pool = bufferPool.Buffer
	}
	s := &RtcClient{extra: id, callback: callback, buf: pool, sessionId: time.Now().UnixNano()}
	c := webRTC.NewClient(urlString, token, isGuest, s.onData)
	c.SetLogger(func(f interface{}, v ...interface{}) {
		msg := fmt.Sprint(f)
		if len(v) > 0 {
			if format, ok := f.(string); ok {
				msg = fmt.Sprintf(format, v...)
			} else {
				args := append([]interface{}{f}, v...)
				msg = fmt.Sprint(args...)
			}
		}
		logger.LogInfo("[RTC] id=%v %s", id, msg)
	})
	c.Attach = id
	c.SetOnChannelOpen(func(c *webRTC.Client, channel *webRTC.DataChannel) {
		s.channel = channel
		s.online = true
		if onOpen != nil {
			onOpen(s)
		}
	})
	c.SetOnClose(func(c *webRTC.Client) {
		s.online = false
		if onClose != nil {
			onClose(s)
		}
	})
	s.Client = c
	//go func() {
	//	tk := time.NewTicker(time.Second)
	//	defer tk.Stop()
	//	for {
	//		select {
	//		case <-tk.C:
	//			if s.channel != nil {
	//				fmt.Println(id, s.channel.GetPackCount())
	//			}
	//		}
	//	}
	//}()
	//c.SetLogger(webRTC.DefaultLogger)
	return s, c.Start()
}

func (s *RtcClient) SetExtra(v interface{}) {
	s.extra = v
}
func (s *RtcClient) Extra() interface{} {
	return s.extra
}
func (s *RtcClient) SessionId() int64 {
	return s.sessionId
}
func (s *RtcClient) GetConnWithDeadline() (NetClient.ConnWithDeadline, error) {
	if s.channel == nil {
		return nil, errors.New("channel is nil")
	}
	return nil, nil
}

func (s *RtcClient) SetOnConnect(f NetClient.ConnectEvent) {

}

// OnHandshake 无用
func (s *RtcClient) OnHandshake(f NetClient.Callback) error {
	return nil
}
func (s *RtcClient) SetOnDataCallback(f NetClient.Callback) {
	s.callback = f
}
func (s *RtcClient) SendPing() error {
	if s.channel != nil {
		return s.channel.SendRaw(NetClient.Ping)
	}
	return nil
}

func (s *RtcClient) SendPong() error {
	if s.channel != nil {
		return s.channel.SendRaw(NetClient.Pong)
	}
	return nil
}
func (s *RtcClient) Name() string {
	return name
}

// OnData 无用
func (s *RtcClient) OnData() {
}
func (s *RtcClient) onData(c *webRTC.Client, channelId uint16, label string, msg *webrtc.DataChannelMessage) {
	if len(msg.Data) < 1 {
		return
	}
	if s.callback != nil {
		packet := s.buf.NewPacket(msg.Data)
		defer packet.Put()
		s.callback(packet, s)
	}
}
func (s *RtcClient) SendPacket(p *bufferPool.Packet) error {
	if !s.online {
		return errors.New("connection not online")
	}
	//fmt.Println(s.extra, "send packet", p.Length)
	return s.channel.Send(p.Bytes())
}
func (s *RtcClient) Send(typ uint8, v interface{}, header uint64) (err error) {
	packet := s.buf.Get()
	defer packet.Put()
	err = packet.Write(typ, header, v)
	if err != nil {
		return
	}
	return s.SendPacket(packet)
}
func (s *RtcClient) SendJson(v interface{}, header uint64) (err error) {
	return s.Send(bufferPool.TypeJson, v, header)
}
func (s *RtcClient) SendMsgpack(v interface{}, header uint64) (err error) {
	return s.Send(bufferPool.TypeMsgpack, v, header)
}
func (s *RtcClient) SendBytes(v []byte, header uint64) (err error) {
	return s.Send(bufferPool.TypeBinary, v, header)
}
func (s *RtcClient) SetReadDeadline(t time.Time) error {
	return nil
}
func (s *RtcClient) RemoteAddr() string {
	return ""
}
func (s *RtcClient) RemoteIp() (string, int) {
	return "", 0
}
func (s *RtcClient) Close() error {
	if s.Client != nil {
		s.Client.SetOnChannelOpen(nil)
		s.Client.SetOnChannelClose(nil)
		s.Client.Close()
		s.channel = nil
		s.Client = nil
	}
	return nil
}
