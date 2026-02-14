package coreClass

import (
	"bytes"
	"image"
	"image/jpeg"
	"port-mapping-demo/Packages/websocketClient"
	"sync"

	"github.com/ghp3000/netclient/netclient"
	"github.com/gin-gonic/gin"
)

const (
	Rtc屏幕墙取webp = 2

	Rtc延迟检测间隔  = 3000
	Rtc心跳间隔    = 3
	Middle心跳间隔 = 3

	TouchType鼠标左键  = 0
	TouchType鼠标滚轮  = 1
	TouchType鼠标右键  = 2
	TouchType键盘输入法 = 3
	TouchType键盘键代码 = 4
)

var (
	S *Core
	startHeartbeatOnce sync.Once
)

type Core struct {
	WsServer    *gin.Engine
	WsPort      int // WebSocket服务器使用的端口号
	TouchChan   chan Touch
	ControlChan chan Control

	WssClient *websocketClient.WSClient
	Devices   []ReqPhoneInfo // 存储设备信息的切片

	MiddleRtcMap  sync.Map //midid 到 conn的映射
	PhoneMap      sync.Map //deviceid 到 phoneInfo的映射
	DownloadMap   sync.Map //deviceId 到 下载信息的映射
	AppList       sync.Map //  app信息
	ShareToUsers  sync.Map //分享人列表
	ShellRec      sync.Map //shell 命令执行记录
	MapPorts      sync.Map //端口映射表
	ShellList     ShellOrderList
	CheckList列表序号 []int32

	wsUrl                string
	Login                login
	LoginRes             loginRes
	DeviceGroups         []DeviceGroup
	UserInfo             UserInformation
	UserMessage          UserPoint
	Points               integral      //积分信息
	FileLists            FileList      //文件列表
	PointsRecords        buyRecord     //积分充值记录
	OrderRecords         Order         //订单购买记录
	ProductLists         ProductList   //商品列表
	IsBuyProductLists    ProductList   //已购买的商品列表
	IsBuyDeviecs         BuyListDevice //已购买的设备列表
	ProductDetails       ProductInfo   //商品详情
	Http订单详情数据           Http订单详情
	Http备份数据列表           BeiFenDeviceList
	UPFileLists          sync.Map
	VisualPhoneDeviceIds []uint64
	IsClose              bool
	CodeId               string //验证码id

	//增加callback
	CallBackIP配置界面回调 func(data string, msgValue string, code int)
	CallBackIP配置界面打开 int32

	CallBack修改设备信息回调 func(data []byte)
	CallBack批量修改定位   int32
	CallBack批量改机     int32

	CallBack端口映射表          func(port int, deviceId uint64)
	Callback端口映射界面打开       int32
	CallbackSetWall        func() //设置屏幕墙的回调
	CallbackWallPicture    func(deviceId int)
	Callback设备列表回调         func(deviceId uint64)
	Callback分享界面刷新         func()
	Callback分享界面打开         int32
	MsgSeq                 uint32
	SyncCallbacks          sync.Map // seq -> chan interface{}
	SyncCallbacksByDeviceF sync.Map // key: "deviceId-F" -> chan interface{}
	LogoutCallback         func()
	ReconnectCallback      func(proxyId uint64)
	CallbackUnifiedMiddlewareMessage func(proxyId uint64, deviceId uint64, msgType int, data interface{})
	ReconnectInProgress    sync.Map // proxyId -> struct{}, to avoid concurrent reconnect storms

	R系统名称             string
	HttpUrl           string
	WsUrl             string
	R系统TenantName     string
	Rtc屏幕墙取图方式        int
	R验证模式             string
	输入法包名             string
	S屏幕墙角度orientation int32
}

type MiddleRtc struct {
	middleId  uint64
	Conn      netclient.NetClient
	DeviceIds []uint64
	Heartbeat int32
	Let设备列表   int32
	Let在线列表   int32
	Let单设备信息  int32
	Let常亮     int32
}
type C还原App选择信息 struct {
	Apk   string
	Apk序号 int
	C选中ID int
}

type S5type struct {
	Id          string `json:"id" msgpack:"id"`
	Sk5Server   string `json:"sk5Server" msgpack:"sk5Server"`
	Sk5Port     string `json:"sk5Port" msgpack:"sk5Port"`
	Sk5Username string `json:"sk5Username" msgpack:"sk5Username"`
	Sk5Password string `json:"sk5Password" msgpack:"sk5Password"`
}

// Req单台设备信息 {f:4,data:{"sysVersion":"android14","country":"United States(US)","appVersion":"","orientation":1,"os":"android","timezone":"Greenwich Mean Time","sysPer":1,"uuid":"aaaaa","seat":1,"osVersion":"Pixel_4a5g_android_14_20241124_v1.0","androidVersion":"14","vendor":"OPPO PHT110","width":1080,"signalMode":"","self":"1.1","model":"PHT110","location":"{}","lang":"en","brand":"OPPO","height":2520},req:false,seq:1,code:0,msg:"xxx"}
type Req单台设备信息 struct {
	F    int       `json:"f" form:"f"`
	Data Phone单台信息 `json:"data" msgpack:"data"`
	Req  bool      `json:"req" msgpack:"req"`
	Seq  int       `json:"seq" msgpack:"seq"`
	Code int       `json:"code" msgpack:"code"`
	Msg  string    `json:"msg" msgpack:"msg"`
}

