package public

type HttpCache struct {
	Url    string `json:"url" msgpack:"url"`
	Enable bool   `json:"enable" msgpack:"enable"`
}

// Version 升级包必备文件: version.dat
type Version struct {
	Name    string `json:"name" msgpack:"name" validate:"required"`       //插件昵称
	Id      uint8  `json:"id" msgpack:"id" validate:"required"`           //插件id,0=中间件,1=jar,大于1 是插件
	Version string `json:"version" msgpack:"version" validate:"required"` //版本号
	Hash    string `json:"hash" msgpack:"hash" validate:"required"`       //升级包文件的sha256
	Url     string `json:"url" msgpack:"url" validate:"required"`         //升级包文件下载地址
}

func (v *Version) Equal(newVersion *Version) bool {
	if v.Id != newVersion.Id || v.Version != newVersion.Version {
		return false
	}
	return true
}

// PluginConfig 升级包必备文件:config.dat,主执行文件
// 可使用环境变量: BASE_DIR =arm_linux所在的目录.WORK_DIR =这个插件解压后所在的目录
type PluginConfig struct {
	File string   `json:"file" msgpack:"file"` //执行文件名,必然存在于插件目录或其子目录
	Hash string   `json:"hash" msgpack:"hash"` //sha256,执行文件的校验码,每次启动程序前校验.
	Root bool     `json:"root" msgpack:"root"` //是否root方式执行.当前root有效.否则无视这个参数.false=使用shell用户启动
	Run  string   `json:"run" msgpack:"run"`   //执行哪个文件,此字段支持环境变量
	Env  []string `json:"env" msgpack:"env"`   //环境变量,此字段支持环境变量
	Args []string `json:"args" msgpack:"args"` //执行参数,此字段支持环境变量

	/*Type 工作方式:
	0=linux进程
	1=apk每秒检查进程是否存在
	2=apk无值守1一分钟检查一次是否安装(首次安装会启动一次)
	*/
	Type        int      `json:"type" msgpack:"type"`
	Permissions []string `json:"permissions,omitempty" msgpack:"permissions,omitempty"` //当type=1时有效,为apk设置权限
}

// VersionSequence 集控平台向中间件下发的版本文件序号和手机vs版本序号
type VersionSequence struct {
	File           int `json:"file" msgpack:"file"`
	Device2Version int `json:"device2version" msgpack:"device2version"`
}

// MgtVersion 集控平台下发的版本列表
type MgtVersion struct {
	Sequence int       `json:"sequence" msgpack:"sequence"` //版本列表的发行序号
	Versions []Version `json:"versions" msgpack:"versions"` //版本列表
}

// MgtDeviceVersion 手机和版本的对于关系.一个手机的描述应该是: {"main":"version","plugin":[DeviceVersion]}
type MgtDeviceVersion struct {
	Seat    uint64           `json:"seat" msgpack:"seat"`       //盘位
	Main    string           `json:"main" msgpack:"main"`       //主版本号
	Plugins map[uint8]string `json:"plugins" msgpack:"plugins"` //插件列表
}

// MgtDeviceVersions 集控平台下发的device2version列表
type MgtDeviceVersions struct {
	Sequence int                `json:"sequence" msgpack:"sequence"`
	List     []MgtDeviceVersion `json:"list" msgpack:"list"`
}

// DeviceVersion 手机的版本描述结构
type DeviceVersion struct {
	Main   Version   `json:"main" msgpack:"main"`     //主进程的版本,也就是jar+arm_linux的版本号
	Plugin []Version `json:"plugin" msgpack:"plugin"` //插件列表
}

type MgtDeviceVersionTmp struct {
	Seat    uint64            `json:"seat" msgpack:"seat"`       //盘位
	Main    string            `json:"main" msgpack:"main"`       //主版本号
	Plugins map[string]string `json:"plugins" msgpack:"plugins"` //插件列表
}
type MgtDeviceVersionsTmp struct {
	Sequence int                   `json:"sequence" msgpack:"sequence"`
	List     []MgtDeviceVersionTmp `json:"list" msgpack:"list"`
}

func (d *DeviceVersion) Equal(mdv *MgtDeviceVersion) bool {
	if d.Main.Version != mdv.Main {
		return false
	}
	if len(d.Plugin) != len(mdv.Plugins) {
		return false
	}
	for i := 0; i < len(d.Plugin); i++ {
		s, ok := mdv.Plugins[d.Plugin[i].Id]
		if !ok || d.Plugin[i].Version != s {
			return false
		}
	}
	return true
}

// DeviceVersionSave 本地缓存的手机版本设置列表文件结构
type DeviceVersionSave struct {
	Sequence int                      `json:"sequence" msgpack:"sequence"`
	Devices  map[uint8]*DeviceVersion `json:"devices" msgpack:"devices"`
}
type TokenInfo struct {
	UserId   uint64 `json:"UserId" msgpack:"UserId"`     //用户id
	DeviceId uint64 `json:"DeviceId" msgpack:"DeviceId"` //uint64(证书发行序号)<<8 + uint8(Seat)
	HostUrl  string `json:"HostUrl" msgpack:"HostUrl"`
	Token    string `json:"Token" msgpack:"Token"`
	GuestUrl string `json:"GuestUrl" msgpack:"GuestUrl"`
	Guest    string `json:"Guest" msgpack:"Guest"`
	Host     string `json:"Host" msgpack:"Host"`
}

type Online struct {
	Seat   uint8  `json:"seat" msgpack:"seat"`
	Online int32  `json:"online" msgpack:"online"`
	Ip     string `json:"ip" msgpack:"ip"`
}

type LoginMsg struct {
	Data []byte `json:"data" msgpack:"data"`
	Sign []byte `json:"sign" msgpack:"sign"`
	Ip   string `json:"ip" msgpack:"ip"`
}

