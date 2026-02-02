package BufferRTC

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/ghp3000/logs"
	"github.com/ghp3000/netclient/bufferPool"
	"github.com/ghp3000/netclient/netclient"
	"github.com/ghp3000/netclient/streamRTC"
)

type StreamClient struct {
	buf *bufferPool.Pool // 收发数据的读写器

	channel *streamRTC.DataChannelStream
	onOpen  netclient.ConnectEvent
	onClose netclient.ConnectEvent

	onData     netclient.Callback
	isCached   bool                    //是否启用带缓存模式
	cancelFunc context.CancelFunc      //强行停止
	ch         chan *bufferPool.Packet //发送数据的管道

	online    bool
	extra     interface{} //附加数据，可以用来做标志，比如这条连接的用户Id，等
	sessionId int64
	Client    *streamRTC.Client

	lock sync.Mutex //发送锁
}

func NewStreamClient(id interface{}, urlString string, token string, isGuest bool, pool *bufferPool.Pool, onOpen, onClose netclient.ConnectEvent, callback netclient.Callback) (*StreamClient, error) {
	if pool == nil {
		pool = bufferPool.Buffer
	}
	s := &StreamClient{
		extra:     id,
		buf:       pool,
		sessionId: time.Now().UnixNano(),
		onOpen:    onOpen,
		onClose:   onClose,
		onData:    callback}
	c := streamRTC.NewClient(urlString, token, isGuest, s.onDataChannelOpen, s.onDataChannelClose)
	c.Attach = id
	//c.SetLogger(streamRTC.DefaultLogger)
	//c.SetOnClose(func(c *streamRTC.Client) {
	//	s.online = false
	//	if onClose != nil {
	//		onClose(s)
	//	}
	//})
	s.Client = c
	return s, c.Start()
}
func NewStreamClientWithCache(id interface{}, urlString string, token string, isGuest bool, pool *bufferPool.Pool, onOpen, onClose netclient.ConnectEvent, callback netclient.Callback) (*StreamClient, error) {
	if pool == nil {
		pool = bufferPool.Buffer
	}
	s := &StreamClient{
		extra:     id,
		buf:       pool,
		isCached:  true,
		sessionId: time.Now().UnixNano(),
		ch:        make(chan *bufferPool.Packet, 100),
		onData:    callback,
	}
	c := streamRTC.NewClient(urlString, token, isGuest, s.onDataChannelOpen, s.onDataChannelClose)
	c.Attach = id
	c.SetLogger(streamRTC.DefaultLogger)
	c.SetOnClose(func(c *streamRTC.Client) {
		s.online = false
		if onClose != nil {
			onClose(s)
		}
	})
	s.Client = c
	ctx, cancelFunc := context.WithCancel(context.Background())
	s.cancelFunc = cancelFunc
	go s.sendLoop(ctx)
	return s, c.Start()
}

func (s *StreamClient) onDataChannelOpen(c *streamRTC.Client, dc *streamRTC.DataChannelStream) {
	s.Client = c
	s.channel = dc
	s.online = true
	if s.onOpen != nil {
		s.onOpen(s)
	}
}
func (s *StreamClient) onDataChannelClose(c *streamRTC.Client, ch *streamRTC.DataChannelStream) {
	s.online = false
	if s.onClose != nil {
		s.onClose(s)
	}
}
func (s *StreamClient) Name() string {
	return "detach webrtc"
}
func (s *StreamClient) SetExtra(v interface{}) {
	s.extra = v
}
func (s *StreamClient) Extra() interface{} {
	return s.extra
}
func (s *StreamClient) SessionId() int64 {
	return s.sessionId
}
func (s *StreamClient) SetOnConnect(f netclient.ConnectEvent) {
	s.onOpen = f
}

