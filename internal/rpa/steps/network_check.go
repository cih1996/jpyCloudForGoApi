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
// 通过 ping IP 地址检测设备是否有互联网连接
// 注意：Android 设备 DNS 可能未配置，ping 域名会卡死 shell 通道，因此统一 ping IP
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
	// 检测目标 IP 列表（避免 ping 域名导致 DNS 卡死 shell 通道）
	// 用户配置的 targetUrl 仅作为日志展示，实际 ping 固定 IP
	targetUrl, _ := params["targetUrl"].(string)
	if targetUrl == "" {
		targetUrl = "114.114.114.114"
	}

	// ping 目标：8.8.8.8 最通用（云手机网络通常放行），其次国内 DNS
	// 注意：不可达的 IP 会导致 ping 超时且吞掉 shell stdout，所以只要有一个通就算成功
	pingTargets := []string{"8.8.8.8", "114.114.114.114", "223.5.5.5"}
	host := extractDomain(targetUrl)
	if host != "" && isIPAddress(host) {
		// 用户指定了 IP，放到第一位
		pingTargets = []string{host, "8.8.8.8", "114.114.114.114"}
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

	logger.LogInfo("[RPA] 设备 %d 开始网络检测, 目标: %s, ping: %v, 最多重试 %d 次", deviceID, targetUrl, pingTargets, maxRetries)

	for i := 1; i <= maxRetries; i++ {
		timeout := retryInterval
		if timeout < 2 {
			timeout = 2
		}

		// 依次尝试 ping 每个 IP
		for _, target := range pingTargets {
			shellCmd := fmt.Sprintf(`ping -c 1 -w %d %s`, timeout, target)
			res, err := s.execShell(deviceID, shellCmd)
			if err != nil {
				logger.LogInfo("[RPA] 设备 %d 网络检测 %d/%d ping %s 失败: %v", deviceID, i, maxRetries, target, err)
				continue
			}

			dataStr := fmt.Sprintf("%v", res.Data)
			logger.LogInfo("[RPA] 设备 %d 网络检测 %d/%d ping %s: code=%d data_len=%d", deviceID, i, maxRetries, target, res.Code, len(dataStr))

			// ping 成功的标志：返回了 "bytes from" 或 "0% packet loss"
			if res.Code == 200 && (strings.Contains(dataStr, "bytes from") || strings.Contains(dataStr, "0% packet loss")) {
				logger.LogInfo("[RPA] 设备 %d 网络检测通过（第%d次）, ping %s 成功", deviceID, i, target)
				return rpa.StepResult{
					Completed: true,
					Success:   true,
					Output: map[string]interface{}{
						"target":     targetUrl,
						"pingTarget": target,
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
		Error:     fmt.Sprintf("网络检测超时: 设备无法连接互联网（重试 %d 次）", maxRetries),
	}
}

// execShell 执行 shell 命令的辅助方法
func (s *NetworkCheckStep) execShell(deviceID int, shell string) (*service.UnifiedResponse, error) {
	req := &service.UnifiedRequest{
		Type: "execShell",
		Data: map[string]interface{}{
			"deviceId": float64(deviceID),
			"shell":    shell,
		},
	}
	return service.HandleUnifiedRequestHTTP(context.Background(), req)
}

// isIPAddress 判断字符串是否为 IP 地址
func isIPAddress(s string) bool {
	for _, c := range s {
		if c != '.' && (c < '0' || c > '9') {
			return false
		}
	}
	// 至少有一个点且不以点开头结尾
	return strings.Contains(s, ".") && s[0] != '.' && s[len(s)-1] != '.'
}

// extractDomain 从 URL 或域名中提取纯域名/IP
// 支持: www.baidu.com, https://www.baidu.com/path, http://1.2.3.4:8080
func extractDomain(raw string) string {
	s := raw
	// 去协议
	for _, prefix := range []string{"https://", "http://", "ws://", "wss://"} {
		if strings.HasPrefix(s, prefix) {
			s = s[len(prefix):]
			break
		}
	}
	// 去路径
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}
	// 去端口
	if idx := strings.LastIndex(s, ":"); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}
