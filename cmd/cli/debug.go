package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// DebugResult 调试执行结果
type DebugResult struct {
	DebugID   string      `json:"debugId"`
	Success   bool        `json:"success"`
	Result    interface{} `json:"result,omitempty"`
	Error     string      `json:"error,omitempty"`
	Logs      []string    `json:"logs,omitempty"`
	Duration  int64       `json:"duration"`
	Timestamp int64       `json:"timestamp"`
}

// DebugCommand 处理 debug 命令
func DebugCommand(args []string) {
	if len(args) == 0 {
		printDebugUsage()
		return
	}

	// 第一个参数是设备ID
	deviceIDStr := args[0]
	if deviceIDStr == "--help" || deviceIDStr == "-h" {
		printDebugUsage()
		return
	}

	deviceID, err := strconv.ParseUint(deviceIDStr, 10, 32)
	if err != nil {
		// 尝试十六进制
		deviceID, err = strconv.ParseUint(deviceIDStr, 16, 32)
		if err != nil {
			fmt.Printf("无效的设备ID: %s\n", deviceIDStr)
			return
		}
	}

	// 解析参数
	var code string
	var timeout int64 = 30000
	jsonOutput := false

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--code":
			if i+1 < len(args) {
				code = args[i+1]
				i++
			}
		case "--code-file":
			if i+1 < len(args) {
				data, err := os.ReadFile(args[i+1])
				if err != nil {
					fmt.Printf("读取文件失败: %v\n", err)
					return
				}
				code = string(data)
				i++
			}
		case "--timeout":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &timeout)
				i++
			}
		case "--json":
			jsonOutput = true
		}
	}

	if code == "" {
		fmt.Println("错误: --code 或 --code-file 必填")
		printDebugUsage()
		return
	}

	// 先查 devicews 在线设备列表，校验设备ID
	resolvedDeviceID := uint32(deviceID)
	client := &http.Client{Timeout: time.Duration(timeout+5000) * time.Millisecond}

	devListResp, err := client.Get(LocalServerURL + "/api/devicews/devices")
	if err != nil {
		fmt.Printf("查询设备列表失败（本地服务是否已启动？）: %v\n", err)
		return
	}
	devListBody, _ := io.ReadAll(devListResp.Body)
	devListResp.Body.Close()

	var devListResult struct {
		Code int                      `json:"code"`
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(devListBody, &devListResult); err == nil && devListResult.Code == 200 {
		// 检查用户给的 ID 是否匹配某个在线设备的 CRC32 deviceId
		found := false
		for _, dev := range devListResult.Data {
			if did, ok := dev["deviceId"].(float64); ok && uint32(did) == resolvedDeviceID {
				found = true
				break
			}
		}
		if !found && len(devListResult.Data) > 0 {
			// 用户可能传了云平台 deviceId，提示正确的 CRC32 ID
			fmt.Printf("设备 %s 未在 devicews 中找到（你可能用了云平台 deviceId）\n", deviceIDStr)
			fmt.Println("当前在线设备:")
			fmt.Printf("  %-12s %-18s %-10s\n", "CRC32 ID", "序列号", "型号")
			for _, dev := range devListResult.Data {
				did := uint32(0)
				if v, ok := dev["deviceId"].(float64); ok {
					did = uint32(v)
				}
				sn, _ := dev["serialno"].(string)
				brand, _ := dev["brand"].(string)
				fmt.Printf("  %-12d %-18s %-10s\n", did, sn, brand)
			}
			fmt.Println("\n用法: jpy-cloud debug <CRC32 ID> --code \"...\"")
			return
		}
		if !found && len(devListResult.Data) == 0 {
			fmt.Println("当前无在线设备")
			return
		}
	}

	// 生成 debugId
	debugID := uuid.New().String()[:8]

	// 发送调试执行请求
	sendURL := fmt.Sprintf("%s/api/devicews/devices/%d/debug", LocalServerURL, resolvedDeviceID)
	body := map[string]interface{}{
		"debugId": debugID,
		"code":    code,
		"timeout": timeout,
	}

	jsonData, _ := json.Marshal(body)
	resp, err := client.Post(sendURL, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		fmt.Printf("发送失败（本地服务是否已启动？）: %v\n", err)
		return
	}
	resp.Body.Close()

	if !jsonOutput {
		fmt.Printf("已发送到设备 %d，等待结果...\n", resolvedDeviceID)
	}

	// 轮询等待结果
	resultURL := fmt.Sprintf("%s/api/devicews/devices/%d/debug/%s", LocalServerURL, resolvedDeviceID, debugID)
	deadline := time.Now().Add(time.Duration(timeout) * time.Millisecond)

	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)

		resp, err := client.Get(resultURL)
		if err != nil {
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var apiResp struct {
			Code int             `json:"code"`
			Msg  string          `json:"msg"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(respBody, &apiResp); err != nil {
			continue
		}

		// 404 = 结果还没回来
		if apiResp.Code == 404 {
			continue
		}

		if apiResp.Code != 200 || len(apiResp.Data) == 0 {
			continue
		}

		var result DebugResult
		if err := json.Unmarshal(apiResp.Data, &result); err != nil {
			continue
		}

		// 拿到结果了
		if jsonOutput {
			OutputJSON(result)
		} else {
			if result.Success {
				fmt.Printf("✓ 执行成功 (%dms)\n", result.Duration)
			} else {
				fmt.Printf("✗ 执行失败 (%dms)\n", result.Duration)
			}
			if result.Result != nil {
				resultJSON, _ := json.MarshalIndent(result.Result, "", "  ")
				fmt.Printf("结果: %s\n", string(resultJSON))
			}
			if result.Error != "" {
				fmt.Printf("错误: %s\n", result.Error)
			}
			if len(result.Logs) > 0 {
				fmt.Println("日志:")
				for _, l := range result.Logs {
					fmt.Printf("  %s\n", l)
				}
			}
		}
		return
	}

	// 超时
	if jsonOutput {
		OutputJSON(map[string]interface{}{
			"success": false,
			"error":   "等待结果超时",
			"debugId": debugID,
		})
	} else {
		fmt.Printf("等待结果超时（%dms），debugId: %s\n", timeout, debugID)
	}
}

func printDebugUsage() {
	fmt.Println(`
临时执行脚本（调试模式）

用法:
  jpy-cloud debug <设备ID> --code <代码> [--code-file <文件>] [--timeout 30000] [--json]

参数:
  设备ID       设备的 CRC32 ID（十进制或十六进制）

选项:
  --code       直接传入代码内容
  --code-file  从文件读取代码内容（与 --code 二选一）
  --timeout    超时时间（毫秒，默认 30000）
  --json       JSON 格式输出

示例:
  jpy-cloud debug 1770329527 --code "return 1+1"
  jpy-cloud debug 698515B7 --code "return screenshot()" --timeout 10000
  jpy-cloud debug 1770329527 --code-file ./test.js --json`)
}
