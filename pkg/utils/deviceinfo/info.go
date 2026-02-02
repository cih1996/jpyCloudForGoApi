package deviceinfo

import (
	"bytes"
	"crypto/md5"
	"strconv"
)

func GetSystemHash() []byte {
	dev := NewDevInfo()
	host := NewHostInfo()
	buffer := bytes.NewBuffer([]byte{})
	buffer.WriteString(strconv.FormatUint(dev.PermAddr, 10))
	buffer.WriteString(dev.Cpu)
	buffer.WriteString(dev.BootDisk.Model)
	buffer.WriteString(dev.BootDisk.SerialNumber)
	buffer.WriteString(strconv.FormatUint(dev.BootDisk.Size, 10))
	buffer.WriteString(host.HostID)
	obj := md5.New()
	obj.Write(buffer.Bytes())
	return obj.Sum([]byte{})
}
