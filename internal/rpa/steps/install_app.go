package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"port-mapping-demo/internal/database"
	"port-mapping-demo/internal/rpa"
	"port-mapping-demo/internal/service"
	"port-mapping-demo/pkg/logger"
	"time"
)

// InstallAppAndWaitStep 安装应用步骤
// 子步骤：发送下载安装指令 -> 等待下载完成 -> 等待安装完成
type InstallAppAndWaitStep struct{}

func init() {
	rpa.RegisterStep(&InstallAppAndWaitStep{})
}

func (s *InstallAppAndWaitStep) Type() string {
	return "install_app_and_wait"
}

func (s *InstallAppAndWaitStep) Name() string {
	return "安装应用"
}

func (s *InstallAppAndWaitStep) SubSteps() []string {
	return []string{"发送下载指令", "等待下载完成", "等待安装完成"}
}

func (s *InstallAppAndWaitStep) Execute(deviceID int, params map[string]interface{}, subStep int, ctx database.StepContext) rpa.StepResult {
	switch subStep {
	case 0:
		return s.sendDownloadInstall(deviceID, params, ctx)
	case 1:
		return s.waitDownloadComplete(deviceID, ctx)
	case 2:
		return s.waitInstallComplete(deviceID, ctx)
	default:
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("未知子步骤: %d", subStep),
		}
	}
}

// sendDownloadInstall 发送下载安装指令（通过 WebSocket 统一接口）
func (s *InstallAppAndWaitStep) sendDownloadInstall(deviceID int, params map[string]interface{}, ctx database.StepContext) rpa.StepResult {
	// 获取参数
	url, _ := params["url"].(string)
	name, _ := params["name"].(string)
	sha256, _ := params["sha256"].(string)
	install := true
	if v, ok := params["install"].(bool); ok {
		install = v
	}

	if url == "" {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "[第2步:安装应用/发送下载指令] 缺少必要参数: url",
		}
	}

	// 自动生成文件名
	if name == "" {
		name = "app.apk"
	}

	// 通过 HandleUnifiedRequestHTTP 发送下载安装请求
	req := &service.UnifiedRequest{
		Type: "downLoadInstallApp",
		Data: map[string]interface{}{
			"devices": []interface{}{float64(deviceID)},
			"name":    name,
			"url":     url,
			"sha256":  sha256,
			"install": install,
			"receive": true,
		},
	}

	logger.LogInfo("[RPA] 设备 %d 发送下载安装请求: url=%s, name=%s", deviceID, url, name)

	res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
	if err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[第2步:安装应用/发送下载指令] 发送下载指令失败: %v", err),
		}
	}

	if res.Code != 200 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[第%d步:安装应用/发送下载指令] %s", 2, res.Msg),
		}
	}

	// 调试：打印返回数据
	resJson, _ := json.Marshal(res.Data)
	logger.LogInfo("[RPA] 设备 %d downLoadInstallApp 返回: type=%T, data=%s", deviceID, res.Data, string(resJson))

	// 解析返回数据获取任务 ID - 通过 JSON 反序列化统一处理
	var taskID float64

	// 尝试解析为 map
	var dataMap map[string]interface{}
	if err := json.Unmarshal(resJson, &dataMap); err == nil {
		if id, ok := dataMap["id"].(float64); ok && id > 0 {
			taskID = id
		}
	}

	// 尝试解析为数组
	if taskID == 0 {
		var dataList []map[string]interface{}
		if err := json.Unmarshal(resJson, &dataList); err == nil && len(dataList) > 0 {
			if id, ok := dataList[0]["id"].(float64); ok && id > 0 {
				taskID = id
			}
		}
	}

	if taskID == 0 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[第%d步:安装应用/发送下载指令] 未获取到下载任务ID，接口返回: %s", 2, res.Msg),
		}
	}

	logger.LogInfo("[RPA] 设备 %d 下载任务已提交, taskId=%v", deviceID, taskID)

	// 更新上下文，进入下一个子步骤
	newCtx := make(database.StepContext)
	newCtx["url"] = url
	newCtx["name"] = name
	newCtx["install"] = install
	newCtx["taskId"] = taskID
	newCtx["startTime"] = float64(time.Now().Unix())

	return rpa.StepResult{
		Completed: false,
		NextSub:   1,
		Context:   newCtx,
	}
}

