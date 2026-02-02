package utils

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// GetAppPath 获取程序启动目录
func GetAppPath() string {
	if path, err := filepath.Abs(filepath.Dir(os.Args[0])); err == nil {
		return path
	}
	return os.Args[0]
}

func GetLocalUdpAddr() (string, error) {
	tmpConn, err := net.Dial("udp", "114.114.114.114:53")
	if err != nil {
		return "", err
	}
	defer tmpConn.Close()
	ip, _ := SplitIpString(tmpConn.LocalAddr().String())
	return ip, nil
}
func SplitIpString(str string) (ip string, port int) {
	s := strings.SplitN(str, ":", 2)
	if len(s) == 2 {
		ip = s[0]
		port, _ = strconv.Atoi(s[1])
	}
	return
}

func GetCommandOutWithTimeout(filename string, timeout time.Duration, args ...string) (ret string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, filename, args...)

	output, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		cmd.Process.Kill()
		return "", fmt.Errorf("command execution timed out")
	}
	if err != nil {
		return string(output), fmt.Errorf("command execution failed: %v", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetStringFromDns 解析域名为txt，txt为base64，转[]byte后aes解密后返回解密[]byte
func GetStringFromDns(domain, aesKey string) []byte {
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

type Uint40 uint64

func NewUint40(high uint32, low uint8) Uint40 {
	return Uint40(uint64(high)<<8 | uint64(low))
}
func (i Uint40) Split() (high uint32, low uint8) {
	high = uint32(i >> 8)
	low = uint8(i & 0xFF)
	return
}
func (i Uint40) High() uint32 {
	return uint32(i >> 8)
}
func (i Uint40) Low() uint8 {
	return uint8(i & 0xFF)
}
func GetRndPwd(pwdLen int) string {
	var ret string
	var r int
	str := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	rnd := rand.New(rand.NewSource(time.Now().UnixNano() + rand.Int63()))
	l := len(str) - 1
	for i := 0; i < pwdLen; i++ {
		r = rnd.Intn(l)
		ret = ret + str[r:r+1]
	}
	return ret
}
func ReadDirNames(dirname string) ([]string, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	names, err := f.Readdirnames(-1)
	_ = f.Close()
	if err != nil {
		return nil, err
	}
	slices.Sort(names)
	return names, nil
}
func HasItem[T comparable](slice []T, item T) bool {
	for _, s := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func ChannelTryPush[T any](ch chan<- T, val T) bool {
	select {
	case ch <- val:
		return true
	default:
		return false
	}
}

func AtomicSubtractUint64(addr *uint64, delta uint64) uint64 {
	return atomic.AddUint64(addr, ^(delta - 1))
}

func AtomicSubtractUint32(addr *uint32, delta uint32) uint32 {
	return atomic.AddUint32(addr, ^(delta - 1))
}
