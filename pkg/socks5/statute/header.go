package statute

import (
	"errors"
	"net"
	"strconv"
)

type Header struct {
	Type    uint8  `json:"type" msgpack:"type"`                     // 请求类型,1=tcp,2=udp
	Version uint8  `json:"version" msgpack:"version"`               // 版本号,5=socks5
	ATYP    uint8  `json:"ATYP" msgpack:"ATYP"`                     // 地址类型,1=ipv4,3=域名,4=ipv6
	Addr    []byte `json:"addr" msgpack:"addr"`                     // 目标地址
	Port    uint16 `json:"port" msgpack:"port"`                     // 目标端口
	Data    []byte `json:"data,omitempty" msgpack:"data,omitempty"` //附件数据,请求的时候nil,返回有错误的时候非nil
}

func (s *Header) VerifyAddr() error {
	if s.ATYP == ATYPIPv4 {
		if len(s.Addr) != 4 {
			return errors.New("ipv4 len error")
		}
	} else if s.ATYP == ATYPIPv6 {
		if len(s.Addr) != 16 {
			return errors.New("ipv6 len error")
		}
	} else if s.ATYP == ATYPDomain {
		if len(s.Addr) < 1 || len(s.Addr) > 255 {
		}
	}
	return nil
}
func (s *Header) AddrToString() string {
	if s.ATYP == ATYPIPv4 {
		return net.JoinHostPort(net.IP(s.Addr).String(), strconv.Itoa(int(s.Port)))
	} else if s.ATYP == ATYPIPv6 {
		return net.JoinHostPort(net.IP(s.Addr).String(), strconv.Itoa(int(s.Port)))
	} else if s.ATYP == ATYPDomain {
		return net.JoinHostPort(string(s.Addr), strconv.Itoa(int(s.Port)))
	}
	return ""
}
