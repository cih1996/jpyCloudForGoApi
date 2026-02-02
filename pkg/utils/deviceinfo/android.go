//go:build android || linux

package deviceinfo

import (
	"bufio"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func AndroidVersion() string {
	output, err := exec.Command("getprop", "ro.build.version.release").CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
func AndroidSN() string {
	var output []byte
	var err error
	if _, err = os.Stat("/metadata/property/serialno"); err == nil {
		output, err = os.ReadFile("/metadata/property/serialno")
	} else {
		output, err = exec.Command("getprop", "ro.serialno").CombinedOutput()
	}
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
func AndroidDiskSize() (size uint64) {
	total, _, _, _ := GetDiskUsage("/")
	size = total
	total, _, _, _ = GetDiskUsage("/data")
	size = total + size
	return
}

// GetDiskUsage 获取指定路径的磁盘空间信息
func GetDiskUsage(path string) (total, free, available uint64, err error) {
	var stat unix.Statfs_t
	err = unix.Statfs(path, &stat)
	if err != nil {
		return
	}

	// 计算总大小、可用空间、空闲空间（单位：字节）
	blockSize := uint64(stat.Bsize)
	total = stat.Blocks * blockSize
	free = stat.Bfree * blockSize
	available = stat.Bavail * blockSize
	return
}

// Uptime 返回当前系统已运行了多少秒
func Uptime() uint64 {
	sysinfo := &unix.Sysinfo_t{}
	if err := unix.Sysinfo(sysinfo); err != nil {
		return 0
	}
	return uint64(sysinfo.Uptime)
}
func CPUCoreCount() int {
	data, err := os.ReadFile("/sys/devices/system/cpu/present")
	if err != nil {
		return 0
	}

	// 解析格式，例如 "0-7" 表示 8 个核心
	parts := strings.Split(strings.TrimSpace(string(data)), "-")
	if len(parts) != 2 {
		return 0
	}

	start, err1 := strconv.Atoi(parts[0])
	end, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0
	}
	return end - start + 1
}
func MemorySize() (uint64, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				memKB, err := strconv.ParseUint(parts[1], 10, 64)
				if err != nil {
					return 0, err
				}
				return memKB * 1024, nil // 转换为字节
			}
		}
	}
	return 0, fmt.Errorf("MemTotal not found")
}
