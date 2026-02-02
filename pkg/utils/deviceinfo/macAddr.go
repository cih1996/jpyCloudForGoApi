package deviceinfo

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
)

// GetHardwareAddr ifName不为空，按ifName取，否则取当前上网网卡的，依然没取到，就取第一张卡的
func GetHardwareAddr(ifName string) (string, error) {
	i, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, netcard := range i {
		if netcard.Name != "lo" {
			if ifName != "" && ifName == netcard.Name {
				//传了连接名，按连接名
				return netcard.HardwareAddr.String(), nil
			} else {
				if netcard.Flags&net.FlagRunning > 0 {
					return netcard.HardwareAddr.String(), nil
				}
			}
		}
	}
	for _, netcard := range i {
		if netcard.Name != "lo" {
			return netcard.HardwareAddr.String(), nil
		}
	}
	return "", errors.New("not found")
}

// MAC2Uint64 mac地址字符串转uint64格式
func MAC2Uint64(mac string) (ret uint64, err1 error) {
	var tmp uint64
	var u []byte
	m, err := net.ParseMAC(mac)
	if err != nil {
		return 0, err
	}
	u = append(u, byte(0x0), byte(0x0))
	u = append(u, m...)
	bytesBuffer := bytes.NewBuffer(u)
	err = binary.Read(bytesBuffer, binary.BigEndian, &tmp)
	return tmp, err
}
