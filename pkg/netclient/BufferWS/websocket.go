package BufferWS

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ghp3000/logs"
	"github.com/ghp3000/netclient/bufferPool"
	"github.com/ghp3000/netclient/NetClient"
	"github.com/ghp3000/utils"
	"github.com/gorilla/websocket"
)

const name = "websocket"

type Conn struct {
	conn      *websocket.Conn  //客户机连接上来的连接
	buf       *bufferPool.Pool // 收发数据的读写器
	onData    NetClient.Callback
	extra     interface{} //附加数据，可以用来做标志，比如这条连接的用户Id，等
	sessionId int64
	lock      sync.Mutex
	r         io.Reader

	isCached   bool                    //是否启用带缓存模式
	cancelFunc context.CancelFunc      //强行停止
	ch         chan *bufferPool.Packet //发送数据的管道
}

// NewConn 实例化一个连接，注意：并未启动接收数据的线程。需要自己手动  go OnData
func NewConn(conn1 *websocket.Conn, pool *bufferPool.Pool, onData NetClient.Callback) *Conn {
	if pool == nil {
		pool = bufferPool.Buffer
	}
	c := Conn{
		conn:      conn1,
		buf:       pool,
		onData:    onData,
		sessionId: time.Now().UnixNano(),
	}
	return &c
}
func NewConnWithCache(conn1 *websocket.Conn, pool *bufferPool.Pool, onData NetClient.Callback) *Conn {
	c := NewConn(conn1, pool, onData)
	c.isCached = true
	c.ch = make(chan *bufferPool.Packet, 100)
	ctx, cancelFunc := context.WithCancel(context.Background())
	c.cancelFunc = cancelFunc
	go c.sendLoop(ctx)
	return c
}
func (c *Conn) SetExtra(v interface{}) {
	c.extra = v
}
func (c *Conn) Extra() interface{} {
	return c.extra
}
func (c *Conn) SessionId() int64 {
	return c.sessionId
}

func (c *Conn) SetOnConnect(f NetClient.ConnectEvent) {

}

