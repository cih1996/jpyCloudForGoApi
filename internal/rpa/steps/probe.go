package steps

import (
	"context"
	"fmt"
	"port-mapping-demo/internal/service"
	"port-mapping-demo/pkg/logger"
	"strings"
	"time"
)

// ProbeShellChannel 探测设备 shell 通道是否真正可用（公共函数）
// 改机重启后中间件到设备的通道可能还没恢复，命令返回200但设备不执行
// 通过发送 echo PROBE_OK 并检查返回值来确认通道可用
//
// maxProbes: 最大探测次数，0 表示使用默认值40
// interval: 探测间隔秒数，0 表示使用默认值3
func ProbeShellChannel(deviceID int, maxProbes int, interval int) bool {
	if maxProbes <= 0 {
		maxProbes = 40
	}
	if interval <= 0 {
		interval = 3
	}

	logger.LogInfo("[RPA] 设备 %d 开始探测shell通道（最多%d次，间隔%d秒）", deviceID, maxProbes, interval)

	for i := 1; i <= maxProbes; i++ {
		req := &service.UnifiedRequest{
			Type: "execShell",
			Data: map[string]interface{}{
				"deviceId": float64(deviceID),
				"shell":    "echo PROBE_OK",
			},
		}

		res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
		if err != nil {
			logger.LogInfo("[RPA] 设备 %d shell探测 %d/%d 失败: %v", deviceID, i, maxProbes, err)
		} else {
			dataStr := fmt.Sprintf("%v", res.Data)
			logger.LogInfo("[RPA] 设备 %d shell探测 %d/%d: code=%d data=[%s]", deviceID, i, maxProbes, res.Code, dataStr)
			if strings.Contains(dataStr, "PROBE_OK") {
				logger.LogInfo("[RPA] 设备 %d shell通道已就绪（第%d次探测）", deviceID, i)
				return true
			}
		}

		if i < maxProbes {
			time.Sleep(time.Duration(interval) * time.Second)
		}
	}

	logger.LogInfo("[RPA] 设备 %d shell通道探测超时（%d次均未响应）", deviceID, maxProbes)
	return false
}
