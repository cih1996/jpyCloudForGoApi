package steps

import (
	"fmt"
	"port-mapping-demo/internal/database"
	"port-mapping-demo/internal/rpa"
	"port-mapping-demo/pkg/logger"
	"time"
)

// RunScriptAndWaitStep 运行脚本步骤
type RunScriptAndWaitStep struct{}

func init() {
	rpa.RegisterStep(&RunScriptAndWaitStep{})
}

func (s *RunScriptAndWaitStep) Type() string {
	return "run_script_and_wait"
}

func (s *RunScriptAndWaitStep) Name() string {
	return "运行脚本"
}

func (s *RunScriptAndWaitStep) SubSteps() []string {
	return []string{"启动脚本", "等待脚本完成"}
}

func (s *RunScriptAndWaitStep) Execute(deviceID int, params map[string]interface{}, subStep int, ctx database.StepContext) rpa.StepResult {
	switch subStep {
	case 0:
		return s.startScript(deviceID, params, ctx)
	case 1:
		return s.waitComplete(deviceID, ctx)
	default:
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("未知子步骤: %d", subStep),
		}
	}
}

func (s *RunScriptAndWaitStep) startScript(deviceID int, params map[string]interface{}, ctx database.StepContext) rpa.StepResult {
	scriptName, _ := params["scriptName"].(string)
	scriptUrl, _ := params["scriptUrl"].(string)
	scriptId, _ := params["scriptId"].(float64)
	scriptParams, _ := params["scriptParams"].(string)

	if scriptName == "" && scriptUrl == "" && scriptId == 0 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "缺少必要参数: scriptName、scriptUrl 或 scriptId",
		}
	}

	// 这里需要通过某种方式启动脚本
	// 可以是调用设备上的脚本引擎，或者通过 RPA SDK 启动
	logger.LogInfo("[RPA] 设备 %d 启动脚本: %s (URL: %s, ID: %.0f)", deviceID, scriptName, scriptUrl, scriptId)

	// 更新设备的脚本状态
	database.UpdateScriptFeedback(deviceID, "running", 0)

	newCtx := make(database.StepContext)
	newCtx["scriptName"] = scriptName
	newCtx["scriptUrl"] = scriptUrl
	newCtx["scriptId"] = scriptId
	newCtx["scriptParams"] = scriptParams
	newCtx["startTime"] = time.Now().Unix()

	return rpa.StepResult{
		Completed: false,
		NextSub:   1,
		Context:   newCtx,
	}
}

func (s *RunScriptAndWaitStep) waitComplete(deviceID int, ctx database.StepContext) rpa.StepResult {
	startTime, _ := ctx["startTime"].(float64)
	timeout := 3600 // 默认 1 小时超时
	if t, ok := ctx["timeout"].(float64); ok {
		timeout = int(t)
	}

	if time.Now().Unix()-int64(startTime) > int64(timeout) {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("脚本执行超时（%d秒）", timeout),
		}
	}

	// 检查设备的脚本状态
	config, err := database.GetDeviceConfig(deviceID)
	if err != nil {
		return rpa.StepResult{
			Completed: false,
			NextSub:   1,
			Context:   ctx,
		}
	}

	// 根据脚本反馈状态判断
	switch config.ScriptStatus {
	case "completed", "success":
		logger.LogInfo("[RPA] 设备 %d 脚本执行完成", deviceID)
		return rpa.StepResult{
			Completed: true,
			Success:   true,
			Output: map[string]interface{}{
				"progress": config.ScriptProgress,
			},
		}
	case "error", "failed":
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "脚本执行失败",
		}
	default:
		// 继续等待
		return rpa.StepResult{
			Completed: false,
			NextSub:   1,
			Context:   ctx,
		}
	}
}