func (s *StreamClient) SetOnDataCallback(f netclient.Callback) {
	s.onData = f
}
func (s *StreamClient) OnData() {
	defer func() {
		s.Close()
		if err := recover(); err != nil {
			logs.Debug(err)
			logs.Fatal(string(debug.Stack()))
		}
	}()
	if err := s._onData(s.onData); err != nil {
		fmt.Println(err)
	}
}
func (s *StreamClient) OnHandshake(f netclient.Callback) error {
	defer func() {
		_ = s.channel.SetReadDeadline(time.Time{})
		if err := recover(); err != nil {
			logs.Debug("%s OnHandshake:", err)
			s.Close()
		}
	}()
	return s._onData(f)
}
func (s *StreamClient) _onData(f netclient.Callback) error {
	if f == nil {
		return errors.New("callback can not be nil")
	}
	var ok bool
	for {
		packet := s.buf.Get()
		err := packet.ReadFromReader(s.channel)
		if err != nil {
			packet.Put()
			return err
		}
		ok = f(packet, s)
		packet.Put()
		if !ok {
			return nil
		}
	}
}
func (s *StreamClient) SendPing() error {
	if !s.online || s.channel == nil {
		return errors.New("connection not online")
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	_, err := s.channel.Write(netclient.Ping)
	return err
}
func (s *StreamClient) SendPong() error {
	if !s.online || s.channel == nil {
		return errors.New("connection not online")
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	_, err := s.channel.Write(netclient.Pong)
	return err
}

func (s *StreamClient) GetConnWithDeadline() (netclient.ConnWithDeadline, error) {
	if s.channel == nil {
		return nil, errors.New("channel is nil")
	}
	return s.channel, nil
}
func (s *StreamClient) SendPacket(p *bufferPool.Packet) error {
	if !s.online || s.channel == nil {
		return errors.New("connection not online")
	}
	if s.isCached { //带缓存发送
		select {
		case s.ch <- p.Ref():
		default:
			p.Put() //当发送缓冲区满了,发不动了,得把引用释放掉.
			return errors.New("send channel full")
		}
		return nil
	}
	return s._sendPacket(p) //同步发送
}
func (s *StreamClient) _sendPacket(p *bufferPool.Packet) error {
	if !s.online || s.channel == nil {
		return errors.New("connection not online")
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	err := binary.Write(s.channel, binary.LittleEndian, p.Length)
	if err != nil {
		return err
	}
	_, err = s.channel.Write(p.Bytes())
	return err
}
func (s *StreamClient) sendLoop(ctx context.Context) error {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	if !s.isCached {
		return nil
	}
	for {
		select {
		case p := <-s.ch:
			if err := s._sendPacket(p); err != nil {
				s.Close()
				return err
			}
			p.Put() //发送结束,必须释放
		case <-ctx.Done():
			return nil
		}
	}
}
func (s *StreamClient) Send(typ uint8, v interface{}, header uint64) (err error) {
	packet := s.buf.Get()
	defer packet.Put()
	err = packet.Write(typ, header, v)
	if err != nil {
		return
	}
	return s.SendPacket(packet)
}
func (s *StreamClient) SendJson(v interface{}, header uint64) (err error) {
	return s.Send(bufferPool.TypeJson, v, header)
}
func (s *StreamClient) SendMsgpack(v interface{}, header uint64) (err error) {
	return s.Send(bufferPool.TypeMsgpack, v, header)
}
func (s *StreamClient) SendBytes(v []byte, header uint64) (err error) {
	return s.Send(bufferPool.TypeBinary, v, header)
}
func (s *StreamClient) SetReadDeadline(t time.Time) error {
	if !s.online || s.channel == nil {
		return errors.New("connection not online")
	}
	return s.channel.SetReadDeadline(t)
}
func (s *StreamClient) RemoteAddr() string {
	return ""
}
func (s *StreamClient) RemoteIp() (string, int) {
	return "", 0
}
func (s *StreamClient) Close() error {
	defer func() { recover() }()
	s.online = false

	if s.cancelFunc != nil {
		s.cancelFunc()
	}
	if s.isCached {
		close(s.ch)
	}
	if s.Client != nil {
		s.Client.Close()
		//s.Client.SetOnChannelOpen(nil)
		//s.Client.SetOnChannelClose(nil)
		//s.channel = nil
		//s.Client = nil
	}
	return nil
}
