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
// 改机重启后检测设备网络是否可达目标地址
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
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "缺少必要参数: targetUrl（目标地址）",
		}
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

	// 从 URL 中提取 host:port 用于检测
	// 支持 ws://host:port/path 和 http://host:port/path 格式
	host := extractHost(targetUrl)
	if host == "" {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("无法从 URL 中提取主机地址: %s", targetUrl),
		}
	}

	logger.LogInfo("[RPA] 设备 %d 开始网络检测, 目标: %s (host=%s), 最多重试 %d 次", deviceID, targetUrl, host, maxRetries)

	for i := 1; i <= maxRetries; i++ {
		// 用 nc (netcat) 检测端口连通性，超时 3 秒
		// 如果没有 nc，退回用 ping 检测
		shellCmd := fmt.Sprintf("nc -z -w 3 %s && echo NETWORK_OK || echo NETWORK_FAIL", host)

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

			if res.Code == 200 && strings.Contains(dataStr, "NETWORK_OK") {
				logger.LogInfo("[RPA] 设备 %d 网络检测通过（第%d次）, 目标: %s", deviceID, i, host)
				return rpa.StepResult{
					Completed: true,
					Success:   true,
					Output: map[string]interface{}{
						"targetUrl":  targetUrl,
						"host":       host,
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
		Error:     fmt.Sprintf("网络检测超时: 设备无法连接 %s（重试 %d 次）", host, maxRetries),
	}
}

// extractHost 从 URL 中提取 host port 部分，返回 "host port" 格式（nc 命令用空格分隔）
func extractHost(rawUrl string) string {
	// 去掉协议前缀
	u := rawUrl
	for _, prefix := range []string{"ws://", "wss://", "http://", "https://"} {
		if strings.HasPrefix(u, prefix) {
			u = u[len(prefix):]
			break
		}
	}
	// 去掉路径
	if idx := strings.Index(u, "/"); idx >= 0 {
		u = u[:idx]
	}
	// 分离 host 和 port
	if idx := strings.LastIndex(u, ":"); idx >= 0 {
		host := u[:idx]
		port := u[idx+1:]
		return host + " " + port
	}
	// 没有端口，返回空（nc 需要端口）
	return ""
}
