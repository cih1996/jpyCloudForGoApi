//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || zos

package disk

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Partition struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Size       uint64 `json:"size"`
	Type       string `json:"type"`
	FsSize     uint64 `json:"fssize"`
	FsType     string `json:"fstype"`
	FsUsed     uint64 `json:"fsused"`
	FsVer      string `json:"fsver"`
	Uuid       string `json:"uuid"`
	MountPoint string `json:"mountpoint"`
}
type Disk struct {
	Name       string      `json:"name"`
	Path       string      `json:"path"`
	Size       uint64      `json:"size"`
	Tran       string      `json:"tran"`
	Type       string      `json:"type"`
	Serial     string      `json:"serial"`
	Model      string      `json:"model"`
	Partitions []Partition `json:"children"`
}

func (d *Disk) PartitionByUUID(uuid string) *Partition {
	for i := 0; i < len(d.Partitions); i++ {
		if uuid == d.Partitions[i].Uuid {
			return &d.Partitions[i]
		}
	}
	return nil
}
func (d *Disk) PartitionByPath(path string) *Partition {
	for i := 0; i < len(d.Partitions); i++ {
		if path == d.Partitions[i].Path {
			return &d.Partitions[i]
		}
	}
	return nil
}

type devices struct {
	BlockDevices []Disk `json:"blockdevices"`
}

// GetDisks 使用lsblk命令获取linux下的磁盘和分区列表
func GetDisks() ([]Disk, error) {
	c := exec.Command("lsblk", "-J", "-b", "-e", "7,11,252", "-o", "NAME,PATH,SIZE,TRAN,TYPE,FSSIZE,FSTYPE,FSUSED,FSVER,UUID,SERIAL,MODEL,MOUNTPOINT")
	output, err := c.CombinedOutput()
	if err != nil {
		return nil, err
	}
	var v devices
	err = json.Unmarshal(output, &v)
	return v.BlockDevices, err
}
func GetDisk(dev string) (*Disk, error) {
	c := exec.Command("lsblk", "-J", "-b", "-e", "7,11,252", "-o", "NAME,PATH,SIZE,TRAN,TYPE,FSSIZE,FSTYPE,FSUSED,FSVER,UUID,SERIAL,MODEL,MOUNTPOINT", dev)
	output, err := c.CombinedOutput()
	if err != nil {
		return nil, err
	}
	var v devices
	err = json.Unmarshal(output, &v)
	if err != nil {
		return nil, err
	}
	if len(v.BlockDevices) == 0 {
		return nil, errors.New("no disk found")
	}
	return &v.BlockDevices[0], nil
}

// GetMountPoint 使用lsblk获取设备的挂载点路径.返回:"",err=nil,表示未挂载
// dev可以是设备路径例如:/dev/sda,也可以是分区的uuid
func GetMountPoint(dev string) (string, error) {
	isUUID := !strings.HasPrefix(dev, "/")
	disks, err := GetDisks()
	if err != nil {
		return "", err
	}
	for _, disk := range disks {
		for _, part := range disk.Partitions {
			if isUUID {
				if part.Uuid == dev {
					return part.MountPoint, nil
				}
			} else {
				if part.Path == dev {
					return part.MountPoint, nil
				}
			}
		}
	}
	return "", errors.New("no disk found")
}

// Mount 挂载dev到mountPoint. mountPoint目录会自动创建.重复挂载自动跳过,无影响.
// dev可以是设备路径例如:/dev/sda,也可以是分区的uuid
// protect=true,chmod 000 mountPoint
func Mount(dev, mountPoint string, protect bool) error {
	isUUID := !strings.HasPrefix(dev, "/")
	if m, e := GetMountPoint(dev); e == nil && m != "" {
		return nil
	} else if e != nil {
		return fmt.Errorf("get mount status: %s", e)
	}
	_ = os.MkdirAll(mountPoint, 0777)
	if protect {
		_ = os.Chmod(mountPoint, 0000)
	}
	// mount --source UUID=f45d84d6-76c0-49a1-afd9-98e9a86f26a3 --target /mnt/sda1
	args := make([]string, 0, 4)
	if isUUID {
		args = append(args, "--source", "UUID="+dev, "--target", mountPoint)
	} else {
		args = append(args, "--source", dev, "--target", mountPoint)
	}
	c := exec.Command("mount", args...)
	return c.Run()
}

// Umount 卸载分区的挂载,path可以是磁盘设备例如:/dev/sda1 ,也可以是挂载点路径例如: /mnt/sda1
func Umount(path string) error {
	c := exec.Command("umount", "-f", path)
	return c.Run()
}

// MakeExt4 格式化dev为ext4格式
func MakeExt4(dev string) error {
	c := exec.Command("mkfs", "-t", "ext4", "-F", dev)
	err := c.Run()
	time.Sleep(time.Millisecond * 100)
	return err
}

// Parted 使用linux的parted程序将dev分成一个ext4分区,容量为全部容量
func Parted(dev string) error {
	err := exec.Command("parted", dev, "--script", "mklabel", "gpt").Run()
	if err != nil {
		return err
	}
	err = exec.Command("parted", dev, "--script", "mkpart", "primary", "ext4", "0%", "100%").Run()
	time.Sleep(time.Millisecond * 100)
	return err
}
