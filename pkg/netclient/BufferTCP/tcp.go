package BufferTCP

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ghp3000/logs"
	"github.com/ghp3000/netclient/bufferPool"
	"github.com/ghp3000/netclient/NetClient"
	"github.com/ghp3000/utils"
)

const name = "tcp"

type Conn struct {
	conn      net.Conn         //客户机连接上来的连接
	buf       *bufferPool.Pool // 收发数据的读写器
	onData    NetClient.Callback
	extra     interface{} //附加数据，可以用来做标志，比如这条连接的用户Id，等
	sessionId int64
	lock      sync.Mutex

	isCached   bool                    //是否启用带缓存模式
	cancelFunc context.CancelFunc      //强行停止
	ch         chan *bufferPool.Packet //发送数据的管道
}

// NewConn 实例化一个连接，注意：并未启动接收数据的线程。需要自己手动  go OnData
func NewConn(conn1 net.Conn, pool *bufferPool.Pool, onData NetClient.Callback) *Conn {
	if pool == nil {
		pool = bufferPool.Buffer
	}
	c := Conn{
		conn:      conn1,
		buf:       pool,
		onData:    onData,
		sessionId: time.Now().UnixNano(),
	}
	//	go c.OnData()
	return &c
}

func NewConnWithCache(conn1 net.Conn, pool *bufferPool.Pool, onData NetClient.Callback) *Conn {
	if pool == nil {
		pool = bufferPool.Buffer
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	c := Conn{
		conn:       conn1,
		buf:        pool,
		onData:     onData,
		sessionId:  time.Now().UnixNano(),
		isCached:   true,
		ch:         make(chan *bufferPool.Packet, 100),
		cancelFunc: cancelFunc,
	}
	go c.sendLoop(ctx)
	return &c
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
		_ = c.conn.SetDeadline(time.Time{})
		if err := recover(); err != nil {
			logs.Debug("%s OnHandshake:", c.conn.RemoteAddr().String(), err)
			c.Close()
		}
	}()
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
		err := packet.ReadFromReader(c.conn)
		if err != nil {
			//logs.Debug(err.Error())
			packet.Put()
			return err
		}
		ok = f(packet, c)
		packet.Put()
		if !ok {
			return nil
		}
	}
}
func (c *Conn) SendPing() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	_, err := c.conn.Write(NetClient.Ping)
	return err
}
func (c *Conn) SendPong() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	_, err := c.conn.Write(NetClient.Pong)
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
func (c *Conn) _sendPacket(p *bufferPool.Packet) (err error) {
	if c == nil || c.conn == nil || c.buf == nil {
		return errors.New("connect is fail")
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	err = binary.Write(c.conn, binary.LittleEndian, p.Length)
	if err != nil {
		return
	}
	_, err = c.conn.Write(p.Bytes())
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
			if err := c._sendPacket(p); err != nil {
				c.Close()
			}
			p.Put() //发送结束,必须释放
		case <-ctx.Done():
			return
		}
	}
}

func (c *Conn) Name() string {
	return name
}

// Send 不需要header的时候给nil
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
func (c *Conn) GetConnWithDeadline() (NetClient.ConnWithDeadline, error) {
	if c.conn == nil {
		return nil, errors.New("conn is nil")
	}
	return c.conn, nil
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
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
