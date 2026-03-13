package steps

import (
	"context"
	"fmt"
	"port-mapping-demo/internal/database"
	"port-mapping-demo/internal/rpa"
	"port-mapping-demo/internal/service"
	"port-mapping-demo/pkg/logger"
	"time"
)

// ShellStep 执行 Shell 命令步骤
type ShellStep struct{}

func init() {
	rpa.RegisterStep(&ShellStep{})
}

func (s *ShellStep) Type() string {
	return "shell"
}

func (s *ShellStep) Name() string {
	return "Shell命令"
}

func (s *ShellStep) SubSteps() []string {
	return []string{"执行命令"}
}

func (s *ShellStep) Execute(deviceID int, params map[string]interface{}, subStep int, ctx database.StepContext) rpa.StepResult {
	command, _ := params["command"].(string)
	if command == "" {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "缺少必要参数: command",
		}
	}

	req := &service.UnifiedRequest{
		Type: "execShell",
		Seq:  int(time.Now().Unix()),
		Data: map[string]interface{}{
			"deviceId": deviceID,
			"shell":    command,
		},
	}

	res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
	if err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("执行Shell失败: %v", err),
		}
	}

	if res.Code != 200 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("执行Shell失败: %s", res.Msg),
		}
	}

	logger.LogInfo("[RPA] 设备 %d 执行Shell: %s", deviceID, command)

	return rpa.StepResult{
		Completed: true,
		Success:   true,
		Output: map[string]interface{}{
			"result": res.Data,
		},
	}
}
