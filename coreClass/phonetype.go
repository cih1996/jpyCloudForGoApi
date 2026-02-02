package coreClass

import (
	netclient "github.com/ghp3000/netclient/netclient"
)

const (
	MouseButton左键 = 1
	MouseButton右键 = 2
	MouseButton滚轮 = 3
	MouseType按下   = 1
	MouseType移动   = 2
	MouseType释放   = 3
)

type DeviceInfo struct {
	DeviceId       uint64 //设备唯一ID
	BoxNumber      string //设备盒子编号
	Seat           uint64
	DeviceBz       string // 设备备注
	DeviceWidth    int    // 设备屏幕宽度
	DeviceHeight   int    // 设备屏幕高度
	Orientation    int    // 设备屏幕方向
	Online         int32  //在线状态
	Heartbeat      int32  //心跳时间
	WallPicId      int32  //对应的屏幕墙的索引
	Picture        []byte //屏幕墙图片
	TimeDelay      int64  //延迟时间
	H264IsOpen     int32
	H264开启判断计次     int32
	AudioIsOpen    int32
	WindowIsOpen   int32
	WindowISOpened int32
	WindowHwnd     uintptr
	RtcConn        netclient.NetClient
	RtcType        string
	RtcDevice      RtcDeviceInfo
	ReqDevice      ReqPhoneInfo
	Time连接成功时间戳    int64
}

type Send窗口参数 struct {
	UserId         int    `json:"UserId" msgpack:"UserId"`
	DeviceId       uint64 `json:"deviceId" msgpack:"deviceId"`
	Orientation    int    `json:"orientation" msgpack:"orientation"`
	Width          int    `json:"width" msgpack:"width"`
	Height         int    `json:"height" msgpack:"height"`
	LightDakr      int    `json:"lightDakr" msgpack:"lightDakr"`
	WsPort         int    `json:"wsPort" msgpack:"wsPort"`
	DeviceNikeName string `json:"deviceNikeName" msgpack:"deviceNikeName"`
	IsCotrol       bool   `json:"isControl" msgpack:"isControl"`
}
