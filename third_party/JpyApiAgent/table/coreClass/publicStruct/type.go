package publicStruct

import (
	"adminApi/rtcCtl"
	"adminApi/userDeviceCtl"
	"portmap"
	"sync"

	"cnb.cool/accbot/goTool/ErrPkg"
	"github.com/ghp3000/netclient/NetClient"
)

// MiddleAgentTypeInfo 中间件类型信息
type MiddleAgentTypeInfo struct {
	Wait          sync.Map                                               `json:"-" msgpack:"-"`
	DevicesLock   sync.RWMutex                                           `json:"-" msgpack:"-"`
	FuncOpen      NetClient.ConnectEvent                                 `json:"-" msgpack:"-"`
	FuncClose     NetClient.ConnectEvent                                 `json:"-" msgpack:"-"`
	FuncCallBack  NetClient.Callback                                     `json:"-" msgpack:"-"`
	Rtc           NetClient.NetClient                                    `json:"-" msgpack:"-"` //Rtc对象
	Delay         int64                                                  `json:"-" msgpack:"-"` //保存延迟检测数值的函数
	GetToken      func(mid uint64) (*rtcCtl.GetRtcTokenRes, *ErrPkg.Err) `json:"-" msgpack:"-"` //获取中间件RtcToken
	DelMiddleware func(mid uint64)                                       `json:"-" msgpack:"-"` //删除中间件函数
	TokenInfo     *rtcCtl.GetRtcTokenRes                                 `json:"-" msgpack:"-"`
	Reconnect     int32                                                  `json:"-" msgpack:"-"`               //是否自动重连 0:自动重连 1:手动重连
	MiddlewareId  uint64                                                 `json:"middleId" msgpack:"middleId"` //中间件ID 只用于偏移运算
	Devices       []*Device                                              `json:"devices" msgpack:"devices"`
	Code          int32                                                  `json:"code" msgpack:"code"` //Rtc执行状态码  连接建立=1；连接成功=2；连接失败=-2；断开连接=-1
	Msg           string                                                 `json:"msg" msgpack:"msg"`   //执行状态
	Seq           uint32                                                 `json:"-" msgpack:"-"`
}
type PortMapTypeInfo struct {
	Wait         sync.Map                                                                         `json:"-" msgpack:"-"`
	Rtc          NetClient.NetClient                                                              `json:"-" msgpack:"-"` //端口映射Rtc对象
	Forwarder    *portmap.Forwarder                                                               `json:"-" msgpack:"-"` //本地映射服务对象
	GetToken     func(tBYunJiUserDeviceId uint64) (*rtcCtl.GetDeviceRtcPortTokenRes, *ErrPkg.Err) `json:"-" msgpack:"-"` //获取设备端口映射RtcToken
	DelPortMapId func(deviceId uint64)                                                            `json:"-" msgpack:"-"` //删除端口映射函数
	TokenInfo    *rtcCtl.GetDeviceRtcPortTokenRes                                                 `json:"-" msgpack:"-"`
	Reconnect    int32                                                                            `json:"-" msgpack:"-"`                 //是否自动重连 0:自动重连 1:手动重连
	DeviceId     uint64                                                                           `json:"deviceId" msgpack:"deviceId"`   //设备ID
	PhonePort    int                                                                              `json:"phonePort" msgpack:"phonePort"` //手机端口
	LocalPort    int                                                                              `json:"localPort" msgpack:"localPort"` //本地端口
	Mode         int                                                                              `json:"mode" msgpack:"mode"`           //0=连接复用的端口映射,1=无连接复用的端口映射,2=socks5代理
	Code         int32                                                                            `json:"code" msgpack:"code"`           //执行状态码
	Msg          string                                                                           `json:"msg" msgpack:"msg"`             //执行状态
}
type Device struct {
	Wait                sync.Map                                                          `json:"-" msgpack:"-"`
	DeviceStatus        Status                                                            `json:"-" msgpack:"-"` // 设备状态
	FuncOpen            NetClient.ConnectEvent                                            `json:"-" msgpack:"-"`
	FuncClose           NetClient.ConnectEvent                                            `json:"-" msgpack:"-"`
	FuncCallBack        NetClient.Callback                                                `json:"-" msgpack:"-"`
	Rtc                 NetClient.NetClient                                               `json:"-" msgpack:"-"`
	Delay               int64                                                             `json:"-" msgpack:"-"` //保存延迟检测数值的函数
	GetToken            func(deviceId uint64) (*rtcCtl.GetDeviceRtcTokenRes, *ErrPkg.Err) `json:"-" msgpack:"-"` //获取设备串流控制RtcToken
	TokenInfo           *rtcCtl.GetDeviceRtcTokenRes                                      `json:"-" msgpack:"-"` //
	VideoStatus         VideoAudioStatus                                                  `json:"-" msgpack:"-"`
	AudioStatus         VideoAudioStatus                                                  `json:"-" msgpack:"-"`
	Reconnect           int32                                                             `json:"-" msgpack:"-"`                                     //是否自动重连 0:自动重连 1:手动重连
	TBYunJiUserDeviceId int64                                                             `json:"tbYunJiUserDeviceId" msgpack:"tbYunJiUserDeviceId"` //用户设备ID
	DeviceId            uint64                                                            `json:"deviceId" msgpack:"deviceId"`                       //设备ID
	CreatedAt           int64                                                             `json:"createdAt" msgpack:"createdAt"`                     //创建时间
	UserId              int64                                                             `json:"userId" msgpack:"userId"`                           //用户ID
	ExpiresTime         int64                                                             `json:"expiresTime" msgpack:"expiresTime"`                 //过期时间
	BuyType             int8                                                              `json:"buyType" msgpack:"buyType"`                         //购买类型
	YunjiUserGroupId    int64                                                             `json:"yunjiUserGroupId" msgpack:"yunjiUserGroupId"`       //用户组ID
	Remark              string                                                            `json:"remark" msgpack:"remark"`                           //备注
	MiddleAgentDevice   MiddleAgentDeviceInfo                                             `json:"middleAgentDevice" msgpack:"middleAgentDevice"`     //设备信息
	DeviceInfo          userDeviceCtl.GetUserDeviceListTbYunJiDeviceInfo                  `json:"deviceInfo" msgpack:"deviceInfo"`                   //设备信息
	Code                int32                                                             `json:"code" msgpack:"code"`                               //Rtc执行状态码  连接建立=1；连接成功=2；连接失败=-2；断开连接=-1
	Msg                 string                                                            `json:"msg" msgpack:"msg"`                                 //执行状态
	Seq                 uint32                                                            `json:"-" msgpack:"-"`
}