type SeatInfo struct {
	Seat    uint64 `json:"seat"`
	Online  int    `json:"online"`
	Ip      string `json:"ip"`
	ProxyId uint64 `json:"proxyId"`
}

type Phone单台信息 struct {
	SysVersion     string `json:"sysVersion" msgpack:"sysVersion"` //系统版本
	Country        string `json:"country" msgpack:"country"`       //国家/地区
	AppVersion     string `json:"appVersion" msgpack:"appVersion"`
	Orientation    int    `json:"orientation" msgpack:"orientation"`
	Os             string `json:"os" msgpack:"os"`
	Timezone       string `json:"timezone" msgpack:"timezone"`
	SysPer         int    `json:"sysPer" msgpack:"sysPer"`
	Uuid           string `json:"uuid" msgpack:"uuid"`
	Seat           uint8  `json:"seat" msgpack:"seat"`
	OsVersion      string `json:"osVersion" msgpack:"osVersion"`
	AndroidVersion string `json:"androidVersion" msgpack:"androidVersion"`
	Vendor         string `json:"vendor" msgpack:"vendor"`
	Width          int    `json:"width" msgpack:"width"`
	SignalMode     string `json:"signalMode" msgpack:"signalMode"`
	Self           string `json:"self" msgpack:"self"`
	Model          string `json:"model" msgpack:"model"`
	Location       string `json:"location" msgpack:"location"`
	Lang           string `json:"lang" msgpack:"lang"`
	Brand          string `json:"brand" msgpack:"brand"`
	Height         int    `json:"height" msgpack:"height"`
}

type RtcDeviceInfo struct {
	AndroidVersion string `msgpack:"androidVersion" json:"androidVersion"`
	CPU            int    `msgpack:"cpu" json:"cpu"`
	DiskSize       int    `msgpack:"diskSize" json:"diskSize"`
	Height         int    `msgpack:"height" json:"height"`
	Memory         int    `msgpack:"memory" json:"memory"`
	Model          string `msgpack:"model" json:"model"`
	Seat           uint8  `msgpack:"seat" json:"seat"`
	UUID           string `msgpack:"uuid" json:"uuid"`
	Width          int    `msgpack:"width" json:"width"`
	// 新增字段
	SysVersion  string `msgpack:"sysVersion" json:"sysVersion"` //系统版本
	Country     string `msgpack:"country" json:"country"`       //国家/地区
	AppVersion  string `msgpack:"appVersion" json:"appVersion"`
	Orientation int    `msgpack:"orientation" json:"orientation"`
	Os          string `msgpack:"os" json:"os"`
	Timezone    string `msgpack:"timezone" json:"timezone"`
	SysPer      int    `msgpack:"sysPer" json:"sysPer"`
	OsVersion   string `msgpack:"osVersion" json:"osVersion"`
	Vendor      string `msgpack:"vendor" json:"vendor"`
	SignalMode  string `msgpack:"signalMode" json:"signalMode"`
	Self        string `msgpack:"self" json:"self"`
	Location    string `msgpack:"location" json:"location"`
	Lang        string `msgpack:"lang" json:"lang"`
	Brand       string `msgpack:"brand" json:"brand"`
}

