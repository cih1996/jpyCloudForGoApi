package BufferTCP

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/ghp3000/netclient/bufferPool"
	"github.com/ghp3000/netclient/NetClient"
)

type Opt struct {
	Buf          *bufferPool.Pool // 收发数据的读写器
	Reconnect    uint32
	Delay        time.Duration
	Dialer       func() (net.Conn, error)
	OnConnect    func(c NetClient.NetClient) //连接成功后将启动独立goroutine
	OnDisconnect func(c NetClient.NetClient)
	OnErr        func(err error)
	OnConnData   NetClient.Callback
}
type Client struct {
	conn    *Conn
	running bool
	online  atomic.Bool
	Opt
}

func NewClient(opt Opt) *Client {
	if opt.Buf == nil {
		opt.Buf = bufferPool.Buffer
	}
	if opt.OnConnData == nil {
		panic("OnConnData cannot be nil")
	}
	c := &Client{
		running: true,
		Opt:     opt,
	}
	return c
}
func (c *Client) Connect() {
	c.running = true
	for c.running {
		if c.Dialer == nil {
			time.Sleep(c.Delay)
			atomic.AddUint32(&c.Reconnect, 1)
			continue
		}

		conn, err := c.Dialer()
		if err != nil {
			if c.OnErr != nil {
				c.OnErr(fmt.Errorf("dial error: %v", err))
			}
			if atomic.LoadUint32(&c.Reconnect) == 0 {
				c.Close()
				return
			}
			time.Sleep(c.Delay)
			atomic.AddUint32(&c.Reconnect, 1)
			continue
		}
		newConn := NewConn(conn, bufferPool.Buffer, c.OnConnData)
		c.conn = newConn
		c.online.Store(true)
		if c.OnConnect != nil {
			go c.OnConnect(c.conn)
		}
		newConn.OnData()
		c.online.Store(false)
		if c.OnDisconnect != nil {
			c.OnDisconnect(c.conn)
		}
		time.Sleep(c.Delay)
	}
}
func (c *Client) SetOnConnect(f NetClient.ConnectEvent) {
	c.OnConnect = f
}
func (c *Client) GetConn() (NetClient.NetClient, error) {
	if !c.online.Load() {
		return nil, fmt.Errorf("client not online")
	}
	return c.conn, nil
}
func (c *Client) IsOnline() bool {
	return c.online.Load()
}
func (c *Client) Close() {
	c.running = false
	if c.conn != nil {
		c.conn.Close()
	}
}
