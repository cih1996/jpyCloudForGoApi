package coreClass

import (
	"encoding/binary"
	"encoding/json"
	"portmap"
	"sync"
	"time"

	"github.com/ghp3000/logs"
	"github.com/ghp3000/netclient/bufferPool"
	"github.com/ghp3000/netclient/netclient"
	"github.com/ghp3000/public"
	"github.com/vmihailenco/msgpack/v5"
)

type ForwardRequest struct {
	Token   string `json:"token,omitempty" msgpack:"token,omitempty"` //最终分配的token
	Agent   string `json:"agent,omitempty" msgpack:"agent,omitempty"` //告诉手机,中继服务的地址: 中间件ip:服务端口
	Seat    uint64 `json:"seat,omitempty" msgpack:"seat,omitempty"`   //要对哪个设备端口映射,该字段是为了兼容管理连接来申请token
	Multi   bool   `json:"multi" msgpack:"multi"`                     //是否开启多路复用
	Proto   string `json:"proto" msgpack:"proto"`                     //协议:tcp/udp
	DstPort int    `json:"dstPort" msgpack:"dstPort"`                 //映射给内网的哪个端口
}

type RequestToken struct {
	Guest   string `json:"Guest" msgpack:"g" validate:"printascii,required"`
	Host    string `json:"Host" msgpack:"h" validate:"printascii,required"`
	Timeout int64  `json:"Timeout"  msgpack:"t" validate:"required,gt=1"`                 //申请的token有效期多少秒
	NoRelay bool   `json:"NoRelay,omitempty"  msgpack:"n,omitempty" validate:"omitempty"` //不使用中继，默认false
}
type ResponseToken struct {
	UserId   uint64 `json:"UserId" msgpack:"UserId"`
	DeviceId uint64 `json:"DeviceId" msgpack:"DeviceId"`
	Guest    string `json:"Guest" msgpack:"g"`
	Host     string `json:"Host" msgpack:"h"`
	GuestUrl string `json:"GuestUrl" msgpack:"gu"`
	HostUrl  string `json:"HostUrl" msgpack:"hu"`
	Token    string `json:"Token" msgpack:"tk"`
}

type Message struct {
	F           uint16             `json:"f" msgpack:"f" validate:"required"`                             //函数名
	Req         bool               `json:"req" msgpack:"req" validate:"omitempty"`                        //request:true=请求,false=返回值
	Seq         uint32             `json:"seq" msgpack:"seq" validate:"required"`                         //包序号
	Code        int32              `json:"code,omitempty" msgpack:"code,omitempty"  validate:"omitempty"` //状态码
	Msg         string             `json:"msg,omitempty" msgpack:"msg,omitempty"  validate:"omitempty"`   //状态文本
	T           int32              `json:"t,omitempty" msgpack:"t,omitempty"  validate:"omitempty"`       //time
	DataJson    json.RawMessage    `json:"data,omitempty" msgpack:"-"  validate:"omitempty"`              //数据
	DataMsgpack msgpack.RawMessage `json:"-" msgpack:"data,omitempty"  validate:"omitempty"`              //数据
	Type        uint8              `json:"-" msgpack:"-" validate:"omitempty"`                            //5=msgpack,6=json
}

type PortMapInfo struct {
	Wait          sync.Map
	DeviceId      uint64
	PhonePort     int                 //手机端口
	LocalPort     int                 //本地端口
	PortMapRtc    netclient.NetClient //端口映射Rtc对象
	Portforwarder *portmap.Forwarder  //本地映射服务对象
	LetOpen       int32               //用户意愿,是否开启端口映射
	IsOpen        int32               //是否开启端口映射
}

func (c *Core) LocalPortGetPortMapInfo(LocalPort int) (*PortMapInfo, bool) {
	value, ok := c.MapPorts.Load(LocalPort)
	if ok {
		info := value.(*PortMapInfo)
		return info, ok
	} else {
		return nil, false
	}
}
func (c *Core) LocalPortSavePortMapInfo(LocalPort int, portmap *PortMapInfo) {
	c.MapPorts.Store(LocalPort, portmap)
}
func (c *Core) LocalPortDelPortMapInfo(LocalPort int) {
	c.MapPorts.Delete(LocalPort)
}
func (c *Core) LocalPortGetAll() (LocalPorts []int, Infos []*PortMapInfo) {
	c.MapPorts.Range(func(key, value interface{}) bool {
		LocalPort := key.(int)
		info := value.(*PortMapInfo)
		LocalPorts = append(LocalPorts, LocalPort)
		Infos = append(Infos, info)
		return true
	})
	return LocalPorts, Infos
}
func (s *PortMapInfo) SyncCall(conn netclient.ConnWithDeadline, seat uint8, msg *public.Message, timeout time.Duration) *public.Message {
	if conn == nil {
		msg.SetCode(public.NotOnline)
		return msg
	}
	pkt := bufferPool.Get()
	defer pkt.Put()
	if err := pkt.Write(bufferPool.TypeMsgpack, 0, msg); err != nil {
		msg.SetCode(public.InternalError)
		return msg
	}
	if err := binary.Write(conn, binary.LittleEndian, pkt.Length); err != nil {
		msg.SetCode(public.NotConnect)
		return msg
	}
	if _, err := conn.Write(pkt.Bytes()); err != nil {
		msg.SetCode(public.NotOnline)
	}
	err := conn.SetReadDeadline(time.Now().Add(time.Second * 5))
	if err != nil {
		logs.Info("SetReadDeadline failed: %v", err)
	}
	if err := pkt.ReadFromReader(conn); err != nil {
		msg.SetCode(public.NotConnect)
		return msg
	}
	if err := pkt.Unmarshal(msg); err != nil {
		msg.SetCode(public.InternalError)
		return msg
	}
	return msg
}