type VideoAudioStatus struct {
	Status    int32 `json:"status" msgpack:"status"`       //视频状态 0:未开始 1:开始
	ReConnect int32 `json:"reConnect" msgpack:"reConnect"` //是否自动重连 0:自动重连 1:手动重连
}

type Status struct {
	Online      *OnlineStatus `json:"online" msgpack:"online"`           // 各种设备是否在线
	Orientation int           `json:"orientation" msgpack:"orientation"` // 屏幕旋转角度 0=正常, 1=90, 2=180, 3=270
}

// MiddleAgentDeviceInfo 中间件交互的设备信息 {"seat":4,"uuid":"3663700C333531","model":"22041216C","osVersion":"Pixel_4a_android_12_20250608_v1.0","androidVersion":"12","brand":"","width":1080,"height":2460,"online":0,"ip":""}
type MiddleAgentDeviceInfo struct {
	Seat           uint8  `json:"seat" msgpack:"seat"`                     //盘位号
	Uuid           string `json:"uuid" msgpack:"uuid"`                     //设备唯一标识
	Model          string `json:"model" msgpack:"model"`                   //设备型号
	OsVersion      string `json:"osVersion" msgpack:"osVersion"`           //操作系统版本
	AndroidVersion string `json:"androidVersion" msgpack:"androidVersion"` //Android版本
	Brand          string `json:"brand" msgpack:"brand"`                   //设备品牌
	Width          int32  `json:"width" msgpack:"width"`                   //屏幕宽度
	Height         int32  `json:"height" msgpack:"height"`                 //屏幕高度
	Online         int32  `json:"online" msgpack:"online"`                 //是否在线
	Ip             string `json:"ip" msgpack:"ip"`                         //IP地址
}

type TouchRequest struct {
	Type     int `msgpack:"type" json:"type"`         // 触摸类型  0=按下，1=抬起，2=移动
	X        int `msgpack:"x" json:"x"`               // X坐标
	Y        int `msgpack:"y" json:"y"`               // Y坐标
	Offset   int `msgpack:"offset" json:"offset"`     // 延迟执行时间
	Pressure int `msgpack:"pressure" json:"pressure"` // 压力值
	Id       int `msgpack:"id" json:"id"`             // 触摸点ID
}
type ScrollRequest struct {
	UpOrDown int `msgpack:"upOrDown" json:"upOrDown"` // 滚动方向 1=上,-1=下,mac系统可能是反的
	X        int `msgpack:"x" json:"x"`               // X坐标
	Y        int `msgpack:"y" json:"y"`               // Y坐标
}

