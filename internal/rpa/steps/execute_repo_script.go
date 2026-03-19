package steps

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"port-mapping-demo/internal/database"
	"port-mapping-demo/internal/devicews"
	"port-mapping-demo/internal/rpa"
	"port-mapping-demo/internal/service"
	"port-mapping-demo/pkg/logger"
	"time"
)

// ExecuteRepoScriptStep 从代码仓库执行脚本步骤
// 通过脚本ID从仓库获取代码，然后推送到设备执行
type ExecuteRepoScriptStep struct{}

func init() {
	rpa.RegisterStep(&ExecuteRepoScriptStep{})
}

func (s *ExecuteRepoScriptStep) Type() string {
	return "execute_repo_script"
}

func (s *ExecuteRepoScriptStep) Name() string {
	return "执行仓库脚本"
}

func (s *ExecuteRepoScriptStep) SubSteps() []string {
	return []string{"加载脚本", "推送脚本", "等待执行结果"}
}

func (s *ExecuteRepoScriptStep) Execute(deviceID int, params map[string]interface{}, subStep int, ctx database.StepContext) rpa.StepResult {
	switch subStep {
	case 0:
		return s.loadScript(deviceID, params, ctx)
	case 1:
		return s.pushScript(deviceID, params, ctx)
	case 2:
		return s.waitResult(deviceID, params, ctx)
	default:
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("未知子步骤: %d", subStep),
		}
	}
}

// loadScript 从仓库加载脚本
func (s *ExecuteRepoScriptStep) loadScript(deviceID int, params map[string]interface{}, ctx database.StepContext) rpa.StepResult {
	// 获取脚本ID
	scriptID, ok := params["scriptId"].(float64)
	if !ok || scriptID <= 0 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "缺少必要参数: scriptId（脚本ID）",
		}
	}

	// 从数据库获取脚本
	script, err := database.GetScript(uint(scriptID))
	if err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("获取脚本失败: %v", err),
		}
	}
	if script == nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("脚本不存在: ID=%d", int(scriptID)),
		}
	}

	// 获取设备配置以读取流程变量
	config, _ := database.GetDeviceConfig(deviceID)
	var flowVars map[string]interface{}
	if config != nil && config.FlowVariables != nil {
		flowVars = config.FlowVariables
	}

	logger.LogInfo("[RPA] 设备 %d 加载仓库脚本: %s (ID=%d), 流程变量: %d 个", deviceID, script.Name, script.ID, len(flowVars))

	// 保存脚本信息到上下文
	newCtx := make(database.StepContext)
	newCtx["code"] = script.Code
	newCtx["scriptName"] = script.Name
	newCtx["timeout"] = script.Timeout
	// 保存流程变量到上下文，供推送时使用
	if flowVars != nil {
		newCtx["flowVariables"] = flowVars
	}

	return rpa.StepResult{
		Completed: false,
		NextSub:   1,
		Context:   newCtx,
	}
}

