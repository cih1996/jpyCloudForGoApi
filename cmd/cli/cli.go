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

// 本地后端服务地址
const LocalServerURL = "http://127.0.0.1:1001"

// UnifiedRequest 统一 API 请求结构
type UnifiedRequest struct {
	Type  string      `json:"type"`
	Seq   int64       `json:"seq"`
	Token string      `json:"token,omitempty"`
	Host  string      `json:"host,omitempty"`
	Data  interface{} `json:"data,omitempty"`
}

// UnifiedResponse 统一 API 响应结构
type UnifiedResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// CallUnified 调用本地后端的统一 API
// platformURL: 集控平台地址（用于登录）
// apiKey: 集控平台 API 密钥
// reqType: 请求类型
// data: 请求数据
func CallUnified(platformURL, apiKey, reqType string, data interface{}) (*UnifiedResponse, error) {
	url := LocalServerURL + "/api/unified"

	reqBody := UnifiedRequest{
		Type:  reqType,
		Seq:   time.Now().UnixMilli(),
		Token: apiKey,
		Host:  platformURL, // 集控平台地址
		Data:  data,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("请求失败（本地服务是否已启动？）: %v", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result UnifiedResponse
	if err := json.Unmarshal(respData, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if result.Code != 200 {
		return nil, fmt.Errorf("API 错误: %s", result.Msg)
	}

	return &result, nil
}

// EnsureLogin 确保已登录集控平台
func EnsureLogin(platformURL, apiKey string) error {
	_, err := CallUnified(platformURL, apiKey, "Login", nil)
	return err
}

// OutputJSON 输出 JSON 格式
func OutputJSON(data interface{}) {
	jsonData, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(jsonData))
}

// GetDevices 获取设备列表
func GetDevices(platformURL, apiKey string, verbose, jsonOutput bool) error {
	// 先登录
	if err := EnsureLogin(platformURL, apiKey); err != nil {
		return fmt.Errorf("登录失败: %v", err)
	}

	// 获取设备列表
	result, err := CallUnified(platformURL, apiKey, "GetDeviceList", nil)
	if err != nil {
		return err
	}

	// 获取当前端口映射
	mappings := GetMappingsQuiet()

	// 解析设备列表
	dataMap, ok := result.Data.(map[string]interface{})
	if !ok {
		fmt.Println("暂无设备")
		return nil
	}

	// 尝试 records 字段（集控平台返回格式）
	devices, ok := dataMap["records"].([]interface{})
	if !ok || len(devices) == 0 {
		// 兼容 list 字段
		devices, ok = dataMap["list"].([]interface{})
		if !ok || len(devices) == 0 {
			if jsonOutput {
				OutputJSON(map[string]interface{}{"devices": []interface{}{}})
			} else {
				fmt.Println("暂无设备")
			}
			return nil
		}
	}

	// JSON 输出模式
	if jsonOutput {
		var deviceList []map[string]interface{}
		for _, d := range devices {
			record, ok := d.(map[string]interface{})
			if !ok {
				continue
			}
			dev, ok := record["deviceInfo"].(map[string]interface{})
			if !ok {
				dev = record
			}

			deviceID := 0
			if id, ok := dev["deviceId"].(float64); ok {
				deviceID = int(id)
			}

			item := map[string]interface{}{
				"deviceId": deviceID,
				"uuid":     dev["uuid"],
				"brand":    dev["brand"],
				"online":   dev["online"],
				"ip":       dev["ip"],
			}

			// S5 代理
			if s5str, ok := dev["s5info"].(string); ok && s5str != "" {
				var s5data map[string]interface{}
				if json.Unmarshal([]byte(s5str), &s5data) == nil {
					item["s5info"] = s5data
				}
			}

			// 隧道状态
			if ports, ok := mappings[deviceID]; ok && len(ports) > 0 {
				item["tunnels"] = ports
			}

			deviceList = append(deviceList, item)
		}
		OutputJSON(map[string]interface{}{"devices": deviceList})
		return nil
	}

	if verbose {
		// 详细模式
		fmt.Printf("%-10s %-18s %-10s %-8s %-16s %-30s %-10s\n",
			"设备ID", "序列号", "型号", "状态", "IP", "S5代理", "隧道")
		fmt.Println(strings.Repeat("-", 110))
	} else {
		fmt.Printf("%-10s %-18s %-10s %-8s %-10s\n", "设备ID", "序列号", "型号", "状态", "隧道")
		fmt.Println(strings.Repeat("-", 60))
	}

	for _, d := range devices {
		record, ok := d.(map[string]interface{})
		if !ok {
			continue
		}

		// 设备信息在 deviceInfo 字段中
		dev, ok := record["deviceInfo"].(map[string]interface{})
		if !ok {
			// 兼容直接返回设备信息的格式
			dev = record
		}

		deviceID := 0
		deviceIDStr := ""
		if id, ok := dev["deviceId"].(float64); ok {
			deviceID = int(id)
			deviceIDStr = fmt.Sprintf("%d", deviceID)
		}

		serialno := ""
		if s, ok := dev["uuid"].(string); ok {
			serialno = s
		}

		model := ""
		if m, ok := dev["brand"].(string); ok {
			model = m
		}

		status := "离线"
		if online, ok := dev["online"].(bool); ok && online {
			status = "在线"
		}

		ip := ""
		if i, ok := dev["ip"].(string); ok {
			ip = i
		}

		// 解析 S5 代理信息
		s5Info := "-"
		if s5str, ok := dev["s5info"].(string); ok && s5str != "" {
			var s5data map[string]interface{}
			if json.Unmarshal([]byte(s5str), &s5data) == nil {
				if s5url, ok := s5data["s5Url"].(string); ok {
					// 简化显示
					if len(s5url) > 28 {
						s5Info = s5url[:28] + "..."
					} else {
						s5Info = s5url
					}
				}
			}
		}

		// 获取隧道状态
		tunnelInfo := "-"
		if ports, ok := mappings[deviceID]; ok && len(ports) > 0 {
			portStrs := make([]string, len(ports))
			for i, p := range ports {
				portStrs[i] = fmt.Sprintf("%d", p)
			}
			tunnelInfo = strings.Join(portStrs, ",")
		}

		if verbose {
			fmt.Printf("%-10s %-18s %-10s %-8s %-16s %-30s %-10s\n",
				deviceIDStr, serialno, model, status, ip, s5Info, tunnelInfo)
		} else {
			fmt.Printf("%-10s %-18s %-10s %-8s %-10s\n",
				deviceIDStr, serialno, model, status, tunnelInfo)
		}
	}

	return nil
}

// GetMappingsQuiet 静默获取当前端口映射
func GetMappingsQuiet() map[int][]int {
	result := make(map[int][]int)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(LocalServerURL+"/api/mappings", "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		return result
	}
	defer resp.Body.Close()

	var data struct {
		Mappings []struct {
			DeviceID  int `json:"deviceId"`
			LocalPort int `json:"localPort"`
		} `json:"mappings"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return result
	}

	for _, m := range data.Mappings {
		result[m.DeviceID] = append(result[m.DeviceID], m.LocalPort)
	}

	return result
}

// GetMappings 获取当前端口映射列表
func GetMappings(jsonOutput bool) error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(LocalServerURL+"/api/mappings", "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		return fmt.Errorf("请求失败（本地服务是否已启动？）: %v", err)
	}
	defer resp.Body.Close()

	var data struct {
		Mappings []struct {
			DeviceID  int `json:"deviceId"`
			LocalPort int `json:"localPort"`
			PhonePort int `json:"phonePort"`
		} `json:"mappings"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	if jsonOutput {
		OutputJSON(data)
		return nil
	}

	if len(data.Mappings) == 0 {
		fmt.Println("当前无活跃的端口映射")
		return nil
	}

	fmt.Printf("%-12s %-12s %-12s\n", "设备ID", "本地端口", "远程端口")
	fmt.Println(strings.Repeat("-", 40))

	for _, m := range data.Mappings {
		fmt.Printf("%-12d %-12d %-12d\n", m.DeviceID, m.LocalPort, m.PhonePort)
	}

	return nil
}

// Connect 建立端口映射（隧道）
func Connect(platformURL, apiKey string, deviceID, localPort, phonePort int, jsonOutput bool) error {
	// 先登录
	if err := EnsureLogin(platformURL, apiKey); err != nil {
		return fmt.Errorf("登录失败: %v", err)
	}

	reqBody := map[string]interface{}{
		"key":       apiKey,
		"deviceId":  deviceID,
		"localPort": localPort,
		"phonePort": phonePort,
	}

	jsonData, _ := json.Marshal(reqBody)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(LocalServerURL+"/api/connect", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	if !result.Success {
		return fmt.Errorf("映射失败: %s", result.Message)
	}

	if jsonOutput {
		OutputJSON(map[string]interface{}{
			"success":   true,
			"deviceId":  deviceID,
			"localPort": localPort,
			"phonePort": phonePort,
		})
	} else {
		fmt.Printf("✓ 端口映射建立成功: 本地 %d -> 设备 %d 端口 %d\n", localPort, deviceID, phonePort)
	}
	return nil
}

// Disconnect 断开端口映射
func Disconnect(apiKey string, localPort int, jsonOutput bool) error {
	reqBody := map[string]interface{}{
		"key":       apiKey,
		"localPort": localPort,
	}

	jsonData, _ := json.Marshal(reqBody)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(LocalServerURL+"/api/disconnect", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	if !result.Success {
		return fmt.Errorf("断开失败: %s", result.Message)
	}

	if jsonOutput {
		OutputJSON(map[string]interface{}{
			"success":   true,
			"localPort": localPort,
		})
	} else {
		fmt.Printf("✓ 已断开本地端口 %d 的映射\n", localPort)
	}
	return nil
}

// EnableAdbWifi 开启 ADB WiFi 调试（完整流程）
// 1. 映射 9009 端口（RPA 通信）
// 2. 通过 execShell 执行开启 ADB WiFi 的命令
// 3. 映射 5555 端口（ADB 端口）
// 4. 完成后可用 adb connect 127.0.0.1:5555
func EnableAdbWifi(platformURL, apiKey string, deviceID int, jsonOutput bool) error {
	log := func(msg string) {
		if !jsonOutput {
			fmt.Println(msg)
		}
	}

	// 先登录
	if err := EnsureLogin(platformURL, apiKey); err != nil {
		return fmt.Errorf("登录失败: %v", err)
	}

	// 1. 映射 9009 端口（用于后续可能的 RPA 操作）
	log("步骤 1/4: 映射 9009 端口...")
	reqBody := map[string]interface{}{
		"key":       apiKey,
		"deviceId":  deviceID,
		"localPort": 9009,
		"phonePort": 9009,
	}
	jsonData, _ := json.Marshal(reqBody)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(LocalServerURL+"/api/connect", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("映射 9009 失败: %v", err)
	}
	resp.Body.Close()

	// 2. 通过 execShell 执行开启 ADB WiFi 的命令
	log("步骤 2/4: 执行 root 提权...")
	// 先尝试 root 提权（可能失败，忽略）
	CallUnified(platformURL, apiKey, "execShell", map[string]interface{}{
		"deviceId": deviceID,
		"shell":    "su -c 'echo root granted'",
	})
	time.Sleep(500 * time.Millisecond)

	log("步骤 3/4: 开启 ADB WiFi 调试...")
	// 执行开启 ADB WiFi 的 shell 命令
	adbCmd := `su -c "setprop service.adb.tcp.port 5555 && stop adbd && start adbd && settings put global adb_enabled 1"`
	result, err := CallUnified(platformURL, apiKey, "execShell", map[string]interface{}{
		"deviceId": deviceID,
		"shell":    adbCmd,
	})
	if err != nil {
		return fmt.Errorf("开启 ADB WiFi 失败: %v", err)
	}
	if result.Code != 200 {
		return fmt.Errorf("开启 ADB WiFi 失败: %s", result.Msg)
	}

	// 等待 ADB WiFi 启动
	log("  等待 ADB 服务启动...")
	time.Sleep(3 * time.Second)

	// 4. 映射 5555 端口
	log("步骤 4/4: 映射 5555 端口...")
	reqBody["localPort"] = 5555
	reqBody["phonePort"] = 5555
	jsonData, _ = json.Marshal(reqBody)
	resp, err = client.Post(LocalServerURL+"/api/connect", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("映射 5555 失败: %v", err)
	}
	resp.Body.Close()

	// 完成
	if jsonOutput {
		OutputJSON(map[string]interface{}{
			"success":  true,
			"deviceId": deviceID,
			"adbHost":  "127.0.0.1:5555",
			"message":  "ADB WiFi 已开启，使用 adb connect 127.0.0.1:5555 连接",
		})
	} else {
		fmt.Println("\n✓ ADB WiFi 调试已开启！")
		fmt.Println("  连接命令: adb connect 127.0.0.1:5555")
	}

	return nil
}

// DisableAdbWifi 关闭 ADB WiFi 调试并断开隧道
func DisableAdbWifi(platformURL, apiKey string, deviceID int, jsonOutput bool) error {
	log := func(msg string) {
		if !jsonOutput {
			fmt.Println(msg)
		}
	}

	// 如果提供了设备ID，尝试关闭 ADB WiFi
	if deviceID > 0 {
		log("关闭 ADB WiFi...")
		// 先登录
		if err := EnsureLogin(platformURL, apiKey); err == nil {
			// 执行关闭 ADB WiFi 的 shell 命令
			adbCmd := `su -c "setprop service.adb.tcp.port -1 && stop adbd && settings put global adb_enabled 0"`
			CallUnified(platformURL, apiKey, "execShell", map[string]interface{}{
				"deviceId": deviceID,
				"shell":    adbCmd,
			})
		}
	}

	// 断开 9009 和 5555 端口映射
	log("断开端口映射...")
	client := &http.Client{Timeout: 10 * time.Second}

	for _, port := range []int{9009, 5555} {
		reqBody := map[string]interface{}{
			"key":       apiKey,
			"localPort": port,
		}
		jsonData, _ := json.Marshal(reqBody)
		resp, _ := client.Post(LocalServerURL+"/api/disconnect", "application/json", bytes.NewReader(jsonData))
		if resp != nil {
			resp.Body.Close()
		}
	}

	if jsonOutput {
		OutputJSON(map[string]interface{}{
			"success": true,
			"message": "ADB WiFi 已关闭",
		})
	} else {
		fmt.Println("✓ ADB WiFi 调试已关闭")
	}

	return nil
}

// ExecuteShell 执行 Shell 命令
func ExecuteShell(platformURL, apiKey, deviceID, command string, jsonOutput bool) error {
	// 先登录
	if err := EnsureLogin(platformURL, apiKey); err != nil {
		return fmt.Errorf("登录失败: %v", err)
	}

	// 解析设备ID
	var devID int
	fmt.Sscanf(deviceID, "%d", &devID)

	// 执行 Shell
	result, err := CallUnified(platformURL, apiKey, "execShell", map[string]interface{}{
		"deviceId": devID,
		"shell":    command,
	})
	if err != nil {
		return err
	}

	// 输出结果
	if jsonOutput {
		OutputJSON(result.Data)
	} else if result.Data != nil {
		data, _ := json.MarshalIndent(result.Data, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Println("执行成功")
	}

	return nil
}

// Screenshot 截图（通过集控平台）
func Screenshot(platformURL, apiKey, deviceID, output string) error {
	// 先登录
	if err := EnsureLogin(platformURL, apiKey); err != nil {
		return fmt.Errorf("登录失败: %v", err)
	}

	// 解析设备ID
	var devID int
	fmt.Sscanf(deviceID, "%d", &devID)

	// 请求截图
	result, err := CallUnified(platformURL, apiKey, "screenshot", map[string]interface{}{
		"deviceId": devID,
	})
	if err != nil {
		return err
	}

	// 检查结果
	if result.Data != nil {
		if output == "" {
			output = fmt.Sprintf("screenshot_%s.png", time.Now().Format("20060102_150405"))
		}
		// TODO: 保存截图数据到文件
		fmt.Printf("截图请求已发送，设备ID: %d\n", devID)
	} else {
		return fmt.Errorf("截图失败")
	}

	return nil
}

// GetInstallPath 获取安装路径
func GetInstallPath() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "jpy-cloud", "jpy-cloud.exe")
	default:
		return "/usr/local/bin/jpy-cloud"
	}
}

// GetDataDir 获取数据目录
func GetDataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".jpy-cloud")
}

// CheckServiceAndGuide 检查本地服务是否运行
func CheckServiceAndGuide() bool {
	resp, err := http.Get("http://127.0.0.1:1001/health")
	if err != nil {
		fmt.Println("错误: 本地服务未运行")
		fmt.Println("请先在另一个终端运行: jpy-cloud serve")
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// TailFollow 实时查看日志（类似 tail -f）
func TailFollow(logPath string) {
	if runtime.GOOS == "windows" {
		// Windows 使用 PowerShell 的 Get-Content -Wait
		cmd := exec.Command("powershell", "-Command", fmt.Sprintf("Get-Content -Path '%s' -Wait -Tail 50", logPath))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	} else {
		cmd := exec.Command("tail", "-f", logPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	}
}

// TailLines 查看最近 N 行日志
func TailLines(logPath string, lines int) {
	if runtime.GOOS == "windows" {
		// Windows 使用 PowerShell 的 Get-Content -Tail
		cmd := exec.Command("powershell", "-Command", fmt.Sprintf("Get-Content -Path '%s' -Tail %d", logPath, lines))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	} else {
		cmd := exec.Command("tail", "-n", fmt.Sprintf("%d", lines), logPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	}
}
