//go:build windows

package deviceinfo

import (
	"fmt"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"github.com/yusufpapurcu/wmi"
	"runtime"
)

func NewDevInfo() (info DevInfo) {
	info.Arch = runtime.GOARCH
	stats, err := cpu.Info()
	if err == nil && len(stats) > 0 {
		info.Cpu = stats[0].ModelName
	}
	memory, err := mem.VirtualMemory()
	if err == nil {
		info.Memory = uint64(memory.Total)
	}
	disks, err := NewDiskInfo("")
	if err == nil && len(disks) > 0 {
		info.BootDisk = disks[0]
	}
	addr, err := GetHardwareAddr("")
	if err == nil {
		info.PermAddr, _ = MAC2Uint64(addr)
	}
	return
}

func NewDiskInfo(name string) (d []Disk, err error) {
	err = wmi.Query("SELECT * FROM Win32_DiskDrive  where InterfaceType !='USB'", &d)
	fmt.Println(d)
	return
}
