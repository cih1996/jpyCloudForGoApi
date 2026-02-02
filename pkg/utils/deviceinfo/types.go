package deviceinfo

type DevInfo struct {
	Arch     string `json:"Arch" msgpack:"Arch"`         //架构，x86/ARM
	Model    string `json:"Model" msgpack:"Model"`       //机器的型号
	Cpu      string `json:"Cpu" msgpack:"Cpu"`           //cpu型号
	Memory   uint64 `json:"Memory" msgpack:"Memory"`     //内存大小
	BootDisk Disk   `json:"BootDisk" msgpack:"BootDisk"` //硬盘信息
	PermAddr uint64 `json:"PermAddr" msgpack:"PermAddr"` //物理网卡的真实mac地址，不能修改的那种
}

type Disk struct {
	Model        string `msgpack:"Model"`        //型号
	SerialNumber string `msgpack:"SerialNumber"` //序列号
	Size         uint64 `msgpack:"Size"`         //大小
}

type InfoStat struct {
	CPU        int32    `json:"cpu"`
	VendorID   string   `json:"vendorId"`
	Family     string   `json:"family"`
	Model      string   `json:"model"`
	Stepping   int32    `json:"stepping"`
	PhysicalID string   `json:"physicalId"`
	CoreID     string   `json:"coreId"`
	Cores      int32    `json:"cores"`
	ModelName  string   `json:"modelName"`
	Mhz        float64  `json:"mhz"`
	CacheSize  int32    `json:"cacheSize"`
	Flags      []string `json:"flags"`
	Microcode  string   `json:"microcode"`
}
type PartitionStat struct {
	Device     string `json:"device"`
	Mountpoint string `json:"mountpoint"`
	Fstype     string `json:"fstype"`
	Opts       string `json:"opts"`
}

type HostInfo struct {
	Hostname        string `json:"Hostname" msgpack:"Hostname"`
	OS              string `json:"Os" msgpack:"Os"` // ex: freebsd, linux
	Arch            string `json:"Arch" msgpack:"Arch"`
	Platform        string `json:"Platform" msgpack:"Platform"`               // ex: ubuntu, linuxmint
	PlatformFamily  string `json:"PlatformFamily" msgpack:"PlatformFamily"`   // ex: debian, rhel
	PlatformVersion string `json:"PlatformVersion" msgpack:"PlatformVersion"` // version of the complete OS
	KernelVersion   string `json:"KernelVersion" msgpack:"KernelVersion"`     // version of the OS kernel (if available)
	HostID          string `json:"Hostid" msgpack:"Hostid"`                   // ex: uuid
}
