package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// APIRequest 发送 API 请求
func APIRequest(serverURL, secretKey, method, path string, body interface{}) (map[string]interface{}, error) {
	url := strings.TrimRight(serverURL, "/") + path

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if secretKey != "" {
		req.Header.Set("Authorization", "Bearer "+secretKey)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respData, &result); err != nil {
		return map[string]interface{}{"raw": string(respData)}, nil
	}

	return result, nil
}

// GetDevices 获取设备列表
func GetDevices(serverURL, secretKey string) error {
	result, err := APIRequest(serverURL, secretKey, "GET", "/api/devicews/devices", nil)
	if err != nil {
		return err
	}

	devices, ok := result["devices"].([]interface{})
	if !ok || len(devices) == 0 {
		fmt.Println("暂无设备连接")
		return nil
	}

	fmt.Printf("%-12s %-20s %-15s %-10s\n", "设备ID", "序列号", "型号", "状态")
	fmt.Println(strings.Repeat("-", 60))

	for _, d := range devices {
		dev := d.(map[string]interface{})
		deviceID := fmt.Sprintf("%08X", int(dev["deviceId"].(float64)))
		serialno := ""
		if s, ok := dev["serialno"].(string); ok {
			serialno = s
		}
		model := ""
		if m, ok := dev["model"].(string); ok {
			model = m
		}
		fmt.Printf("%-12s %-20s %-15s %-10s\n", deviceID, serialno, model, "在线")
	}

	return nil
}

// ExecuteShell 执行 Shell 命令
func ExecuteShell(serverURL, secretKey, deviceID, command string) error {
	body := map[string]interface{}{
		"deviceId": deviceID,
		"command":  command,
	}

	result, err := APIRequest(serverURL, secretKey, "POST", "/api/devicews/shell", body)
	if err != nil {
		return err
	}

	if output, ok := result["output"].(string); ok {
		fmt.Println(output)
	} else {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	}

	return nil
}

// Screenshot 截图
func Screenshot(serverURL, secretKey, deviceID, output string) error {
	body := map[string]interface{}{
		"deviceId": deviceID,
	}

	result, err := APIRequest(serverURL, secretKey, "POST", "/api/devicews/screenshot", body)
	if err != nil {
		return err
	}

	if _, ok := result["data"].(string); ok {
		if output == "" {
			output = fmt.Sprintf("screenshot_%s.png", time.Now().Format("20060102_150405"))
		}
		fmt.Printf("截图已保存: %s\n", output)
	} else {
		return fmt.Errorf("截图失败")
	}

	return nil
}

// GetInstallPath 获取安装路径
func GetInstallPath() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "jpy-server", "jpy-server.exe")
	default:
		return "/usr/local/bin/jpy-server"
	}
}

// GetDataDir 获取数据目录
func GetDataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".jpy-server")
}

// ServiceInstall 安装服务
func ServiceInstall(binPath, workDir string) error {
	// 确保数据目录存在
	os.MkdirAll(GetDataDir(), 0755)

	switch runtime.GOOS {
	case "darwin":
		return installMacService(binPath, workDir)
	case "linux":
		return installLinuxService(binPath, workDir)
	case "windows":
		return installWindowsService(binPath, workDir)
	default:
		return fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}
}

// ServiceUninstall 卸载服务
func ServiceUninstall() error {
	switch runtime.GOOS {
	case "darwin":
		return uninstallMacService()
	case "linux":
		return uninstallLinuxService()
	case "windows":
		return uninstallWindowsService()
	default:
		return fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}
}

// ServiceStart 启动服务
func ServiceStart() error {
	switch runtime.GOOS {
	case "darwin":
		return runCommand("launchctl", "load", getMacPlistPath())
	case "linux":
		return runCommand("systemctl", "--user", "start", "jpy-server")
	case "windows":
		return runCommand("sc", "start", "jpy-server")
	default:
		return fmt.Errorf("不支持的操作系统")
	}
}

// ServiceStop 停止服务
func ServiceStop() error {
	switch runtime.GOOS {
	case "darwin":
		return runCommand("launchctl", "unload", getMacPlistPath())
	case "linux":
		return runCommand("systemctl", "--user", "stop", "jpy-server")
	case "windows":
		return runCommand("sc", "stop", "jpy-server")
	default:
		return fmt.Errorf("不支持的操作系统")
	}
}

// ServiceStatus 服务状态
func ServiceStatus() error {
	switch runtime.GOOS {
	case "darwin":
		return runCommand("launchctl", "list", "com.jpy.server")
	case "linux":
		return runCommand("systemctl", "--user", "status", "jpy-server")
	case "windows":
		return runCommand("sc", "query", "jpy-server")
	default:
		return fmt.Errorf("不支持的操作系统")
	}
}