// {"userDeviceId":3847,"groupId":2,"deviceBz":null,"nowTime":1749886834,"proxyId":71,"deviceToken":"efa76ac9","deviceId":18267,"onLine":1,"overTime":1752385670,"ip":"36.251.67.28","intranetIp":"192.168.255.3","intranetPort":9011,"port":"9011","xh":"2109119BC","sysPer":2,"pingpai":null,"buyGroupId":0,"sharedBy":null,"shareToUsers":[],"productId":23,"thaliId":34,"deviceIp":"192.168.12.91","sysVersion":"13","deviceWidth":1080,"deviceHeight":2400,"orientation":null,"boxNumber":"HXY025","portNumber":"11","buyLock":null,"username":null}
type ReqPhoneInfo struct {
	DeviceId     uint64 `json:"deviceId"`     //设备唯一ID
	DeviceBz     string `json:"deviceBz"`     // 设备备注
	ProxyId      uint64 `json:"proxyId"`      //中间件ID
	DeviceWidth  int    `json:"deviceWidth"`  // 设备屏幕宽度
	DeviceHeight int    `json:"deviceHeight"` // 设备屏幕高度
	Orientation  int    `json:"orientation"`  // 设备屏幕方向
	GroupId      int    `json:"groupId"`      //分组ID
	DeviceToken  string `json:"deviceToken"`  //设备令牌
	OnLine       int32  `json:"onLine"`       //在线状态
	NowTime      int64  `json:"nowTime"`      //当前时间戳
	OverTime     int64  `json:"overTime"`     //过期时间戳
	Ip           string `json:"ip"`           //中间件内网IP
	Port         string `json:"port"`         //中间件内网端口
	IntranetIp   string `json:"intranetIp"`   // 中间件外网IP
	IntranetPort int    `json:"intranetPort"` // 中间件外网端口
	DeviceIp     string `json:"deviceIp"`     //设备内网IP
	SysVersion   string `json:"sysVersion"`   //系统版本
	Xh           string `json:"xh"`           //设备型号
	SysPer       int    `json:"sysPer"`       //系统权限
	BuyGroupId   int    `json:"buyGroupId"`   //购买分组ID
	ProductId    int    `json:"productId"`    //产品ID
	ThaliId      int    `json:"thaliId"`      //套餐ID
	BoxNumber    string `json:"boxNumber"`    // 盒子编号
	PortNumber   uint64 `json:"portNumber"`   // 盒子端口号
	Pingpai      string `json:"pingpai"`      // 设备品牌
	SharedBy     int    `json:"sharedBy"`     // 共享者
	ShareToUsers []int  `json:"shareToUsers"` //分享给的用户列表
	UserDeviceId int    `json:"userDeviceId"` //用户设备ID
	BuyLock      uint64 `json:"buyLock"`
	Username     string `json:"username"`
}
type DelayRequest struct {
	Timestamp int64 `json:"timestamp" msgpack:"timestamp"`
}
type login struct {
	UserName   string `json:"userName"`
	PassWord   string `json:"passWord"`
	TenantName string `json:"tenantName"`
	AutoLogin  bool   `json:"autoLogin"`
	TenantId   int    `json:"tenantId"`
}
type loginRes struct {
	UserId       int    `json:"userId"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresTime  int64  `json:"expiresTime"`
}
type WakeUpAways struct {
	State    string `msgpack:"state"`
	TimeLong uint64 `msgpack:"timeLong"`
}
type DeviceGroup struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	UserId      int    `json:"userId"`
	CreateTime  int64  `json:"createTime"`
	PhoneNumber int    `json:"phoneNumber"`
}
type UserInformation struct {
	Id         int    `json:"id"`
	Username   string `json:"username"`
	Nickname   string `json:"nickname"`
	Email      string `json:"email"`
	Mobile     string `json:"mobile"`
	Sex        int    `json:"sex"`
	Avatar     string `json:"avatar"`
	LoginIp    string `json:"loginIp"`
	LoginDate  int64  `json:"loginDate"`
	CreateTime int64  `json:"createTime"`
	Roles      []struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
	}
	Dept        interface{} `json:"dept"`
	Posts       interface{} `json:"posts"`
	SocialUsers []struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
	} `json:"socialUsers"`
}
type UserPoint struct {
	UserId     int   `json:"userId"`
	Username   int   `json:"username"`
	Integral   int   `json:"integral"`
	CreateTime int64 `json:"createTime"`
}
type WssMessage struct {
	Type    string `json:"type"`    // 消息类型
	Content string `json:"content"` // 消息内容
}
type RtcToken struct {
	UserId   uint64 `json:"UserId" msgpack:"UserId"`     //用户id
	DeviceId uint64 `json:"DeviceId" msgpack:"DeviceId"` //uint64(证书发行序号)<<8 + uint8(Seat)
	HostUrl  string `json:"HostUrl" msgpack:"HostUrl"`
	Token    string `json:"Token" msgpack:"Token"`
	GuestUrl string `json:"GuestUrl" msgpack:"GuestUrl"`
	Guest    string `json:"Guest" msgpack:"Guest"`
	Host     string `json:"Host" msgpack:"Host"`
}
type FileList struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		List []struct {
			Id         int    `json:"id"`
			CreateTime int64  `json:"createTime"`
			FileName   string `json:"fileName"`
			Url        string `json:"url"`
			FileSize   int    `json:"fileSize"`
			FileType   string `json:"fileType"`
		}
		Total int `json:"total"`
	}
}
type integral struct {
	Code int `json:"code"`
	Data struct {
		UserId     int   `json:"userId"`
		Username   int   `json:"username"`
		Integral   int   `json:"integral"`
		CreateTime int64 `json:"createTime"`
	}
}
type buyRecord struct {
	Code int `json:"code"`
	Data struct {
		List []struct {
			Id         int    `json:"id"`
			Integral   int    `json:"integral"`
			Money      int    `json:"money"`
			MoneyType  int    `json:"moneyType"`
			CreateTime string `json:"createTime"`
			OrderId    int    `json:"orderId"`
		}
		Total int `json:"total"`
	}
	Msg string `json:"msg"`
}
type Order struct {
	Code int `json:"code"`
	Data struct {
		List []struct {
			Id                int     `json:"id"`
			Money             float32 `json:"money"`
			PayType           int     `json:"payType"`
			PayLog            string  `json:"payLog"`
			Type              int     `json:"type"`
			OrderSn           string  `json:"orderSn"`
			UserId            int     `json:"userId"`
			OrderState        int     `json:"orderState"`
			PayState          int     `json:"payState"`
			CreateTime        int64   `json:"createTime"`
			PayTime           int64   `json:"payTime"`
			ChargeBackReason  string  `json:"chargeBackReason"`
			ChargeBackTime    int64   `json:"chargeBackTime"`
			ChargeBackHandler string  `json:"chargeBackHandler"`
			DeviceIds         []int   `json:"deviceIds"`
		}
		Total int `json:"total"`
	}
	Msg string `json:"msg"`
}
type Up文件信息 struct {
	C序号    int32
	C文件路径  string
	C文件名称  string
	C文件大小  int64
	C进度    string
	Sha256 string
	UpLoad UpLoad
	C状态    int32 //0 未上传 1 上传中 2 上传成功 3 上传失败
}
type UpLoad struct {
	Code int `json:"code"`
	Data struct {
		ConfigId  int    `json:"configId"`
		UploadUrl string `json:"uploadUrl"`
		Url       string `json:"url"`
	}
	Msg string `json:"msg"`
}
type Mousescroll struct {
	UpOrDown int `msgpack:"upOrDown"`
	X        int `msgpack:"x"`
	Y        int `msgpack:"y"`
}
type keyboard struct {
	Action  int `msgpack:"action"`
	KeyCode int `msgpack:"keyCode"`
}
type TouchRequest struct {
	Type     int `msgpack:"type"`     // 触摸类型
	X        int `msgpack:"x"`        // X坐标
	Y        int `msgpack:"y"`        // Y坐标
	Offset   int `msgpack:"offset"`   // 偏移量
	Pressure int `msgpack:"pressure"` // 压力值
	Id       int `msgpack:"id"`       // 触摸点ID
}
type H264StreamRequest struct {
	FPS     int `json:"fps" msgpack:"fps"`         // 帧率
	Bitrate int `json:"bit" msgpack:"bit"`         // 码率
	Quality int `json:"quality" msgpack:"quality"` // 质量
	Width   int `json:"width" msgpack:"width"`     // 宽度
}
type AudioStreamRequest struct {
	SampleRate   int `json:"sampleRate" msgpack:"sampleRate"`     // 采样率
	AudioBitRate int `json:"audioBitRate" msgpack:"audioBitRate"` // 音频码率
}
type shellcmd struct {
	Shell string `msgpack:"shell"`
}

type GenericCommand struct {
	F    uint16      `json:"f" msgpack:"f"`
	Data interface{} `json:"data" msgpack:"data"`
	Req  bool        `json:"req" msgpack:"req"`
	Seq  uint32      `json:"seq" msgpack:"seq"`
}

type methodInput struct {
	Text string `msgpack:"text"`
}

type Touch struct {
	Type      int //0=鼠标左键 1=鼠标滚轮 2=鼠标右键 3=键盘输入法 4=键盘键代码 5=命令文本
	DeviceIds []uint64
	TouchInfo TouchRequest
	Mouse滚轮   Mousescroll
	Key键盘     keyboard
	Key输入文本   string
	Key命令文本   string
}

type Setting0 struct {
	Type   string `json:"type" msgpack:"type"`
	Params struct {
		SwitchBack *int `json:"switchBack" msgpack:"switchBack"`
	} `json:"params" msgpack:"params"`
}
type Setting1 struct {
	Type   string `json:"type" msgpack:"type"`
	Params struct {
		SwitchFront *int `json:"switchFront" msgpack:"switchFront"`
	} `json:"params" msgpack:"params"`
}

// {"switchBack":null}
type SwitchBack struct {
	SwitchBack bool `json:"switchBack"`
}

type Control struct {
	Type     int //0=命令文本
	DeviceId uint64
	Shell    string
}

// ProductList {"code":0,"data":{"list":[{"id":14,"name":"test","brief":"<p><br></p>","content":"<p><br></p>","createTime":1744679680000}],"total":3},"msg":"成功"}
type ProductList struct {
	Code int `json:"code"`
	Data struct {
		List []struct {
			Id         int    `json:"id"`
			Name       string `json:"name"`
			Brief      string `json:"brief"`
			Content    string `json:"content"`
			CreateTime int64  `json:"createTime"`
			WallId     int32  `json:"wallId"`
		} `json:"list"`
		Total int `json:"total"`
	} `json:"data"`
	Msg string `json:"msg"`
}

// ProductInfo {"code":0,"data":{"product":{"id":16,"name":"Android14","brief":"<p><strong>冠绝古今的强悍手机，高档骁驴888，</strong></p><p><strong>优势在我！~</strong></p>","content":"<p>冠绝古今的强悍手机，高档骁驴888，</p><p>优势在我！~</p>","createTime":1744960336000},"thaliList":[{"id":16,"name":"3hour","brief":null,"content":null,"price":1.00,"createTime":1744960360000,"discount":1.00,"productId":16,"timeLong":180,"status":0}],"canSellCount":18},"msg":"成功"}
type ProductInfo struct {
	Code int `json:"code"`
	Data struct {
		Product struct {
			Id         int    `json:"id"`
			Name       string `json:"name"`
			Brief      string `json:"brief"`
			Content    string `json:"content"`
			CreateTime int64  `json:"createTime"`
			WallId     int32  `json:"wallId"`
		} `json:"product"`
		ThaliList []struct {
			Id         int     `json:"id"`
			Name       string  `json:"name"`
			Brief      string  `json:"brief"`
			Content    string  `json:"content"`
			Price      float32 `json:"price"`
			CreateTime int64   `json:"createTime"`
			Discount   float32 `json:"discount"`
			ProductId  int     `json:"productId"`
			TimeLong   int     `json:"timeLong"`
			Status     int     `json:"status"`
		} `json:"thaliList"`
		CanSellCount int `json:"canSellCount"`
	} `json:"data"`
	Msg string `json:"msg"`
}
type OrderList struct {
	PruductId int `json:"pruductId"`
	ThaliId   int `json:"thaliId"`
	ThaliNum  int `json:"thaliNum"`
	DeviceNum int `json:"deviceNum"`
}
type OrderListIsbuy struct {
	PruductId int `json:"pruductId"`
	ThaliId   int `json:"thaliId"`
	ThaliNum  int `json:"thaliNum"`
	DeviceNum int `json:"deviceNum"`
	DeviceIds int `json:"deviceIds"`
}

// BuyListDevice {"code":0,"data":{"list":[{"userDeviceId":2865,"groupId":2,"deviceBz":null,"nowTime":null,"proxyId":21,"deviceToken":"3663700C333537","deviceId":5378,"onLine":-1,"overTime":1776237395,"ip":"49.68.130.254","intranetIp":"192.168.2.36","intranetPort":9011,"port":9011,"xh":"PDKM00","sysPer":2,"pingpai":null,"buyGroupId":null,"bz":null,"productId":14,"sharedBy":"","shareToUsers":null,"sysVersion":"12","deviceIp":"192.168.20.248","thaliId":14,"boxNumber":null,"portNumber":null,"location":null,"lang":null,"timezone":null,"country":null,"signalMode":null}],"total":1},"msg":"成功"}
type BuyListDevice struct {
	Code int `json:"code"`
	Data struct {
		List []struct {
			UserDeviceId int    `json:"userDeviceId"`
			GroupId      int    `json:"groupId"`
			DeviceBz     string `json:"deviceBz"`
			NowTime      int64  `json:"nowTime"`
			ProxyId      int    `json:"proxyId"`
			DeviceToken  string `json:"deviceToken"`
			DeviceId     int    `json:"deviceId"`
			OnLine       int    `json:"onLine"`
			OverTime     int64  `json:"overTime"`
			Ip           string `json:"ip"`
			IntranetIp   string `json:"intranetIp"`
			IntranetPort int    `json:"intranetPort"`
			Port         int    `json:"port"`
			Xh           string `json:"xh"`
			SysPer       int    `json:"sysPer"`
			Pingpai      string `json:"pingpai"`
			BuyGroupId   int    `json:"buyGroupId"`
			Bz           string `json:"bz"`
			ProductId    int    `json:"productId"`
			SharedBy     string `json:"sharedBy"`
			ShareToUsers []int  `json:"shareToUsers"`
			SysVersion   string `json:"sysVersion"`
			DeviceIp     string `json:"deviceIp"`
			ThaliId      int    `json:"thaliId"`
			BoxNumber    string `json:"boxNumber"`
			PortNumber   string `json:"portNumber"`
			Location     string `json:"location"`
			Lang         string `json:"lang"`
			Timezone     string `json:"timezone"`
			Country      string `json:"country"`
			SignalMode   string `json:"signalMode"`
		} `json:"list"`
		Total int `json:"total"`
	} `json:"data"`
	Msg string `json:"msg"`
}

// DownFile {url:"xxx",md5:"",install:false,name:"filename",receive:true}
type DownFile struct {
	Url     string `json:"url" msgpack:"url"`
	Md5     string `json:"md5" msgpack:"md5"`
	Install bool   `json:"install" msgpack:"install"`
	Name    string `json:"name" msgpack:"name"`
	Receive bool   `json:"receive" msgpack:"receive"`
}

// DownMsgInfo {"beginTime":1745465877,"id":1,"install":false,"lastTime":1745465880,"md5":null,"path":"/sdcard/download_1/Wandoujied.apk","process":1,"status":3,"url":"https://cos.htsystem.cn/acc-1312687568/cb16919b0d71cb2d8352fbb9f3e6cc911661ff6165b706b8fe0b8848f2d5b326.apk"}
type DownMsgInfo struct {
	BeginTime int64   `json:"beginTime" msgpack:"beginTime"`
	Id        int     `json:"id" msgpack:"id"`
	Install   bool    `json:"install" msgpack:"install"`
	LastTime  int64   `json:"lastTime" msgpack:"lastTime"`
	Md5       string  `json:"md5" msgpack:"md5"`
	Path      string  `json:"path" msgpack:"path"`
	Process   float64 `json:"process" msgpack:"process"`
	Status    int     `json:"status" msgpack:"status"`
	Url       string  `json:"url" msgpack:"url"`
	Name      string
	Ord       string
}

// AppInfo {"appname":"AppService","packageName":"com.ss.appservice","versionCode":1,"versionName":"1.0","firstInstallTime":1745400159447,"lastUpdateTime":1745400159796}
type AppInfo struct {
	Appname          string `json:"appname" msgpack:"appname"`
	PackageName      string `json:"packageName" msgpack:"packageName"`
	VersionCode      int    `json:"versionCode" msgpack:"versionCode"`
	VersionName      string `json:"versionName" msgpack:"versionName"`
	FirstInstallTime int64  `json:"firstInstallTime" msgpack:"firstInstallTime"`
	LastUpdateTime   int64  `json:"lastUpdateTime" msgpack:"lastUpdateTime"`
}
type Datauserid struct {
	Id       int    `json:"id"`
	Nickname string `json:"nickname"`
	Status   int    `json:"status"`
	DeptId   int    `json:"deptId"`
	PostIds  []int  `json:"postIds"`
	Mobile   string `json:"mobile"`
}
type ShellOrder struct {
	Name  string `json:"name" msgpack:"name"`
	Order string `json:"order" msgpack:"order"`
	Rec   string `json:"rec" msgpack:"rec"`
}

type ShellOrderList struct {
	Common []ShellOrder `json:"common" msgpack:"common"`
	Quick  []ShellOrder `json:"quick" msgpack:"quick"`
}

// Http订单详情 {"code":0,"data":{"product":{"id":17,"name":"4A5G-Android14","brief":"<p><br></p>","content":"<p><br></p>","createTime":1745307720000},"detailList":[{"id":672,"userId":1169,"pruductId":17,"thaliId":18,"thaliNum":1,"deviceNum":20,"deviceIds":"7229,7230,7231,7232,7233,7234,7235,7236,7237,7238,7239,7240,7241,7242,7243,7244,7245,7246,7247,7248","orderId":10704,"createTime":1746861746000,"thaliTimeLong":72,"thaliName":"3DAY"}],"deviceIds":[7248,7247,7246,7245,7244,7243,7242,7241,7240,7239,7238,7237,7236,7235,7234,7233,7232,7231,7230,7229],"money":20.00},"msg":"成功"}
type Http订单详情 struct {
	Code int `json:"code"`
	Data struct {
		Product struct {
			Id         int    `json:"id"`
			Name       string `json:"name"`
			Brief      string `json:"brief"`
			Content    string `json:"content"`
			CreateTime int64  `json:"createTime"`
		} `json:"product"`
		DetailList []struct {
			Id            int    `json:"id"`
			UserId        int    `json:"userId"`
			PruductId     int    `json:"pruductId"`
			ThaliId       int    `json:"thaliId"`
			ThaliNum      int    `json:"thaliNum"`
			DeviceNum     int    `json:"deviceNum"`
			DeviceIds     string `json:"deviceIds"`
			OrderId       int    `json:"orderId"`
			CreateTime    int64  `json:"createTime"`
			ThaliTimeLong int    `json:"thaliTimeLong"`
			ThaliName     string `json:"thaliName"`
		} `json:"detailList"`
		DeviceIds []int   `json:"deviceIds"`
		Money     float32 `json:"money"`
	} `json:"data"`
	Msg string `json:"msg"`
}

// ContentInfo {"f":7,"data":{"proxyId":[21,28]},"req":true,"seq":0}
type ContentInfo struct {
	F    int `json:"f"`
	Data struct {
		ProxyId []uint64 `json:"proxyId"`
	} `json:"data"`
	Req bool   `json:"req"`
	Seq uint32 `json:"seq"`
}

// SystemMod {"deviceId":1036,"type":"setting","func":2,"paramsAll":{"zuobiao":{"latitude":34.259342,"longitude":117.198539}}}
type SystemMod struct {
	DeviceId  uint64 `json:"deviceId"`
	Type      string `json:"type"`
	Func      int    `json:"func"`
	ParamsAll struct {
		Zuobiao struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"zuobiao"`
	} `json:"paramsAll"`
}

