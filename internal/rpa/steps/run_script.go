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

// RunScriptStep 执行脚本步骤
// 通过 WebSocket 推送脚本到设备执行
type RunScriptStep struct{}

func init() {
	rpa.RegisterStep(&RunScriptStep{})
}

func (s *RunScriptStep) Type() string {
	return "run_script"
}

func (s *RunScriptStep) Name() string {
	return "执行脚本"
}

func (s *RunScriptStep) SubSteps() []string {
	return []string{"推送脚本", "等待执行结果"}
}

func (s *RunScriptStep) Execute(deviceID int, params map[string]interface{}, subStep int, ctx database.StepContext) rpa.StepResult {
	switch subStep {
	case 0:
		return s.pushScript(deviceID, params, ctx)
	case 1:
		return s.waitResult(deviceID, params, ctx)
	default:
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("未知子步骤: %d", subStep),
		}
	}
}

// pushScript 推送脚本到设备
func (s *RunScriptStep) pushScript(deviceID int, params map[string]interface{}, ctx database.StepContext) rpa.StepResult {
	// 获取脚本代码
	code, _ := params["code"].(string)
	if code == "" {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "缺少必要参数: code（脚本代码）",
		}
	}

	// 获取可选参数
	taskName, _ := params["taskName"].(string)
	if taskName == "" {
		taskName = "RPA脚本任务"
	}
	timeout := int64(60000) // 默认 60 秒
	if t, ok := params["timeout"].(float64); ok && t > 0 {
		timeout = int64(t)
	}

	// 获取设备配置以读取流程变量
	config, _ := database.GetDeviceConfig(deviceID)
	var flowVars map[string]interface{}
	if config != nil && config.FlowVariables != nil {
		flowVars = config.FlowVariables
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
	deviceUUID := getDeviceUUID(deviceID)
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
	taskID := fmt.Sprintf("rpa_%d_%d", deviceID, time.Now().UnixNano())
	scriptHash := hashScript(code)

	// 判断脚本大小，决定推送方式
	// 小于 10KB 直接推送代码，大于则只推送 hash
	const maxDirectSize = 10 * 1024
	var payload devicews.TaskPushPayload

	if len(code) < maxDirectSize {
		// 小脚本：直接推送代码
		payload = devicews.TaskPushPayload{
			TaskID:     taskID,
			TaskName:   taskName,
			ScriptHash: scriptHash,
			Code:       code,
			Timeout:    timeout,
			Priority:   1,
			Variables:  flowVars, // 注入流程变量
		}
		logger.LogInfo("[RPA] 设备 %d 直接推送脚本（%d 字节），变量: %d 个", deviceID, len(code), len(flowVars))
	} else {
		// 大脚本：先上传到集控平台缓存，只推送 hash
		// TODO: 实现大脚本上传到集控平台
		// 目前先直接推送
		payload = devicews.TaskPushPayload{
			TaskID:     taskID,
			TaskName:   taskName,
			ScriptHash: scriptHash,
			Code:       code,
			Timeout:    timeout,
			Priority:   1,
			Variables:  flowVars, // 注入流程变量
		}
		logger.LogInfo("[RPA] 设备 %d 推送大脚本（%d 字节），变量: %d 个", deviceID, len(code), len(flowVars))
	}

	// 发送任务推送
	if err := dc.SendTaskPush(&payload); err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("推送脚本失败: %v", err),
		}
	}

	logger.LogInfo("[RPA] 设备 %d 脚本已推送，任务ID: %s", deviceID, taskID)

	// 保存上下文，进入等待结果步骤
	newCtx := make(database.StepContext)
	newCtx["taskId"] = taskID
	newCtx["startTime"] = time.Now().Unix()
	newCtx["timeout"] = timeout
	newCtx["deviceUUID"] = deviceUUID

	return rpa.StepResult{
		Completed: false,
		NextSub:   1,
		Context:   newCtx,
	}
}

// waitResult 等待脚本执行结果
func (s *RunScriptStep) waitResult(deviceID int, params map[string]interface{}, ctx database.StepContext) rpa.StepResult {
	taskID, _ := ctx["taskId"].(string)
	startTime, _ := ctx["startTime"].(float64)
	timeout, _ := ctx["timeout"].(float64)
	deviceUUID, _ := ctx["deviceUUID"].(string)

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

	// 检查任务结果（从设备连接的结果缓存中获取）
	result, hasResult := dc.GetTaskResult(taskID)
	if hasResult {
		if result.Success {
			logger.LogInfo("[RPA] 设备 %d 脚本执行成功，任务ID: %s", deviceID, taskID)
			return rpa.StepResult{
				Completed: true,
				Success:   true,
				Output: map[string]interface{}{
					"taskId":   taskID,
					"result":   result.Result,
					"duration": result.Duration,
				},
			}
		} else {
			return rpa.StepResult{
				Completed: true,
				Success:   false,
				Error:     fmt.Sprintf("脚本执行失败: %s", result.Error),
			}
		}
	}

	// 继续等待
	return rpa.StepResult{
		Completed: false,
		NextSub:   1,
		Context:   ctx,
	}
}

// hashScript 计算脚本的 SHA256 hash
func hashScript(code string) string {
	h := sha256.New()
	h.Write([]byte(code))
	return hex.EncodeToString(h.Sum(nil))
}