type KeyBoardRequest struct {
	KeyCode int `msgpack:"keyCode" json:"keyCode"` // 键代码
	Action  int `msgpack:"action" json:"action"`   // 操作 0按下1抬起，3按下并抬起，4组合ctrl键
}
type ModeSwitch struct {
	Seat uint8 `msgpack:"seat" json:"seat"` // 盘位号,通过deviceId获取,调用publicStruct.GetSeatFromDeviceId(deviceId)运算获取
	Mode int   `msgpack:"mode" json:"mode"` // 模式 0=otg, 1=usb
}

type DevicePowerControl struct {
	Seat uint8 `msgpack:"seat" json:"seat"` // 盘位号,通过deviceId获取,调用publicStruct.GetSeatFromDeviceId(deviceId)运算获取
	Mode int   `msgpack:"mode" json:"mode"` // 电源状态 0=断电,1=供电,2=强制重启(断电后再供电)
}

type EnterFlashingMode struct {
	Seat uint8 `msgpack:"seat" json:"seat"` // 盘位号,通过deviceId获取,调用publicStruct.GetSeatFromDeviceId(deviceId)运算获取
	Mode int   `msgpack:"mode" json:"mode"` //1=模式1[开关+音量减5秒];2=模式2[开关+音量减9秒];3=模式3[开关+音量加5秒]
}
type SetDeviceToFindMode struct {
	Seat uint8 `msgpack:"seat" json:"seat"` // 盘位号,通过deviceId获取,调用publicStruct.GetSeatFromDeviceId(deviceId)运算获取
	Mode int   `msgpack:"mode" json:"mode"` // 查找模式 0=关闭,1=开启
}

type OnlineInfo struct {
	Online int32  `msgpack:"online" json:"online"` // 在线状态 online<=1离线,online>1在线
	Seat   uint8  `msgpack:"seat" json:"seat"`     // 座位号
	Ip     string `msgpack:"ip" json:"ip"`         // 中间件ID
}

type ScreenRotation struct {
	Orientation int `msgpack:"orientation" json:"orientation"` // 屏幕旋转角度 0=0度,1=90度,2=180度,3=270度
}
type ShellCmd struct {
	Shell string `msgpack:"shell" json:"shell"`
}

type RunApp struct {
	PackageName string `msgpack:"packageName" json:"packageName"` // 应用包名
}
type DownloadAndInstall struct {
	Url     string `json:"url" msgpack:"url"`
	Sha256  string `json:"sha256" msgpack:"sha256"`
	Install bool   `json:"install" msgpack:"install"`
	Name    string `json:"name" msgpack:"name"`
	Receive bool   `json:"receive" msgpack:"receive"`
}

type GetDownloadAndInstallMessage struct {
	Id uint32 `json:"id" msgpack:"id"` //任务id 来源于发起下载任务的Seq
}

type DownloadAndInstallMessage struct {
	Id        int     `json:"id" msgpack:"id"`               //任务id 来源于发起下载任务的Seq
	Url       string  `json:"url" msgpack:"url"`             // 下载地址
	Md5       string  `json:"md5" msgpack:"md5"`             // 任务md5 通sha256校验值
	Status    int     `json:"status" msgpack:"status"`       //任务状态 -1失败 ,0排队中,1正在下载,2等待重试,3下载完成,4安装成功,-2安装失败
	LastTime  int64   `json:"lastTime" msgpack:"lastTime"`   // 最后更新时间
	BeginTime int64   `json:"beginTime" msgpack:"beginTime"` // 开始时间
	Process   float64 `json:"process" msgpack:"process"`     // 下载进度 0-1的小数
	Path      string  `json:"path" msgpack:"path"`           // 安装路径
	Install   bool    `json:"install" msgpack:"install"`     // 是否安装
}