// SystemModList {"f":515,"data":[{"deviceId":1036,"type":"setting","func":2,"paramsAll":{"zuobiao":{"latitude":34.259342,"longitude":117.198539}}}],"req":true,"seq":2}
type SystemModList struct {
	F    int         `json:"f"`
	Data []SystemMod `json:"data"`
	Req  bool        `json:"req"`
	Seq  uint32      `json:"seq"`
}

type SystemChangDeviceList struct {
	F    int            `json:"f"`
	Data []ChangeDevice `json:"data"`
	Req  bool           `json:"req"`
	Seq  uint32         `json:"seq"`
}
type System备份List struct {
	F    int          `json:"f"`
	Data []BeiFenInfo `json:"data"`
	Req  bool         `json:"req"`
	Seq  uint32       `json:"seq"`
}
type System还原List struct {
	F    int              `json:"f"`
	Data []HuanyuanMobile `json:"data"`
	Req  bool             `json:"req"`
	Seq  uint32           `json:"seq"`
}
type System还原AppList struct {
	F    int           `json:"f"`
	Data []HuanyuanApp `json:"data"`
	Req  bool          `json:"req"`
	Seq  uint32        `json:"seq"`
}
type System隐藏AppList struct {
	F    int       `json:"f"`
	Data []Yincang `json:"data"`
	Req  bool      `json:"req"`
	Seq  uint32    `json:"seq"`
}

