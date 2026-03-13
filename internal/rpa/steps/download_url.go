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

// DownloadUrlStep 从 URL 下载文件步骤
type DownloadUrlStep struct{}

func init() {
	rpa.RegisterStep(&DownloadUrlStep{})
}

func (s *DownloadUrlStep) Type() string {
	return "download_url"
}

func (s *DownloadUrlStep) Name() string {
	return "URL下载"
}

func (s *DownloadUrlStep) SubSteps() []string {
	return []string{"发送下载指令", "等待下载完成"}
}

func (s *DownloadUrlStep) Execute(deviceID int, params map[string]interface{}, subStep int, ctx database.StepContext) rpa.StepResult {
	switch subStep {
	case 0:
		return s.sendDownload(deviceID, params, ctx)
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

func (s *DownloadUrlStep) sendDownload(deviceID int, params map[string]interface{}, ctx database.StepContext) rpa.StepResult {
	url, _ := params["url"].(string)
	name, _ := params["name"].(string)
	sha256, _ := params["sha256"].(string)

	if url == "" {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "[URL下载/发送下载指令] 缺少必要参数: url",
		}
	}

	if name == "" {
		name = "downloaded_file"
	}

	// 使用 unified 接口发送下载命令（install=false 表示只下载不安装）
	req := &service.UnifiedRequest{
		Type: "downLoadInstallApp",
		Seq:  int(time.Now().UnixNano() % 1000000),
		Data: map[string]interface{}{
			"devices": []interface{}{float64(deviceID)},
			"url":     url,
			"name":    name,
			"sha256":  sha256,
			"install": false,
			"receive": true,
		},
	}

	logger.LogInfo("[RPA] 设备 %d 发送URL下载指令: url=%s, name=%s", deviceID, url, name)

	res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
	if err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[URL下载/发送下载指令] %v", err),
		}
	}

	if res.Code != 200 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[URL下载/发送下载指令] %s", res.Msg),
		}
	}

	// 解析返回数据获取任务 ID
	resJson, _ := json.Marshal(res.Data)
	logger.LogInfo("[RPA] 设备 %d downLoadInstallApp 返回: %s", deviceID, string(resJson))

	var taskID float64
	var dataMap map[string]interface{}
	if err := json.Unmarshal(resJson, &dataMap); err == nil {
		if id, ok := dataMap["id"].(float64); ok && id > 0 {
			taskID = id
		}
	}

	if taskID == 0 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[URL下载/发送下载指令] 未获取到下载任务ID，接口返回: %s", res.Msg),
		}
	}

	logger.LogInfo("[RPA] 设备 %d URL下载任务已提交, taskId=%v", deviceID, taskID)

	newCtx := make(database.StepContext)
	newCtx["url"] = url
	newCtx["name"] = name
	newCtx["taskId"] = taskID
	newCtx["startTime"] = float64(time.Now().Unix())

	return rpa.StepResult{
		Completed: false,
		NextSub:   1,
		Context:   newCtx,
	}
}

func (s *DownloadUrlStep) waitComplete(deviceID int, ctx database.StepContext) rpa.StepResult {
	startTime, _ := ctx["startTime"].(float64)
	timeout := 300.0
	if t, ok := ctx["timeout"].(float64); ok {
		timeout = t
	}

	if float64(time.Now().Unix())-startTime > timeout {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[URL下载/等待下载完成] 下载超时（%.0f秒）", timeout),
		}
	}

	taskID, _ := ctx["taskId"].(float64)
	if taskID == 0 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "[URL下载/等待下载完成] 上下文中缺少 taskId",
		}
	}

	// 查询下载进度
	taskIDStr := fmt.Sprintf("%v", taskID)
	logger.LogInfo("[RPA] 设备 %d 查询URL下载进度, taskId=%s", deviceID, taskIDStr)

	req := &service.UnifiedRequest{
		Type: "getDownloadProgress",
		Seq:  int(time.Now().UnixNano() % 1000000),
		Data: map[string]interface{}{
			"deviceId": float64(deviceID),
			"id":       taskIDStr,
		},
	}

	res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
	if err != nil {
		logger.LogInfo("[RPA] 设备 %d 查询URL下载进度失败: %v", deviceID, err)
		return rpa.StepResult{
			Completed: false,
			NextSub:   1,
			Context:   ctx,
		}
	}

	resJson, _ := json.Marshal(res.Data)
	logger.LogInfo("[RPA] 设备 %d getDownloadProgress 返回, taskId=%s, code=%d, data=%s", deviceID, taskIDStr, res.Code, string(resJson))

	if res.Code != 200 {
		return rpa.StepResult{
			Completed: false,
			NextSub:   1,
			Context:   ctx,
		}
	}

	// 解析返回数据
	var status int
	var path string
	var dataMap map[string]interface{}
	if err := json.Unmarshal(resJson, &dataMap); err == nil {
		if s, ok := dataMap["status"].(float64); ok {
			status = int(s)
		}
		if p, ok := dataMap["path"].(string); ok {
			path = p
		}
	}

	logger.LogInfo("[RPA] 设备 %d URL下载进度, taskId=%s, status=%d, path=%s", deviceID, taskIDStr, status, path)

	// Status: -1失败, 0排队中, 1正在下载, 2等待重试, 3下载完成
	switch status {
	case -1:
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[URL下载/等待下载完成] 下载失败: status=%d", status),
		}
	case 3:
		// 下载完成
		logger.LogInfo("[RPA] 设备 %d URL下载完成, 文件路径: %s", deviceID, path)
		return rpa.StepResult{
			Completed: true,
			Success:   true,
			Output: map[string]interface{}{
				"deviceId":   deviceID,
				"downloaded": true,
				"path":       path,
			},
		}
	}

	// 继续等待
	return rpa.StepResult{
		Completed: false,
		NextSub:   1,
		Context:   ctx,
	}
}
