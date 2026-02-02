//go:build linux
// +build linux

package deviceinfo

import (
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"strings"
	"unsafe"
)

const (
	IFNAMSIZ          = 16 // Maximum size of an interface name
	SIOCETHTOOL       = 0x8946
	PERMADDR_LEN      = 32
	ETHTOOL_GPERMADDR = 0x00000020 /* Get permanent hardware address */
)

type ifreq struct {
	ifr_name [IFNAMSIZ]byte
	ifr_data uintptr
}
type ethtoolPermAddr struct {
	cmd  uint32
	size uint32
	data [PERMADDR_LEN]byte
}

func GetRealMac(ifName string) (mac string, err error) {
	permAddr, err := getPermAddr(ifName)
	if err != nil {
		return
	}
	if permAddr.data[0] == 0 && permAddr.data[1] == 0 &&
		permAddr.data[2] == 0 && permAddr.data[3] == 0 &&
		permAddr.data[4] == 0 && permAddr.data[5] == 0 {
		return "", errors.New("not found")
	}
	//bytesBuffer := bytes.NewBuffer(permAddr.data[:6])
	//err = binary.Read(bytesBuffer, binary.BigEndian, &mac)
	m := fmt.Sprintf("%x:%x:%x:%x:%x:%x",
		permAddr.data[0:1],
		permAddr.data[1:2],
		permAddr.data[2:3],
		permAddr.data[3:4],
		permAddr.data[4:5],
		permAddr.data[5:6])
	return m, nil
}
func getPermAddr(intf string) (ethtoolPermAddr, error) {
	permAddr := ethtoolPermAddr{
		cmd:  ETHTOOL_GPERMADDR,
		size: PERMADDR_LEN,
	}

	if err := ioctl(intf, uintptr(unsafe.Pointer(&permAddr))); err != nil {
		return ethtoolPermAddr{}, err
	}
	return permAddr, nil
}
func ioctl(intf string, data uintptr) error {
	var name [IFNAMSIZ]byte
	copy(name[:], []byte(intf))

	ifr := ifreq{
		ifr_name: name,
		ifr_data: data,
	}
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, unix.IPPROTO_IP)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	_, _, ep := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), SIOCETHTOOL, uintptr(unsafe.Pointer(&ifr)))
	if ep != 0 {
		return ep
	}
	return nil
}
func GetLogicMac(ifName string) (mac string, err error) {
	devicePath := fmt.Sprintf("/sys/class/net/%s/address", ifName)
	buf, err := os.ReadFile(devicePath)
	if err != nil {
		return "", err
	}
	mac = strings.TrimSpace(string(buf))
	return
}
