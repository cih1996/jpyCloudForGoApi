package utils

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"time"
	"unsafe"
)

func InetAddrToN(addr net.HardwareAddr) uint64 {
	if len(addr) < 6 {
		return 0
	}
	return uint64(addr[0])<<40 | uint64(addr[1])<<32 | uint64(addr[2])<<24 |
		uint64(addr[3])<<16 | uint64(addr[4])<<8 | uint64(addr[5])
}

func InetNToMAC(u uint64) net.HardwareAddr {
	mac := make(net.HardwareAddr, 6)
	mac[0] = byte(u >> 40)
	mac[1] = byte(u >> 32)
	mac[2] = byte(u >> 24)
	mac[3] = byte(u >> 16)
	mac[4] = byte(u >> 8)
	mac[5] = byte(u)
	return mac
}

func InetIPtoN(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return binary.BigEndian.Uint32(ip)
}

func InetNtoIP(u uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, u)
	return ip
}
func InetNtoMask(u uint32) net.IPMask {
	ip := make(net.IPMask, 4)
	binary.BigEndian.PutUint32(ip, u)
	return ip
}

var nativeEndian binary.ByteOrder

// NativeEndian 判断当前系统支持的大小端
func NativeEndian() binary.ByteOrder {
	if nativeEndian == nil {
		var x uint32 = 0x01020304
		if *(*byte)(unsafe.Pointer(&x)) == 0x01 {
			nativeEndian = binary.BigEndian
		} else {
			nativeEndian = binary.LittleEndian
		}
	}
	return nativeEndian
}

// InetNtoA 整数IP转点分
func InetNtoA(ip uint32) string {
	return InetNtoIP(ip).String()
	//return fmt.Sprintf("%d.%d.%d.%d",
	//	byte(ip), byte(ip>>8), byte(ip>>16), byte(ip>>24))
}

// InetAtoN 点分IP转整数
func InetAtoN(ip string) (i uint32) {
	binary.Read(bytes.NewReader(net.ParseIP(ip).To4()), binary.BigEndian, &i)
	return
}

// InetAtoNLe 点分IP转整数
func InetAtoNLe(ip string) (i uint32) {
	binary.Read(bytes.NewReader(net.ParseIP(ip).To4()), binary.LittleEndian, &i)
	return
}

// InetMaskToLen 如 255.255.255.0 对应的网络位长度为 24
func InetMaskToLen(netmask string) (int, error) {
	ip := net.ParseIP(netmask)
	if ip == nil {
		return 0, errors.New("netmask error")
	}
	mask := net.IPMask(ip.To4())
	if mask == nil {
		return 0, errors.New("netmask error")
	}
	ones, _ := mask.Size()
	return ones, nil
}

// InetLenToMask 如 24 对应的子网掩码地址为 255.255.255.0
func InetLenToMask(subnet int) string {
	var buff bytes.Buffer
	for i := 0; i < subnet; i++ {
		buff.WriteString("1")
	}
	for i := subnet; i < 32; i++ {
		buff.WriteString("0")
	}
	masker := buff.String()
	a, _ := strconv.ParseUint(masker[:8], 2, 64)
	b, _ := strconv.ParseUint(masker[8:16], 2, 64)
	c, _ := strconv.ParseUint(masker[16:24], 2, 64)
	d, _ := strconv.ParseUint(masker[24:32], 2, 64)
	resultMask := fmt.Sprintf("%v.%v.%v.%v", a, b, c, d)
	return resultMask
}

// GetIpRange ip掩码计算起始ip和结束ip
func GetIpRange(ipNet *net.IPNet) (min, max uint32) {
	ip := ipNet.IP.To4()
	for i := 0; i < 4; i++ {
		b := uint32(ip[i] & ipNet.Mask[i])
		min += b << ((3 - uint(i)) * 8)
	}
	one, _ := ipNet.Mask.Size()
	max = min | uint32(math.Pow(2, float64(32-one))-1)
	// max 是广播地址，忽略
	// i & 0x000000ff  == 0 是尾段为0的IP，根据RFC的规定，忽略
	return
}
func GetIpRange2(ipNum uint32, maskNum int) (min, max uint32) {
	ip := net.ParseIP(InetNtoA(ipNum))
	mask := net.CIDRMask(maskNum, 32)
	ipnet := net.IPNet{IP: ip, Mask: mask}
	return GetIpRange(&ipnet)
}
func IsContainsIp(ipStr, gatewayStr, maskStr string) error {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return errors.New("ip地址错误")
	}
	mask := net.ParseIP(maskStr)
	if mask == nil {
		return errors.New("子网掩码错误")
	}
	gateway := net.ParseIP(gatewayStr)
	if gateway == nil {
		return errors.New("网关地址错误")
	}

	ipNetwork := net.IPNet{IP: ip, Mask: net.IPMask(mask.To4())}
	if !ipNetwork.Contains(gateway) {
		return errors.New("ip地址和网关不在同一个子网内")
	}
	return nil
}

// VerifyIPString 校验点分ip是否格式正确
func VerifyIPString(ipStr string) error {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return errors.New("ip地址错误")
	}
	return nil
}

// MAC2Uint64 mac地址字符串转uint64格式
func MAC2Uint64(mac string) (ret uint64, err error) {
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

// Uint64ToMAC uint64转mac地址字符串，冒号分割，小写
func Uint64ToMAC(u uint64) string {
	bytesBuffer := bytes.NewBuffer([]byte{})
	_ = binary.Write(bytesBuffer, binary.BigEndian, &u)
	var mac net.HardwareAddr
	mac = bytesBuffer.Bytes()[2:]
	return mac.String()
}

// GetLocalIP 获取本机上网ip,传入远端地址,比如:114.114.114.114:53
func GetLocalIP(ipv6 bool) (string, error) {
	var addr string
	if ipv6 {
		addr = "[2001:4860:4860::8888]:53"
	} else {
		addr = "8.8.8.8:53"
	}
	tmpConn, err := net.Dial("udp", addr)
	if err != nil {
		return "", err
	}
	defer tmpConn.Close()
	ip, _, err := net.SplitHostPort(tmpConn.LocalAddr().String())
	return ip, nil
}

func GetInternetTime(url ...string) time.Time {
	if url == nil {
		url = []string{
			"https://www.baidu.com",
			"https://www.163.com",
			"https://www.taobao.com",
			"https://www.sogou.com",
			"https://www.sina.com",
			"https://www.google.com",
			"https://time.is",
		}
	}

	var times []time.Time
	for _, v := range url {
		res, err := http.Head(v)
		if err != nil {
			continue
		}
		str := res.Header.Get("Date")
		if str == "" {
			continue
		}
		if t, e := time.ParseInLocation("Mon, 02 Jan 2006 03:04:05 GMT", str, time.Local); e == nil {
			times = append(times, t)
		}
	}
	if len(times) > 0 {
		rnd := rand.New(rand.NewSource(time.Now().UnixMilli()))
		return times[rnd.Intn(len(times)-1)]
	}
	return time.Time{}
}

// GetDataFromDns 解析域名为txt，txt为base64，转[]byte后aes解密后返回解密[]byte
func GetDataFromDns(domain, aesKey string) []byte {
	//"industrial.accbot.cn"
	txt, err := net.LookupTXT(domain)
	if err != nil {
		return nil
	}
	for _, s := range txt {
		buf, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			continue
		}
		decrypt, err := AesDecrypt(buf, aesKey)
		if err != nil {
			continue
		}
		return decrypt
	}
	return nil
}