type OnlineStatus struct {
	Manager       bool `msgpack:"manager" json:"manager"`             // 管理连接是否在线
	Business      bool `msgpack:"business" json:"business"`           // 业务连接是否在线
	Authorization bool `msgpack:"authorization" json:"authorization"` // 授权状态是否异常
	ControlBoard  bool `msgpack:"controlBoard" json:"controlBoard"`   // 控制小板是否在线
	Power         bool `msgpack:"power" json:"power"`                 // 手机是否断电
	UsbObject     bool `msgpack:"usbObject" json:"usbObject"`         // USB是否设定为usb&otg
	UsbCurrent    bool `msgpack:"usbCurrent" json:"usbCurrent"`       // USB当前状态是否为usb&otg
	Fastboot      bool `msgpack:"fastboot" json:"fastboot"`           // Fastboot是否在线
	Adb           bool `msgpack:"adb" json:"adb"`                     // Adb是否在线
	Itunes        bool `msgpack:"itunes" json:"itunes"`               // Itunes是否在线
	UnSupport     bool `msgpack:"unsupport" json:"unsupport"`         // 是否不支持
}

type ScreenOnOff struct {
	State    string `msgpack:"state" json:"state"`       // 屏幕状态 off=关闭,on=开启
	TimeLong int64  `msgpack:"timeLong" json:"timeLong"` //默认 31536000
}

type RootApp struct {
	Pkg string `msgpack:"pkg" json:"pkg"` // 应用包名
}

type SetSwitchBack struct {
	Type   string `json:"type" msgpack:"type"`
	Params struct {
		SwitchBack *int `json:"switchBack" msgpack:"switchBack"`
	} `json:"params" msgpack:"params"`
}
type SetSwitchFront struct {
	Type   string `json:"type" msgpack:"type"`
	Params struct {
		SwitchFront *int `json:"switchFront" msgpack:"switchFront"`
	} `json:"params" msgpack:"params"`
}

type ImageRequest struct {
	Width  int `json:"width" msgpack:"width"`
	Height int `json:"height" msgpack:"height"`
	Qua    int `json:"qua" msgpack:"qua"`
	Scale  int `json:"scale" msgpack:"scale"`
	X      int `json:"x" msgpack:"x"`
	Y      int `json:"y" msgpack:"y"`
	Type   int `json:"type" msgpack:"type"` //2=webp 格式 ；其他=jpg
}
type DelayRequest struct {
	Timestamp int64 `json:"timestamp" msgpack:"timestamp"`
}

type Base64Image struct {
	Base64Img string `json:"base64Img" msgpack:"base64Img"`
}

type EnterText struct {
	Text string `msgpack:"text" json:"text"` // 输入的文本
}

type H264StreamRequest struct {
	FPS     int   `json:"fps" msgpack:"fps"`         // 帧率
	Bitrate int   `json:"bit" msgpack:"bit"`         // 码率
	Quality int   `json:"quality" msgpack:"quality"` // 质量
	Width   int32 `json:"width" msgpack:"width"`     // 宽度
}
type AudioStreamRequest struct {
	SampleRate   int `json:"sampleRate" msgpack:"sampleRate"`     // 采样率
	AudioBitRate int `json:"audioBitRate" msgpack:"audioBitRate"` // 音频码率
}

func GetMiddleIdFromDeviceId(deviceId uint64) uint64 {
	mid := deviceId >> 8 //中间件授权id
	return mid
}
func GetSeatFromDeviceId(deviceId uint64) uint8 {
	seat := deviceId & 0xFF //座位号
	return uint8(seat)
}
func GetDeviceIdFromMiddleIdAndSeat(mid uint64, seat uint8) uint64 {
	deviceId := (mid << 8) + uint64(seat)
	return deviceId
}

func OnlineJudgment(online int32) *OnlineStatus {
	var status OnlineStatus
	if online&1 > 0 {
		status.Manager = true
	} else {
		status.Manager = false
	}
	if online&2 > 0 {
		status.Business = true
	} else {
		status.Business = false
	}
	if online&4 > 0 {
		status.Authorization = true
	} else {
		status.Authorization = false
	}
	if online&8 > 0 {
		status.ControlBoard = true
	} else {
		status.ControlBoard = false
	}
	if online&16 > 0 {
		status.Power = true
	} else {
		status.Power = false
	}
	if online&32 > 0 {
		status.UsbObject = true
	} else {
		status.UsbObject = false
	}
	if online&64 > 0 {
		status.UsbCurrent = true
	} else {
		status.UsbCurrent = false
	}
	if online&128 > 0 {
		status.Fastboot = true
	} else {
		status.Fastboot = false
	}
	if online&256 > 0 {
		status.Adb = true
	} else {
		status.Adb = false
	}
	if online&512 > 0 {
		status.Itunes = true
	} else {
		status.Itunes = false
	}
	if online&1024 > 0 {
		status.UnSupport = true
	} else {
		status.UnSupport = false
	}
	return &status
}