// pushScript 推送脚本到设备
func (s *ExecuteRepoScriptStep) pushScript(deviceID int, params map[string]interface{}, ctx database.StepContext) rpa.StepResult {
	// 从上下文获取脚本代码
	code, _ := ctx["code"].(string)
	if code == "" {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "脚本代码为空",
		}
	}

	scriptName, _ := ctx["scriptName"].(string)
	if scriptName == "" {
		scriptName = "仓库脚本任务"
	}

	// 获取流程变量
	var flowVars map[string]interface{}
	if fv, ok := ctx["flowVariables"].(map[string]interface{}); ok {
		flowVars = fv
	} else if fv, ok := ctx["flowVariables"].(database.StepContext); ok {
		flowVars = fv
	}

	// 获取超时时间（优先使用参数中的，其次使用脚本配置的）
	timeout := int64(60000)
	if t, ok := params["timeout"].(float64); ok && t > 0 {
		timeout = int64(t)
	} else if t, ok := ctx["timeout"].(float64); ok && t > 0 {
		timeout = int64(t)
	} else if t, ok := ctx["timeout"].(int); ok && t > 0 {
		timeout = int64(t)
	}

	// 获取 WebSocket 服务器
	wsServer := service.GetDeviceWSServer()
	if wsServer == nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "WebSocket 服务器未初始化",
		}
	}

	// 获取设备的 UUID
	deviceUUID := service.GetDeviceUUID(deviceID)
	if deviceUUID == "" {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "无法获取设备 UUID",
		}
	}

	// 查找设备连接
	dc, ok := wsServer.GetManager().GetBySerialNo(deviceUUID)
	if !ok {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "设备未通过 WebSocket 连接",
		}
	}

	// 生成任务 ID 和脚本 hash
	taskID := fmt.Sprintf("repo_%d_%d", deviceID, time.Now().UnixNano())
	h := sha256.New()
	h.Write([]byte(code))
	scriptHash := hex.EncodeToString(h.Sum(nil))

	// 构建推送负载（包含流程变量）
	payload := devicews.TaskPushPayload{
		TaskID:     taskID,
		TaskName:   scriptName,
		ScriptHash: scriptHash,
		Code:       code,
		Timeout:    timeout,
		Priority:   1,
		Variables:  flowVars, // 注入流程变量
	}

	// 发送任务推送
	if err := dc.SendTaskPush(&payload); err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("推送脚本失败: %v", err),
		}
	}

	logger.LogInfo("[RPA] 设备 %d 仓库脚本已推送，任务ID: %s, 脚本: %s, 变量: %d 个", deviceID, taskID, scriptName, len(flowVars))

	// 保存上下文，进入等待结果步骤
	newCtx := make(database.StepContext)
	newCtx["taskId"] = taskID
	newCtx["startTime"] = time.Now().Unix()
	newCtx["timeout"] = timeout
	newCtx["deviceUUID"] = deviceUUID
	newCtx["scriptName"] = scriptName

	return rpa.StepResult{
		Completed: false,
		NextSub:   2,
		Context:   newCtx,
	}
}

// waitResult 等待脚本执行结果
func (s *ExecuteRepoScriptStep) waitResult(deviceID int, params map[string]interface{}, ctx database.StepContext) rpa.StepResult {
	taskID, _ := ctx["taskId"].(string)
	startTime, _ := ctx["startTime"].(float64)
	timeout, _ := ctx["timeout"].(float64)
	deviceUUID, _ := ctx["deviceUUID"].(string)
	scriptName, _ := ctx["scriptName"].(string)

	// 检查超时
	elapsed := time.Now().Unix() - int64(startTime)
	if elapsed*1000 > int64(timeout) {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("脚本执行超时（%d 秒）", elapsed),
		}
	}

	// 获取 WebSocket 服务器
	wsServer := service.GetDeviceWSServer()
	if wsServer == nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "WebSocket 服务器未初始化",
		}
	}

	// 查找设备连接
	dc, ok := wsServer.GetManager().GetBySerialNo(deviceUUID)
	if !ok {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "设备 WebSocket 连接已断开",
		}
	}

	// 检查任务结果
	result, hasResult := dc.GetTaskResult(taskID)
	if hasResult {
		if result.Success {
			logger.LogInfo("[RPA] 设备 %d 仓库脚本执行成功，脚本: %s", deviceID, scriptName)
			return rpa.StepResult{
				Completed: true,
				Success:   true,
				Output: map[string]interface{}{
					"taskId":     taskID,
					"scriptName": scriptName,
					"result":     result.Result,
					"duration":   result.Duration,
				},
			}
		} else {
			return rpa.StepResult{
				Completed: true,
				Success:   false,
				Error:     fmt.Sprintf("脚本执行失败: %s", result.Error),
				Output: map[string]interface{}{
					"taskId":     taskID,
					"scriptName": scriptName,
					"error":      result.Error,
				},
			}
		}
	}

	// 继续等待
	return rpa.StepResult{
		Completed: false,
		NextSub:   2,
		Context:   ctx,
	}
}