// waitDownloadComplete 等待下载完成（通过 WebSocket 统一接口查询）
func (s *InstallAppAndWaitStep) waitDownloadComplete(deviceID int, ctx database.StepContext) rpa.StepResult {
	startTime, _ := ctx["startTime"].(float64)
	timeout := 300.0 // 默认超时 300 秒（5分钟）
	if t, ok := ctx["downloadTimeout"].(float64); ok {
		timeout = t
	}

	// 检查超时
	if float64(time.Now().Unix())-startTime > timeout {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[第2步:安装应用/等待下载完成] 下载超时（%.0f秒）", timeout),
		}
	}

	// 获取 taskId（可能是 float64 或 string）
	var taskID interface{}
	if v, ok := ctx["taskId"].(float64); ok {
		taskID = v
	} else if v, ok := ctx["taskId"].(string); ok {
		taskID = v
	}

	if taskID == nil {
		ctxJson, _ := json.Marshal(ctx)
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[第2步:安装应用/等待下载完成] 上下文中缺少 taskId, ctx=%s", string(ctxJson)),
		}
	}

	// 通过 HandleUnifiedRequestHTTP 查询下载进度
	// taskId 转为 string，deviceId 转为 float64
	taskIDStr := fmt.Sprintf("%v", taskID)
	logger.LogInfo("[RPA] 设备 %d 查询下载进度, taskId=%s", deviceID, taskIDStr)
	req := &service.UnifiedRequest{
		Type: "getDownloadProgress",
		Data: map[string]interface{}{
			"deviceId": float64(deviceID),
			"id":       taskIDStr,
		},
	}

	res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
	if err != nil {
		logger.LogInfo("[RPA] 设备 %d 查询下载进度失败, taskId=%v, err=%v", deviceID, taskID, err)
		// 查询失败，继续等待重试
		return rpa.StepResult{
			Completed: false,
			NextSub:   1,
			Context:   ctx,
		}
	}

	// 打印返回数据
	resJson, _ := json.Marshal(res.Data)
	logger.LogInfo("[RPA] 设备 %d getDownloadProgress 返回, taskId=%v, code=%d, data=%s", deviceID, taskID, res.Code, string(resJson))

	if res.Code != 200 {
		logger.LogInfo("[RPA] 设备 %d 查询下载进度返回非200, taskId=%v, code=%d, msg=%s", deviceID, taskID, res.Code, res.Msg)
		// 继续等待
		return rpa.StepResult{
			Completed: false,
			NextSub:   1,
			Context:   ctx,
		}
	}

	// 解析返回数据 - 通过 JSON 反序列化统一处理
	// 返回格式: {id: 26, status: 3, process: 1, ...}
	status := 0
	process := 0

	var dataMap map[string]interface{}
	if err := json.Unmarshal(resJson, &dataMap); err == nil {
		if s, ok := dataMap["status"].(float64); ok {
			status = int(s)
		}
		if p, ok := dataMap["process"].(float64); ok {
			process = int(p)
		}
	}

	logger.LogInfo("[RPA] 设备 %d 下载进度, taskId=%v, status=%d, process=%d", deviceID, taskID, status, process)

	// Status: -1失败, 0排队中, 1正在下载, 2等待重试, 3下载完成(不需安装)/等待安装, 4安装成功, -2安装失败
	// Process: 0进行中, 1传输完成
	switch status {
	case -1, -2:
		// 失败
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[第2步:安装应用/等待下载完成] 下载/安装失败: status=%d", status),
		}
	case 3:
		// 下载完成
		logger.LogInfo("[RPA] 设备 %d 下载完成", deviceID)
		ctx["downloadCompleteTime"] = float64(time.Now().Unix())

		// 检查是否需要安装
		install, _ := ctx["install"].(bool)
		if !install {
			// 不需要安装，直接完成
			return rpa.StepResult{
				Completed: true,
				Success:   true,
				Output: map[string]interface{}{
					"deviceId":   deviceID,
					"downloaded": true,
					"installed":  false,
				},
			}
		}

		// 进入等待安装阶段
		return rpa.StepResult{
			Completed: false,
			NextSub:   2,
			Context:   ctx,
		}
	case 4:
		// 安装成功（有些情况下会直接跳到安装成功）
		logger.LogInfo("[RPA] 设备 %d 安装成功", deviceID)
		return rpa.StepResult{
			Completed: true,
			Success:   true,
			Output: map[string]interface{}{
				"deviceId":   deviceID,
				"downloaded": true,
				"installed":  true,
			},
		}
	default:
		// 继续等待（0排队中, 1正在下载, 2等待重试）
		return rpa.StepResult{
			Completed: false,
			NextSub:   1,
			Context:   ctx,
		}
	}
}