// PreSetting dhcp服务端下发的预分配设置
type PreSetting struct {
	AndroidSN string `json:"SN" msgpack:"SN"`     //手机唯一标识
	Seat      uint8  `json:"seat" msgpack:"seat"` //盘位
	Addr      uint32 `json:"addr" msgpack:"addr"` //中间件地址
}

func (s *PreSetting) Equals(newSetting *PreSetting) bool {
	if s.AndroidSN != newSetting.AndroidSN || s.Seat != newSetting.Seat || s.Addr != newSetting.Addr {
		return false
	}
	return true
}

type MobileConfig struct {
	UUID       string `json:"uuid" msgpack:"uuid" validate:"required"`             //apple=app的容器uuid,android=sn
	Id         uint64 `json:"id" msgpack:"id" validate:"required"`                 //中间件id,签发者id
	Seat       uint8  `json:"seat" msgpack:"seat" validate:"required"`             //由中间件分配的设备位置seat,由id改为seat,统一概念,更好理解.修改类型为uint8
	WS         string `json:"ws" msgpack:"ws" validate:"required"`                 //app要监听的websocket地址 比如:0.0.0.0:9009
	PluginPort int    `json:"pluginPort" msgpack:"pluginPort" validate:"required"` //插件工作端口
	Agent      string `json:"agent" msgpack:"agent" validate:"required"`           //中间件地址 比如: 192.168.2.145:8007
	Time       int64  `json:"time" msgpack:"time" validate:"required"`             //fixed,新增字段,配置文件签发时间
}

// VideoOpt 开启h264串流需要的参数结构
type VideoOpt struct {
	Width   uint32 `json:"width" msgpack:"width" validate:"required"`     //期望图像宽度,高度使用等比例缩放
	Quality uint8  `json:"quality" msgpack:"quality" validate:"required"` //图像质量百分百, 最大100,推荐25
	Fps     uint8  `json:"fps" msgpack:"fps" validate:"required"`         //fps,1~120,推荐24
	Bit     uint32 `json:"bit" msgpack:"bit" validate:"required"`         //期望码率,推荐 宽*高/100*quality*4
}

func (s *VideoOpt) Verify() bool {
	if s.Width < 100 || s.Quality < 1 || s.Fps <= 1 {
		return false
	}
	return true
}

const (
	ReadyHidStatus  = 1 << 0 //0位,是否存在硬件小板的hid设备
	ReadyPreSetting = 1 << 1 //1位,是否存在预配置
	ReadySetting    = 1 << 2 //2位,是否存在完整的设置信息
	ReadyConnected  = 1 << 3 //3位,是否已正确连接到发号器
)

// DeviceInfo udp扫描的回应数据
type DeviceInfo struct {
	AndroidSN  string `json:"SN" msgpack:"SN"`             //手机唯一标识
	AndroidVer string `json:"sysVer" msgpack:"sysVer"`     //安卓版本号
	Cpu        int    `json:"cpu" msgpack:"cpu"`           //cpu核心数量
	Memory     uint64 `json:"memory" msgpack:"memory"`     //内存大小
	DiskSize   uint64 `json:"diskSize" msgpack:"diskSize"` //磁盘大小
	Uptime     uint64 `json:"uptime" msgpack:"uptime"`     //系统已运行的毫秒数

	Version   string `json:"version" msgpack:"version"` //armLinux版本号
	Seat      uint8  `json:"seat" msgpack:"seat"`       //盘位
	AgentId   uint64 `json:"aId" msgpack:"aId"`         //中间件id
	AgentAddr string `json:"addr" msgpack:"addr"`       //中间件地址
	Ready     int8   `json:"ready" msgpack:"ready"`     //是否正确配置,0=未配置,1=有预配置,2=hid标识存在,3=正式配置存在
}

func (d *DeviceInfo) SetReady(hid, preSetting, setting, connected bool) *DeviceInfo {
	d.Ready = 0
	if hid {
		d.Ready |= ReadyHidStatus
	}
	if preSetting {
		d.Ready |= ReadyPreSetting
	}
	if setting {
		d.Ready |= ReadySetting
	}
	if connected {
		d.Ready |= ReadyConnected
	}
	return d
}

func (d *DeviceInfo) GetHidStatus() bool {
	return d.Ready&ReadyHidStatus != 0
}

type TermAction struct {
	Action int    `json:"action" msgpack:"action"` //0=关闭,1=开启,2=resize
	Rows   uint16 `json:"rows" msgpack:"rows"`     //设定行数(action>0有效)
	Cols   uint16 `json:"cols" msgpack:"cols"`     //设定列数(action>0有效)
}
type ForwardRequest struct {
	Token   string `json:"token,omitempty" msgpack:"token,omitempty"` //最终分配的token
	Agent   string `json:"agent,omitempty" msgpack:"agent,omitempty"` //告诉手机,中继服务的地址: 中间件ip:服务端口
	Seat    uint64 `json:"seat,omitempty" msgpack:"seat,omitempty"`   //要对哪个设备端口映射,该字段是为了兼容管理连接来申请token
	Mode    int    `json:"mode,omitempty" msgpack:"mode,omitempty"`   //工作模式,0=连接复用的端口映射,1=无连接复用的端口映射,2=socks5代理
	Multi   bool   `json:"multi" msgpack:"multi"`                     // Deprecated:被mode代替. 是否开启多路复用
	Proto   string `json:"proto" msgpack:"proto"`                     //协议:tcp/udp,mode=2时忽略
	DstPort int    `json:"dstPort" msgpack:"dstPort"`                 //映射给内网的哪个端口,mode=2时忽略
}
