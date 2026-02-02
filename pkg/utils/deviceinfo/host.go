package deviceinfo

import (
	"context"
	"github.com/shirou/gopsutil/host"
	"os"
	"runtime"
)

func NewHostInfo() (ret HostInfo) {
	ret.Hostname, _ = os.Hostname()
	ret.OS = runtime.GOOS
	ret.Arch = runtime.GOARCH
	ret.Platform, ret.PlatformFamily, ret.PlatformVersion, _ = host.PlatformInformationWithContext(context.Background())
	ret.KernelVersion, _ = host.KernelVersion()
	ret.HostID, _ = host.HostIDWithContext(context.Background())
	return
}