// SystemModResult [{"type":"setting","func":2,"state":1,"shellResult":"操作成功","mainTaskId":"1933740163719045120","taskId":"1933740163719045121","deviceId":5378,"finish":1}]
type SystemModResult struct {
	Type        string `json:"type"`
	Func        int    `json:"func"`
	State       int    `json:"state"`
	ShellResult string `json:"shellResult"`
	MainTaskId  string `json:"mainTaskId"`
	TaskId      string `json:"taskId"`
	DeviceId    uint64 `json:"deviceId"`
	Finish      int    `json:"finish"`
}

// ChangeDevice [{"deviceId":1044,"type":"changeDevice","func":1,"paramsAll":{"sdk":{"value":31},"zhiding":{"PKG1":"com.ss.appservice","PKG2":""},"huanji":{}}}]
type ChangeDevice struct {
	DeviceId  uint64 `json:"deviceId"`
	Type      string `json:"type"`
	Func      int    `json:"func"`
	ParamsAll struct {
		Sdk struct {
			Value int `json:"value"`
		} `json:"sdk"`
		Zhiding struct {
			Pkg1 string `json:"PKG1"`
			Pkg2 string `json:"PKG2"`
		} `json:"zhiding"`
		Huanji struct {
		} `json:"huanji"`
	} `json:"paramsAll"`
}