// waitInstallComplete 等待安装完成
func (s *InstallAppAndWaitStep) waitInstallComplete(deviceID int, ctx database.StepContext) rpa.StepResult {
	downloadCompleteTime, _ := ctx["downloadCompleteTime"].(float64)
	installTimeout := 120.0 // 安装超时 120 秒
	if t, ok := ctx["installTimeout"].(float64); ok {
		installTimeout = t
	}

	// 检查超时
	if float64(time.Now().Unix())-downloadCompleteTime > installTimeout {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[第2步:安装应用/等待安装完成] 安装超时（%.0f秒）", installTimeout),
		}
	}

	// 获取 taskId
	var taskID interface{}
	if v, ok := ctx["taskId"].(float64); ok {
		taskID = v
	} else if v, ok := ctx["taskId"].(string); ok {
		taskID = v
	}

	if taskID == nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "[第2步:安装应用/等待安装完成] 上下文中缺少 taskId",
		}
	}

	// 通过 HandleUnifiedRequestHTTP 查询安装进度
	taskIDStr := fmt.Sprintf("%v", taskID)
	logger.LogInfo("[RPA] 设备 %d 查询安装进度, taskId=%s", deviceID, taskIDStr)
	req := &service.UnifiedRequest{
		Type: "getDownloadProgress",
		Data: map[string]interface{}{
			"deviceId": float64(deviceID),
			"id":       taskIDStr,
		},
	}

	res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
	if err != nil {
		logger.LogInfo("[RPA] 设备 %d 查询安装进度失败, taskId=%v, err=%v", deviceID, taskID, err)
		// 查询失败，继续等待重试
		return rpa.StepResult{
			Completed: false,
			NextSub:   2,
			Context:   ctx,
		}
	}

	resJson, _ := json.Marshal(res.Data)
	logger.LogInfo("[RPA] 设备 %d getDownloadProgress(安装) 返回, taskId=%v, code=%d, data=%s", deviceID, taskID, res.Code, string(resJson))

	if res.Code != 200 {
		// 继续等待
		return rpa.StepResult{
			Completed: false,
			NextSub:   2,
			Context:   ctx,
		}
	}

	// 解析返回数据 - 通过 JSON 反序列化统一处理
	status := 0
	var dataMap map[string]interface{}
	if err := json.Unmarshal(resJson, &dataMap); err == nil {
		if s, ok := dataMap["status"].(float64); ok {
			status = int(s)
		}
	}

	logger.LogInfo("[RPA] 设备 %d 安装进度, taskId=%v, status=%d", deviceID, taskID, status)

	// Status: -1失败, 0排队中, 1正在下载, 2等待重试, 3下载完成, 4安装成功, -2安装失败
	switch status {
	case -1, -2:
		// 失败
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[第2步:安装应用/等待安装完成] 安装失败: status=%d", status),
		}
	case 4:
		// 安装成功
		logger.LogInfo("[RPA] 设备 %d 安装成功", deviceID)
		return rpa.StepResult{
			Completed: true,
			Success:   true,
			Output: map[string]interface{}{
				"deviceId":   deviceID,
				"downloaded": true,
				"installed":  true,
			},
		}
	default:
		// 继续等待
		return rpa.StepResult{
			Completed: false,
			NextSub:   2,
			Context:   ctx,
		}
	}
}
