package NetClient

import (
	"time"

	"github.com/ghp3000/netclient/bufferPool"
)

var Ping = []byte{2, 0, 0, 0, bufferPool.TypePing, 0}
var Pong = []byte{2, 0, 0, 0, bufferPool.TypePong, 0}

// Callback OnHandshake时返回true将继续接收,否则认为握手结束.
type Callback func(packet *bufferPool.Packet, c NetClient) bool
type ConnectEvent func(c NetClient)

type NetClient interface {
	SetOnConnect(f ConnectEvent)
	SetExtra(v interface{}) //存放连接id
	Extra() interface{}     //取出连接id
	SessionId() int64
	Name() string
	GetConnWithDeadline() (ConnWithDeadline, error)           //返回一个具有read,write,setdeadline,close的连接,供某些特定的
	OnHandshake(f Callback) error                             //握手函数(鉴权函数)
	SetOnDataCallback(f Callback)                             //设置收到数据的回调函数.
	OnData()                                                  //开始接收数据.此处会阻塞.
	SendPing() error                                          //快速发送ping
	SendPong() error                                          //快速回复ping
	SendPacket(p *bufferPool.Packet) (err error)              //发送数据的函数.发送的是packet类型
	Send(typ uint8, v interface{}, header uint64) (err error) //发送任意数据.需要自己指定详细的参数
	SendJson(v interface{}, header uint64) (err error)        //发送json
	SendMsgpack(v interface{}, header uint64) (err error)     //发送msgpack
	SendBytes(v []byte, header uint64) (err error)            //发送字节类型
	SetReadDeadline(t time.Time) error                        //设置这个客户端的读取死亡线
	RemoteAddr() string                                       //取通信的对方的ip
	RemoteIp() (string, int)                                  //对方ip,但是是分割过ip和端口的
	Close() error                                             //关闭
}