// Zhiding {"PKG1":"com.ss.appservice","PKG2":""}
type Zhiding struct {
	Pkg1 string `json:"PKG1"`
	Pkg2 string `json:"PKG2"`
}

// BeiFenInfo {"deviceId":1044,"type":"beifenToRemote","func":10,"paramsAll":{"beifen":{"appname":"AppService","packageName":"com.ss.appservice"},"beifenRemote":{"appname":"AppService","packageName":"com.ss.appservice"}}}
type BeiFenInfo struct {
	DeviceId  uint64 `json:"deviceId"`
	Type      string `json:"type"`
	Func      int    `json:"func"`
	ParamsAll struct {
		Beifen struct {
			Appname     string `json:"appname"`
			PackageName string `json:"packageName"`
		} `json:"beifen"`
		BeifenRemote struct {
			Appname     string `json:"appname"`
			PackageName string `json:"packageName"`
		} `json:"beifenRemote"`
	} `json:"paramsAll"`
}

// AppMsg {"appname":"AppService","packageName":"com.ss.appservice"}
type AppMsg struct {
	AppName string `json:"appName"`
	Package string `json:"package"`
}

// BeiFenDeviceInfo {"id":118,"appPackage":"sg.omi","userId":1169,"backupFile":null,"url":"https://0.0.0.0/b1dd88cdd8bf09af2539d0b345e647129d8fd55c92b6d824b6ecc53efd531028","sn":"06142246097463","createTime":1749971158000,"deviceId":7178,"appName":"Omi","sha256":"b1dd88cdd8bf09af2539d0b345e647129d8fd55c92b6d824b6ecc53efd531028"}
type BeiFenDeviceInfo struct {
	Id         int    `json:"id"`
	AppPackage string `json:"appPackage"`
	UserId     int    `json:"userId"`
	BackupFile string `json:"backupFile"`
	Url        string `json:"url"`
	Sn         string `json:"sn"`
	CreateTime int64  `json:"createTime"`
	DeviceId   uint64 `json:"deviceId"`
	AppName    string `json:"appName"`
	Sha256     string `json:"sha256"`
}