// macOS 服务安装
func installMacService(binPath, workDir string) error {
	plistPath := getMacPlistPath()
	dataDir := GetDataDir()

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.jpy.server</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>serve</string>
    </array>
    <key>WorkingDirectory</key>
    <string>%s</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>%s/stdout.log</string>
    <key>StandardErrorPath</key>
    <string>%s/stderr.log</string>
</dict>
</plist>`, binPath, workDir, dataDir, dataDir)

	// 确保目录存在
	os.MkdirAll(filepath.Dir(plistPath), 0755)

	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		return err
	}

	fmt.Printf("服务配置已写入: %s\n", plistPath)
	fmt.Println("启动服务: jpy-server service start")
	return nil
}

func uninstallMacService() error {
	ServiceStop()
	plistPath := getMacPlistPath()
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	fmt.Println("服务已卸载")
	return nil
}

func getMacPlistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", "com.jpy.server.plist")
}

// Linux 服务安装
func installLinuxService(binPath, workDir string) error {
	home, _ := os.UserHomeDir()
	serviceDir := filepath.Join(home, ".config", "systemd", "user")
	servicePath := filepath.Join(serviceDir, "jpy-server.service")

	service := fmt.Sprintf(`[Unit]
Description=JPY Server - 集控平台本地代理
After=network.target

[Service]
Type=simple
ExecStart=%s serve
WorkingDirectory=%s
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
`, binPath, workDir)

	os.MkdirAll(serviceDir, 0755)
	if err := os.WriteFile(servicePath, []byte(service), 0644); err != nil {
		return err
	}

	runCommand("systemctl", "--user", "daemon-reload")
	runCommand("systemctl", "--user", "enable", "jpy-server")

	fmt.Printf("服务配置已写入: %s\n", servicePath)
	fmt.Println("启动服务: jpy-server service start")
	return nil
}

func uninstallLinuxService() error {
	ServiceStop()
	runCommand("systemctl", "--user", "disable", "jpy-server")

	home, _ := os.UserHomeDir()
	servicePath := filepath.Join(home, ".config", "systemd", "user", "jpy-server.service")
	os.Remove(servicePath)
	runCommand("systemctl", "--user", "daemon-reload")

	fmt.Println("服务已卸载")
	return nil
}

// Windows 服务安装
func installWindowsService(binPath, workDir string) error {
	// 使用 sc 命令创建服务
	cmd := exec.Command("sc", "create", "jpy-server",
		fmt.Sprintf("binPath=%s serve", binPath),
		"start=auto",
		"DisplayName=JPY Server")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("创建服务失败（需要管理员权限）: %v", err)
	}

	fmt.Println("服务已创建")
	fmt.Println("启动服务: jpy-server service start")
	return nil
}

func uninstallWindowsService() error {
	ServiceStop()
	cmd := exec.Command("sc", "delete", "jpy-server")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("删除服务失败（需要管理员权限）: %v", err)
	}
	fmt.Println("服务已卸载")
	return nil
}

// Install 安装程序到系统
func Install(srcPath string) error {
	dstPath := GetInstallPath()

	// 确保目标目录存在
	os.MkdirAll(filepath.Dir(dstPath), 0755)

	// 复制文件
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("创建文件失败（可能需要管理员权限）: %v", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	// 设置执行权限
	os.Chmod(dstPath, 0755)

	fmt.Printf("已安装到: %s\n", dstPath)

	// 提示添加到 PATH
	switch runtime.GOOS {
	case "windows":
		fmt.Println("\n请将以下路径添加到系统 PATH 环境变量:")
		fmt.Printf("  %s\n", filepath.Dir(dstPath))
	case "darwin", "linux":
		if !strings.Contains(os.Getenv("PATH"), filepath.Dir(dstPath)) {
			fmt.Println("\n程序已安装到 /usr/local/bin，通常已在 PATH 中")
		}
	}

	return nil
}

// Uninstall 卸载程序
func Uninstall() error {
	// 先卸载服务
	ServiceUninstall()

	// 删除程序
	dstPath := GetInstallPath()
	if err := os.Remove(dstPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除程序失败（可能需要管理员权限）: %v", err)
	}

	// 删除数据目录
	os.RemoveAll(GetDataDir())

	fmt.Println("卸载完成")
	return nil
}

// Upgrade 升级程序
func Upgrade(newBinPath string) error {
	currentPath := GetInstallPath()

	// 检查当前是否已安装
	if _, err := os.Stat(currentPath); os.IsNotExist(err) {
		fmt.Println("程序未安装，执行安装...")
		return Install(newBinPath)
	}

	fmt.Printf("升级: %s -> %s\n", newBinPath, currentPath)

	// 1. 停止服务
	fmt.Println("停止服务...")
	ServiceStop()

	// 2. 备份旧版本
	backupPath := currentPath + ".backup"
	fmt.Printf("备份旧版本: %s\n", backupPath)
	os.Rename(currentPath, backupPath)

	// 3. 安装新版本
	fmt.Println("安装新版本...")
	if err := Install(newBinPath); err != nil {
		// 恢复备份
		os.Rename(backupPath, currentPath)
		return fmt.Errorf("升级失败: %v", err)
	}

	// 4. 启动服务
	fmt.Println("启动服务...")
	if err := ServiceStart(); err != nil {
		fmt.Printf("启动失败: %v\n", err)
		fmt.Println("可手动启动: jpy-server service start")
	}

	// 5. 删除备份
	os.Remove(backupPath)

	fmt.Println("升级完成!")
	return nil
}

// runCommand 执行命令
func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
