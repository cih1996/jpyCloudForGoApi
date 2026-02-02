//go:build android || linux

package utils

import (
	"bufio"
	"context"
	"errors"
	"github.com/shogo82148/androidbinary/apk"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ApkInfo 从apk文件解析出包名和版本
func ApkInfo(f string) (packageName, versionName string, err error) {
	pkg, err := apk.OpenFile(f)
	if err != nil {
		return "", "", err
	}
	defer pkg.Close()
	manifest := pkg.Manifest()
	packageName = pkg.PackageName()
	versionName, err = manifest.VersionName.String()
	return packageName, versionName, err
}

// PackageExist 检查安卓系统是否安装了指定的apk
// 例如:com.ss.android.ugc.aweme 抖音
func PackageExist(name string) bool {
	//pm list package xxx
	ret, err := GetCommandOutWithTimeout("pm", time.Second*5, "list", "package", name)
	if err != nil {
		return false
	}
	return strings.HasSuffix(strings.TrimSpace(ret), name)
}

// PackageInstall 安装apk.f为apk文件路径
func PackageInstall(f string) error {
	if _, err := os.Stat(f); err != nil {
		return err
	}
	ret, err := GetCommandOutWithTimeout("pm", time.Minute*3, "install", f)
	if err != nil {
		return err
	}
	if strings.Contains(strings.ToLower(ret), "success") {
		return nil
	}
	return errors.New("install failed: " + ret)
}

// PackageUninstall 卸载包,f为包名称,比如抖音:com.ss.android.ugc.aweme
func PackageUninstall(f string) error {
	//pm uninstall com.ss.appservice
	ret, err := GetCommandOutWithTimeout("pm", time.Second*60, "uninstall", f)
	if err != nil {
		return err
	}
	if strings.Contains(strings.ToLower(ret), "success") {
		return nil
	}
	return errors.New("uninstall failed: " + ret)
}

// SetPermissions 循环调用pm grant com.ss.example android.permission.ACCESS_BACKGROUND_LOCATION
// 为指定的包,赋予权限
func SetPermissions(name string, args ...string) error {
	var err error
	for _, v := range args {
		_, err = GetCommandOutWithTimeout("pm", time.Second*3, "grant", name, v)
	}
	return err
}

type Permission struct {
	Name    string
	Granted bool
}

// GetPackageInfoByName 解析dumpsys package com.ss.android.ugc.aweme 的返回值中的versionName,和 runtime permissions:
// ret []Permission, version *string 传nil表示不取该项.
func GetPackageInfoByName(name string, timeout time.Duration, ret *[]Permission, version *string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "dumpsys", "package", name)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	defer stdout.Close()
	if err = cmd.Start(); err != nil {
		return err
	}
	ParserPackageInfo(stdout, ret, version)
	if err = cmd.Wait(); err != nil {
		return err
	}
	return err
}

func ParserPackageInfo(r io.ReadCloser, ret *[]Permission, version *string) {
	scanner := bufio.NewScanner(r)
	var i int
	var start = 0xFFFFFFFF
	var line string
	var granted bool
	var after string
	var found bool
	for scanner.Scan() {
		i++
		line = scanner.Text()
		if strings.Contains(line, "versionName=") { //处理版本号
			if _, after, found = strings.Cut(line, "="); found {
				if version != nil {
					*version = strings.TrimSpace(after)
				}
			}
			continue
		}
		if ret == nil { //不取权限列表,跳过
			continue
		}

		if strings.Contains(line, "runtime permissions:") {
			start = i
		}
		if start < i { //已经找到起始位置了.
			if len(line) < 10 { //到达结束位置
				break
			}
			split := strings.SplitN(line, ":", 2)
			if len(split) == 2 {
				split2 := strings.Split(split[1], "=")
				if len(split2) > 1 && strings.Contains(split2[0], "granted") {
					if strings.Contains(split2[1], "true") {
						granted = true
					} else {
						granted = false
					}
					p := Permission{Name: strings.TrimSpace(split[0]), Granted: granted}
					*ret = append(*ret, p)
				}
			}
		}
	}
	return
}
