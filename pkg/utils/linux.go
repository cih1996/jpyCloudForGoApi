//go:build android || linux

package utils

import (
	"bytes"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func GetUid(name string) (int, error) {
	cmd := exec.Command("id", "-u", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(output)))
}
func GetGid(name string) (int, error) {
	cmd := exec.Command("id", "-g", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(output)))
}
func LookupUser(name string) (uint32, uint32, error) {
	uid, err := GetUid(name)
	if err != nil {
		return 0, 0, err
	}
	gid, err := GetGid(name)
	if err != nil {
		return 0, 0, err
	}
	return uint32(uid), uint32(gid), nil
}
func CurrentUser() string {
	cmd := exec.Command("id", "-un")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// Passwd 修改linux用户的密码，必须以root身份运行
func Passwd(name, passwd string) error {
	if len(name) < 1 {
		return errors.New("name is null")
	}
	if len(passwd) < 1 {
		return errors.New("password is null")
	}
	if runtime.GOOS != "linux" {
		return errors.New("only support linux")
	}
	current, err := user.Current()
	if err != nil {
		return fmt.Errorf("get current user error,%w", err)
	}
	if current.Username != "root" {
		return errors.New("must run as root")
	}
	cmd := exec.Command("chpasswd")
	cmd.Stdin = strings.NewReader(fmt.Sprintf("%s:%s", name, passwd))
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("failed to change password: %w", err)
	}
	return nil
}

// GetCpuFreGov 获取linux下当前的cpu性能方案
func GetCpuFreGov() []string {
	fPath := "/sys/devices/system/cpu/cpu0/cpufreq/scaling_available_governors"
	buf, err := os.ReadFile(fPath)
	if err != nil {
		return nil
	}
	g := bytes.Split(bytes.TrimSpace(buf), []byte{0x20})
	var ret []string
	for _, b := range g {
		ret = append(ret, string(b))
	}
	return ret
}

// SetCpuFreGov 设置linux的cpu性能方案，powersave userspace conservative ondemand performance schedutil
func SetCpuFreGov(gov string) error {
	//cpufreq-set -g performance
	govs := GetCpuFreGov()
	if !HasItem(govs, gov) {
		return fmt.Errorf("%v excluding %s", govs, gov)
	}

	fPath := "/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor"
	if _, err := os.Stat(fPath); err != nil {
		return errors.New("can not find governor")
	}
	buf, err := os.ReadFile(fPath)
	if err != nil {
		return errors.New("can not read governor")
	}
	now := strings.TrimSpace(string(buf))
	if now != gov {
		err = os.WriteFile(fPath, []byte(gov), 0644)
		if err != nil {
			return errors.New("set governor fail")
		}
	}
	return nil
}

type Pid int32

func Processes() ([]Pid, error) {
	names, err := ReadDirNames("/proc")
	if err != nil {
		return nil, err
	}
	pids := make([]Pid, len(names))
	var pid int64
	for i := 0; i < len(names); i++ {
		pid, err = strconv.ParseInt(names[i], 10, 32)
		if err != nil {
			// if not numeric name, just skip
			continue
		}
		pids = append(pids, Pid(pid))
	}
	return pids, nil
}

func FindProcess(name string) (Pid, error) {
	procs, err := Processes()
	if err != nil {
		return 0, err
	}
	for _, proc := range procs {

		cmdline, err := proc.Cmdline()
		if err != nil {
			continue
		}
		if strings.Contains(cmdline, name) {
			return proc, nil
		}
	}
	return 0, fmt.Errorf("process %s not found", name)
}

func Pkill(name string) (string, error) {
	out, err := GetCommandOutWithTimeout("pkill", time.Second*3, "-f", name)
	return out, err
}

// Exists 检查进程是否还活着
func (p Pid) Exists() bool {
	if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(int(p)))); err == nil {
		return true
	}
	return false
}

// Cmdline 取得该进程的命令行
func (p Pid) Cmdline() (string, error) {
	file, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(int(p)), "cmdline"))
	if err != nil {
		return "", err
	}
	return string(file), nil
}

// Terminate 发送 SIGTERM消息给进程
func (p Pid) Terminate() error {
	process, err := os.FindProcess(int(p))
	if err != nil {
		return err
	}
	return process.Signal(unix.SIGTERM)
}