// BeiFenDeviceList {"code":0,"data":{"list":[{"id":118,"appPackage":"sg.omi","userId":1169,"backupFile":null,"url":"https://0.0.0.0/b1dd88cdd8bf09af2539d0b345e647129d8fd55c92b6d824b6ecc53efd531028","sn":"06142246097463","createTime":1749971158000,"deviceId":7178,"appName":"Omi","sha256":"b1dd88cdd8bf09af2539d0b345e647129d8fd55c92b6d824b6ecc53efd531028"}],"total":1},"msg":"成功"}
type BeiFenDeviceList struct {
	Code int `json:"code"`
	Data struct {
		List  []BeiFenDeviceInfo `json:"list"`
		Total int                `json:"total"`
	} `json:"data"`
	Msg string `json:"msg"`
}

// HuanyuanMobile  {"deviceId":1036,"type":"setting","func":5,"paramsAll":{"huanyuanMobile":{"sn":"06071131323604"}}}
type HuanyuanMobile struct {
	DeviceId  uint64 `json:"deviceId"`
	Type      string `json:"type"`
	Func      int    `json:"func"`
	ParamsAll struct {
		HuanYuanMobile struct {
			Sn string `json:"sn"`
		} `json:"huanyuanMobile"`
	} `json:"paramsAll"`
}

// HuanyuanApp {"deviceId":5378,"type":"huanyuanFromRemote","func":9,"paramsAll":{"download":{"fileName":"zuoyebang_androidPhone_wbqd_pc_downloadpage_zyb.apk","url":"https://cos.htsystem.cn/acc-1312687568/2bcf83e3d59bdaec21bee327c426858c0053996685bd920dd84afe290fb5f15f.apk"},"huanyuanRemote":{"url":"https://0.0.0.0/fae8988450c6ad487df4a8c0cefe5cc57897ceaad1979489758fecdfcceb5051","sn":"06101352132347","sha256":"fae8988450c6ad487df4a8c0cefe5cc57897ceaad1979489758fecdfcceb5051","packageName":"com.baidu.homework"},"huanyuan":{"packageName":"com.baidu.homework"}}}
type HuanyuanApp struct {
	DeviceId  uint64 `json:"deviceId"`
	Type      string `json:"type"`
	Func      int    `json:"func"`
	ParamsAll struct {
		Download struct {
			FileName string `json:"fileName"`
			Url      string `json:"url"`
		} `json:"download"`
		HuanYuanRemote struct {
			Url         string `json:"url"`
			Sn          string `json:"sn"`
			Sha256      string `json:"sha256"`
			PackageName string `json:"packageName"`
		} `json:"huanyuanRemote"`
		HuanYuan struct {
			PackageName string `json:"packageName"`
		} `json:"huanyuan"`
	} `json:"paramsAll"`
}

