//go:build linux
// +build linux

package deviceinfo

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func NewDiskInfo(name string) (d Disk, err error) {
	var stat unix.Stat_t
	err = unix.Stat(name, &stat)
	if err != nil {
		return
	}
	major := unix.Major(uint64(stat.Rdev))
	//minor := unix.Minor(uint64(stat.Rdev))
	devicePath := GetEnv("HOST_SYS", "/sys", fmt.Sprintf("dev/block/%d:0", major))
	m, _ := os.ReadFile(filepath.Join(devicePath, "device", "model"))
	if len(m) > 0 {
		d.Model = strings.TrimSpace(string(m))
	} else if n, _ := os.ReadFile(filepath.Join(devicePath, "device", "name")); len(n) > 0 {
		d.Model = strings.TrimSpace(string(n))
	}
	s, _ := os.ReadFile(filepath.Join(devicePath, "device", "serial"))
	if len(s) > 0 {
		d.SerialNumber = strings.TrimSpace(string(s))
	}
	sz, _ := os.ReadFile(filepath.Join(devicePath, "size"))
	if len(sz) > 0 {
		d.Size, _ = strconv.ParseUint(strings.TrimSpace(string(sz)), 10, 64)
		if d.Size > 0 {
			d.Size = d.Size / 2
		}
	}
	return
}

func Partitions(all bool) ([]PartitionStat, error) {
	useMounts := false

	filename := GetEnv("HOST_PROC", "/proc", "1/mountinfo")
	lines, err := ReadLines(filename)
	if err != nil {
		if err != err.(*os.PathError) {
			return nil, err
		}
		// if kernel does not support 1/mountinfo, fallback to 1/mounts (<2.6.26)
		useMounts = true
		filename = GetEnv("HOST_PROC", "/proc", "1/mounts")
		lines, err = ReadLines(filename)
		if err != nil {
			return nil, err
		}
	}

	fs, err := getFileSystems()
	if err != nil && !all {
		return nil, err
	}

	ret := make([]PartitionStat, 0, len(lines))

	for _, line := range lines {
		var d PartitionStat
		if useMounts {
			fields := strings.Fields(line)

			d = PartitionStat{
				Device:     fields[0],
				Mountpoint: unescapeFstab(fields[1]),
				Fstype:     fields[2],
				Opts:       fields[3],
			}

			if !all {
				if d.Device == "none" || !StringsHas(fs, d.Fstype) {
					continue
				}
			}
		} else {
			// a line of 1/mountinfo has the following structure:
			// 36  35  98:0 /mnt1 /mnt2 rw,noatime master:1 - ext3 /dev/root rw,errors=continue
			// (1) (2) (3)   (4)   (5)      (6)      (7)   (8) (9)   (10)         (11)

			// split the mountinfo line by the separator hyphen
			parts := strings.Split(line, " - ")
			if len(parts) != 2 {
				return nil, fmt.Errorf("found invalid mountinfo line in file %s: %s ", filename, line)
			}

			fields := strings.Fields(parts[0])
			blockDeviceID := fields[2]
			mountPoint := fields[4]
			mountOpts := fields[5]

			if rootDir := fields[3]; rootDir != "" && rootDir != "/" {
				if len(mountOpts) == 0 {
					mountOpts = "bind"
				} else {
					mountOpts = "bind," + mountOpts
				}
			}

			fields = strings.Fields(parts[1])
			fstype := fields[0]
			device := fields[1]

			d = PartitionStat{
				Device:     device,
				Mountpoint: unescapeFstab(mountPoint),
				Fstype:     fstype,
				Opts:       mountOpts,
			}

			if !all {
				if d.Device == "none" || !StringsHas(fs, d.Fstype) {
					continue
				}
			}

			if strings.HasPrefix(d.Device, "/dev/mapper/") {
				devpath, err := filepath.EvalSymlinks(GetEnv("HOST_DEV", "/dev", strings.Replace(d.Device, "/dev", "", -1)))
				if err == nil {
					d.Device = devpath
				}
			}

			// /dev/root is not the real device name
			// so we get the real device name from its major/minor number
			if d.Device == "/dev/root" {
				devpath, err := os.Readlink(GetEnv("HOST_PROC", "/proc", "/dev/block/"+blockDeviceID))
				if err != nil {
					return nil, err
				}
				d.Device = strings.Replace(d.Device, "root", filepath.Base(devpath), 1)
			}
		}
		ret = append(ret, d)
	}

	return ret, nil
}

// getFileSystems returns supported filesystems from /proc/filesystems
func getFileSystems() ([]string, error) {
	filename := GetEnv("HOST_PROC", "/proc", "filesystems")
	lines, err := ReadLines(filename)
	if err != nil {
		return nil, err
	}
	var ret []string
	for _, line := range lines {
		if !strings.HasPrefix(line, "nodev") {
			ret = append(ret, strings.TrimSpace(line))
			continue
		}
		t := strings.Split(line, "\t")
		if len(t) != 2 || t[1] != "zfs" {
			continue
		}
		ret = append(ret, strings.TrimSpace(t[1]))
	}

	return ret, nil
}

// GetEnv retrieves the environment variable key. If it does not exist it returns the default.
func GetEnv(key string, dfault string, combineWith ...string) string {
	value := os.Getenv(key)
	if value == "" {
		value = dfault
	}

	switch len(combineWith) {
	case 0:
		return value
	case 1:
		return filepath.Join(value, combineWith[0])
	default:
		all := make([]string, len(combineWith)+1)
		all[0] = value
		copy(all[1:], combineWith)
		return filepath.Join(all...)
	}
}

// StringsHas checks the target string slice contains src or not.
func StringsHas(target []string, src string) bool {
	for _, t := range target {
		if strings.TrimSpace(t) == src {
			return true
		}
	}
	return false
}

// Unescape escaped octal chars (like space 040, ampersand 046 and backslash 134) to their real value in fstab fields issue#555
func unescapeFstab(path string) string {
	escaped, err := strconv.Unquote(`"` + path + `"`)
	if err != nil {
		return path
	}
	return escaped
}
