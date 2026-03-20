package steps

import (
	"context"
	"fmt"
	"port-mapping-demo/internal/database"
	"port-mapping-demo/internal/rpa"
	"port-mapping-demo/internal/service"
	"port-mapping-demo/pkg/logger"
	"strings"
	"time"
)

// NetworkCheckStep 网络检测步骤
// 改机重启后检测设备是否能访问目标网址（HTTP 可达性）
type NetworkCheckStep struct{}

func init() {
	rpa.RegisterStep(&NetworkCheckStep{})
}

func (s *NetworkCheckStep) Type() string {
	return "network_check"
}

func (s *NetworkCheckStep) Name() string {
	return "网络检测"
}

func (s *NetworkCheckStep) SubSteps() []string {
	return []string{"检测网络"}
}

func (s *NetworkCheckStep) Execute(deviceID int, params map[string]interface{}, subStep int, ctx database.StepContext) rpa.StepResult {
	targetUrl, _ := params["targetUrl"].(string)
	if targetUrl == "" {
		targetUrl = "https://www.baidu.com"
	}

	// 自动补协议
	if !strings.HasPrefix(targetUrl, "http://") && !strings.HasPrefix(targetUrl, "https://") {
		targetUrl = "https://" + targetUrl
	}

	// 重试参数
	maxRetries := 20
	if mr, ok := params["maxRetries"].(float64); ok && mr > 0 {
		maxRetries = int(mr)
	}
	retryInterval := 3
	if ri, ok := params["retryInterval"].(float64); ok && ri > 0 {
		retryInterval = int(ri)
	}

	logger.LogInfo("[RPA] 设备 %d 开始网络检测, 目标: %s, 最多重试 %d 次", deviceID, targetUrl, maxRetries)

	for i := 1; i <= maxRetries; i++ {
		// 用 curl 检测 HTTP 可达性，超时 5 秒
		// 返回 HTTP 状态码，2xx/3xx 算成功
		shellCmd := fmt.Sprintf(
			`curl -s -o /dev/null -w "%%{http_code}" --connect-timeout 5 --max-time 10 "%s"`,
			targetUrl,
		)

		req := &service.UnifiedRequest{
			Type: "execShell",
			Seq:  int(time.Now().UnixMilli()),
			Data: map[string]interface{}{
				"deviceId": float64(deviceID),
				"shell":    shellCmd,
			},
		}

		res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
		if err != nil {
			logger.LogInfo("[RPA] 设备 %d 网络检测 %d/%d shell失败: %v", deviceID, i, maxRetries, err)
		} else {
			dataStr := fmt.Sprintf("%v", res.Data)
			logger.LogInfo("[RPA] 设备 %d 网络检测 %d/%d: code=%d data=[%s]", deviceID, i, maxRetries, res.Code, dataStr)

			// curl 返回的 HTTP 状态码在 data 中，检查是否为 2xx 或 3xx
			if res.Code == 200 && isHTTPSuccess(dataStr) {
				logger.LogInfo("[RPA] 设备 %d 网络检测通过（第%d次）, 目标: %s", deviceID, i, targetUrl)
				return rpa.StepResult{
					Completed: true,
					Success:   true,
					Output: map[string]interface{}{
						"targetUrl":  targetUrl,
						"httpCode":   strings.TrimSpace(dataStr),
						"retryCount": i,
					},
				}
			}
		}

		if i < maxRetries {
			time.Sleep(time.Duration(retryInterval) * time.Second)
		}
	}

	return rpa.StepResult{
		Completed: true,
		Success:   false,
		Error:     fmt.Sprintf("网络检测超时: 设备无法访问 %s（重试 %d 次）", targetUrl, maxRetries),
	}
}

// isHTTPSuccess 检查 curl 返回的数据中是否包含 2xx 或 3xx 状态码
func isHTTPSuccess(dataStr string) bool {
	// curl -w "%{http_code}" 返回的是纯数字如 "200"、"301" 等
	// 但经过中间件返回后可能包含其他内容，所以用 Contains 匹配
	successCodes := []string{"200", "201", "202", "204", "301", "302", "303", "304", "307", "308"}
	for _, code := range successCodes {
		if strings.Contains(dataStr, code) {
			return true
		}
	}
	return false
}