// Yincang {"deviceId":1044,"type":"setting","func":4,"paramsAll":{"yincang":{"app":"com.ss.appservice","flag":0}}}
type Yincang struct {
	DeviceId  uint64 `json:"deviceId"`
	Type      string `json:"type"`
	Func      int    `json:"func"`
	ParamsAll struct {
		Yincang struct {
			App  string `json:"app"`
			Flag int    `json:"flag"`
		} `json:"yincang"`
	} `json:"paramsAll"`
}

type Route struct {
	DeviceId    uint64 `json:"deviceId" msgpack:"deviceId"`
	DeviceIp    string `json:"deviceIp" msgpack:"deviceIp"`
	Sk5Server   string `json:"sk5Server" msgpack:"sk5Server"`
	Sk5Port     int    `json:"sk5Port" msgpack:"sk5Port"`
	Sk5Username string `json:"sk5Username" msgpack:"sk5Username"`
	Sk5Password string `json:"sk5Password" msgpack:"sk5Password"`
	Type        string `json:"type" msgpack:"type"`
}

type RouterConfig struct {
	F    int     `json:"f" msgpack:"f"`
	Data []Route `json:"data" msgpack:"data"`
	Req  bool    `json:"req" msgpack:"req"`
	Seq  uint32  `json:"seq" msgpack:"seq"`
}

func rotate90(m image.Image) image.Image {
	rotate90 := image.NewRGBA(image.Rect(0, 0, m.Bounds().Dy(), m.Bounds().Dx()))
	// 矩阵旋转
	for x := m.Bounds().Min.Y; x < m.Bounds().Max.Y; x++ {
		for y := m.Bounds().Max.X - 1; y >= m.Bounds().Min.X; y-- {
			//  设置像素点
			rotate90.Set(m.Bounds().Max.Y-x, y, m.At(y, x))
		}
	}
	return rotate90
}

func Rotate90Bytes(Picture []byte) []byte {
	// 创建新的读取器

	reader := bytes.NewReader(Picture)

	// 解码图片
	srcImg, _, err := image.Decode(reader)
	if err != nil {

	}

	// 调用rotate90函数旋转图片
	rotatedImg := rotate90(srcImg)
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, rotatedImg, &jpeg.Options{Quality: 95})

	// 编码图像回字节数组
	return buf.Bytes()

}
func rotate180(m image.Image) image.Image {
	rotate180 := image.NewRGBA(image.Rect(0, 0, m.Bounds().Dx(), m.Bounds().Dy()))
	// 180度旋转：将(x,y)映射到(width-1-x, height-1-y)
	for x := m.Bounds().Min.X; x < m.Bounds().Max.X; x++ {
		for y := m.Bounds().Min.Y; y < m.Bounds().Max.Y; y++ {
			// 设置像素点
			rotate180.Set(m.Bounds().Max.X-1-x, m.Bounds().Max.Y-1-y, m.At(x, y))
		}
	}
	return rotate180
}

// Rotate180Bytes 将字节数组表示的图片旋转180度
func Rotate180Bytes(Picture []byte) []byte {
	// 创建新的读取器
	reader := bytes.NewReader(Picture)

	// 解码图片
	srcImg, _, err := image.Decode(reader)
	if err != nil {
		return Picture // 如果解码失败，返回原图
	}

	// 调用rotate180函数旋转图片
	rotatedImg := rotate180(srcImg)
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, rotatedImg, &jpeg.Options{Quality: 95})
	if err != nil {
		return Picture // 如果编码失败，返回原图
	}

	// 编码图像回字节数组
	return buf.Bytes()
}

// rotate270 将图像旋转270度（逆时针90度）
func rotate270(m image.Image) image.Image {
	rotate270 := image.NewRGBA(image.Rect(0, 0, m.Bounds().Dy(), m.Bounds().Dx()))
	// 270度旋转：将(x,y)映射到(y, width-1-x)
	for x := m.Bounds().Min.X; x < m.Bounds().Max.X; x++ {
		for y := m.Bounds().Min.Y; y < m.Bounds().Max.Y; y++ {
			// 设置像素点
			rotate270.Set(y, m.Bounds().Max.X-1-x, m.At(x, y))
		}
	}
	return rotate270
}

// Rotate270Bytes 将字节数组表示的图片旋转270度
func Rotate270Bytes(Picture []byte) []byte {
	// 创建新的读取器
	reader := bytes.NewReader(Picture)

	// 解码图片
	srcImg, _, err := image.Decode(reader)
	if err != nil {
		return Picture // 如果解码失败，返回原图
	}

	// 调用rotate270函数旋转图片
	rotatedImg := rotate270(srcImg)
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, rotatedImg, &jpeg.Options{Quality: 95})
	if err != nil {
		return Picture // 如果编码失败，返回原图
	}

	// 编码图像回字节数组
	return buf.Bytes()
}