func (c *Conn) OnHandshake(f NetClient.Callback) error {
	defer func() {
		if c.conn != nil {
			_ = c.conn.SetReadDeadline(time.Time{})
		}
		if err := recover(); err != nil {
			logs.Debug("%s OnHandshake:", c.conn.RemoteAddr().String(), err)
			c.Close()
		}
	}()
	if c.conn != nil {
		_ = c.conn.SetReadDeadline(time.Now().Add(time.Second * 3))
	}
	return c._onData(f)
}
func (c *Conn) SetOnDataCallback(f NetClient.Callback) {
	c.onData = f
}
func (c *Conn) OnData() {
	defer func() {
		c.Close()
		if err := recover(); err != nil {
			logs.Debug(err)
			logs.Fatal(string(debug.Stack()))
		}
	}()
	_ = c._onData(c.onData)
}
func (c *Conn) _onData(f NetClient.Callback) error {
	if f == nil {
		return errors.New("callback can not be nil")
	}
	var ok bool
	for {
		packet := c.buf.Get()
		messageType, p, err := c.conn.ReadMessage()
		if err != nil {
			//logs.Debug(err.Error())
			packet.Put()
			return err
		}
		if messageType == websocket.BinaryMessage {
			packet.InitWithBytes(p)
		} else if messageType == websocket.TextMessage {
			err = packet.Write(bufferPool.TypeText, 0, p)
		} else {
			continue
		}
		ok = f(packet, c)
		packet.Put()
		if !ok {
			return nil
		}
	}
}
func (c *Conn) Name() string {
	return name
}
func (c *Conn) SendPing() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	err := c.conn.WriteMessage(websocket.BinaryMessage, NetClient.Ping[4:])
	return err
}
func (c *Conn) SendPong() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	err := c.conn.WriteMessage(websocket.BinaryMessage, NetClient.Pong[4:])
	return err
}
func (c *Conn) SendPacket(p *bufferPool.Packet) (err error) {
	if c == nil || c.conn == nil || c.buf == nil {
		return errors.New("connect is fail")
	}
	if c.isCached {
		if !utils.ChannelTryPush(c.ch, p.Ref()) {
			p.Put() //当发送缓冲区满了,发不动了,得把引用释放掉.
			return errors.New("send channel full")
		}
		return nil
	}
	return c._sendPacket(p)
}
func (c *Conn) Read(p []byte) (n int, err error) {
	if c == nil || c.conn == nil {
		return 0, errors.New("connect is fail")
	}
	c.lock.Lock()
	defer c.lock.Unlock()

	// 循环尝试读取，直到读取到数据或遇到非临时错误
	for {
		if c.r == nil {
			_, r, err := c.conn.NextReader()
			if err != nil {
				return 0, err
			}
			c.r = r
		}

		n, err = c.r.Read(p)
		if err == io.EOF { // 当前 reader 已读完，切换到下一个
			c.r = nil
			if n > 0 {
				return n, nil
			}
			continue // 否则继续读取下一个消息
		}
		return n, err
	}
}
func (c *Conn) Write(p []byte) (n int, err error) {
	if c == nil || c.conn == nil || c.buf == nil {
		return 0, errors.New("connect is fail")
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	w, err := c.conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return 0, err
	}
	defer w.Close()
	return w.Write(p)
}
func (c *Conn) GetConnWithDeadline() (NetClient.ConnWithDeadline, error) {
	if c == nil || c.conn == nil || c.buf == nil {
		return nil, errors.New("connect is fail")
	}
	return c, nil
}
func (c *Conn) _sendPacket(p *bufferPool.Packet) (err error) {
	if c == nil || c.conn == nil || c.buf == nil {
		return errors.New("connect is fail")
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	err = c.conn.WriteMessage(websocket.BinaryMessage, p.Bytes())
	return
}
func (c *Conn) sendLoop(ctx context.Context) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	if !c.isCached {
		return
	}
	for {
		select {
		case p := <-c.ch:
			if err := c.conn.WriteMessage(websocket.BinaryMessage, p.Bytes()); err != nil {
				c.Close()
				return
			}
			p.Put() //发送结束,必须释放
		case <-ctx.Done():
			return
		}
	}
}

// Send 不需要header的时候给0
func (c *Conn) Send(typ uint8, v interface{}, header uint64) (err error) {
	packet := c.buf.Get()
	defer packet.Put()
	err = packet.Write(typ, header, v)
	if err != nil {
		return
	}
	return c.SendPacket(packet)
}
func (c *Conn) SendJson(v interface{}, header uint64) (err error) {
	return c.Send(bufferPool.TypeJson, v, header)
}
func (c *Conn) SendMsgpack(v interface{}, header uint64) (err error) {
	return c.Send(bufferPool.TypeMsgpack, v, header)
}
func (c *Conn) SendBytes(v []byte, header uint64) (err error) {
	return c.Send(bufferPool.TypeBinary, v, header)
}
func (c *Conn) SetReadDeadline(t time.Time) error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.SetReadDeadline(t)
}
func (c *Conn) SetWriteDeadline(t time.Time) error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.SetWriteDeadline(t)
}
func (c *Conn) RemoteAddr() string {
	if c == nil || c.conn == nil {
		return ""
	}
	return c.conn.RemoteAddr().String()
}
func (c *Conn) RemoteIp() (string, int) {
	if c == nil || c.conn == nil {
		return "", 0
	}
	addr := c.conn.RemoteAddr().String()
	sl := strings.Split(addr, ":")
	if len(sl) > 1 {
		ip := sl[0]
		port, _ := strconv.Atoi(sl[1])
		return ip, port
	}
	return "", 0
}
func (c *Conn) Close() error {
	defer func() {
		if err := recover(); err != nil {
			//logs.Debug("OnClose:", err)
		}
	}()
	if c.cancelFunc != nil {
		c.cancelFunc()
	}
	if c.isCached {
		close(c.ch)
	}
	if c.r != nil {
		c.r = nil
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
