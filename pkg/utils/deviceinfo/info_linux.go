//go:build linux
// +build linux

package deviceinfo

import (
	"os"
	"runtime"
	"strings"
	"syscall"
)

// cpu型号,型号,内存大小,磁盘大小,磁盘序列号,mac地址,系统版本

func NewDevInfo() (info DevInfo) {
	info.Arch = runtime.GOARCH
	buf, err := os.ReadFile("devicetree/base/model")
	if err == nil {
		info.Model = strings.TrimSpace(string(buf))
	}
	cpu, err := CpuInfo()
	if err == nil && len(cpu) > 0 {
		info.Cpu = cpu[0].ModelName
	}
	var inf syscall.Sysinfo_t
	err = syscall.Sysinfo(&inf)
	if err == nil {
		info.Memory = uint64(inf.Totalram)
	}
	addr, err := GetHardwareAddr("")
	if err == nil {
		info.PermAddr, err = MAC2Uint64(addr)
	}
	ps, err := Partitions(false)
	if err == nil {
		for _, p := range ps {
			if p.Mountpoint == "/" {
				info.BootDisk, err = NewDiskInfo(p.Device)
				break
			}
		}
	}
	return
}
