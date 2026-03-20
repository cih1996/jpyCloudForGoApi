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

// DownloadCloudStep 从云端下载文件步骤（支持多文件）
type DownloadCloudStep struct{}

func init() {
	rpa.RegisterStep(&DownloadCloudStep{})
}

func (s *DownloadCloudStep) Type() string {
	return "download_cloud"
}

func (s *DownloadCloudStep) Name() string {
	return "云端下载"
}

func (s *DownloadCloudStep) SubSteps() []string {
	return []string{"发送下载指令", "等待下载完成", "后处理（移动/权限）"}
}

func (s *DownloadCloudStep) Execute(deviceID int, params map[string]interface{}, subStep int, ctx database.StepContext) rpa.StepResult {
	switch subStep {
	case 0:
		return s.sendDownloads(deviceID, params, ctx)
	case 1:
		return s.waitComplete(deviceID, params, ctx)
	case 2:
		return s.postProcess(deviceID, params, ctx)
	default:
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("未知子步骤: %d", subStep),
		}
	}
}

// CloudFile 云端文件信息
type CloudFile struct {
	FileName string `json:"fileName"`
	FileID   int    `json:"fileId"`
	URL      string `json:"url"`
	Hash     string `json:"hash"`
}

func (s *DownloadCloudStep) sendDownloads(deviceID int, params map[string]interface{}, ctx database.StepContext) rpa.StepResult {
	// 解析文件列��
	var files []CloudFile
	if filesRaw, ok := params["files"].([]interface{}); ok {
		for _, f := range filesRaw {
			if fm, ok := f.(map[string]interface{}); ok {
				file := CloudFile{
					FileName: getString(fm, "fileName"),
					URL:      getString(fm, "url"),
					Hash:     getString(fm, "hash"),
				}
				if id, ok := fm["fileId"].(float64); ok {
					file.FileID = int(id)
				}
				if file.URL != "" {
					files = append(files, file)
				}
			}
		}
	}

	// 兼容旧格式（单文件）
	if len(files) == 0 {
		url := getString(params, "url")
		fileName := getString(params, "fileName")
		if fileName == "" {
			fileName = getString(params, "name")
		}
		if url != "" {
			files = append(files, CloudFile{
				FileName: fileName,
				URL:      url,
				Hash:     getString(params, "sha256"),
			})
		}
	}

	if len(files) == 0 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "[云端下载/发送下载指令] 缺少必要参数: files 或 url",
		}
	}

	// 存储任务信息
	tasks := make([]map[string]interface{}, 0)

	// 为每个文件发送下载指令
	for _, file := range files {
		fileName := file.FileName
		if fileName == "" {
			fileName = "cloud_file"
		}

		req := &service.UnifiedRequest{
			Type: "downLoadInstallApp",
			Data: map[string]interface{}{
				"devices": []interface{}{float64(deviceID)},
				"url":     file.URL,
				"name":    fileName,
				"sha256":  file.Hash,
				"install": false,
				"receive": true,
			},
		}

		logger.LogInfo("[RPA] 设备 %d 发送云端下载指令: url=%s, name=%s", deviceID, file.URL, fileName)

		res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
		if err != nil {
			return rpa.StepResult{
				Completed: true,
				Success:   false,
				Error:     fmt.Sprintf("[云端下载/发送下载指令] %v", err),
			}
		}

		if res.Code != 200 {
			return rpa.StepResult{
				Completed: true,
				Success:   false,
				Error:     fmt.Sprintf("[云端下载/发送下载指令] %s", res.Msg),
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
				Error:     fmt.Sprintf("[云端下载/发送下载指令] 未获取到下载任务ID，文件: %s", fileName),
			}
		}

		tasks = append(tasks, map[string]interface{}{
			"taskId":   taskID,
			"fileName": fileName,
			"url":      file.URL,
			"status":   0,
			"path":     "",
		})

		logger.LogInfo("[RPA] 设备 %d 云端下载任务已提交, taskId=%v, fileName=%s", deviceID, taskID, fileName)
	}

	newCtx := make(database.StepContext)
	newCtx["tasks"] = tasks
	newCtx["startTime"] = float64(time.Now().Unix())

	return rpa.StepResult{
		Completed: false,
		NextSub:   1,
		Context:   newCtx,
	}
}

func (s *DownloadCloudStep) waitComplete(deviceID int, params map[string]interface{}, ctx database.StepContext) rpa.StepResult {
	startTime, _ := ctx["startTime"].(float64)
	timeout := 300.0
	if t, ok := ctx["timeout"].(float64); ok {
		timeout = t
	}

	if float64(time.Now().Unix())-startTime > timeout {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[云端下载/等待下载完成] 下载超时（%.0f秒）", timeout),
		}
	}

	tasksRaw, _ := ctx["tasks"].([]map[string]interface{})
	if len(tasksRaw) == 0 {
		// 尝试从 interface{} 转换
		if tasksInterface, ok := ctx["tasks"].([]interface{}); ok {
			for _, t := range tasksInterface {
				if tm, ok := t.(map[string]interface{}); ok {
					tasksRaw = append(tasksRaw, tm)
				}
			}
		}
	}

	if len(tasksRaw) == 0 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "[云端下载/等待下载完成] 上下文中缺少 tasks",
		}
	}

	allCompleted := true
	downloadedPaths := make([]string, 0)

	for i, task := range tasksRaw {
		status, _ := task["status"].(float64)
		if status == 3 {
			// 已完成
			if path, ok := task["path"].(string); ok && path != "" {
				downloadedPaths = append(downloadedPaths, path)
			}
			continue
		}

		taskID, _ := task["taskId"].(float64)
		if taskID == 0 {
			continue
		}

		// 查询下载进度
		taskIDStr := fmt.Sprintf("%v", taskID)
		req := &service.UnifiedRequest{
			Type: "getDownloadProgress",
			Data: map[string]interface{}{
				"deviceId": float64(deviceID),
				"id":       taskIDStr,
			},
		}

		res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
		if err != nil {
			allCompleted = false
			continue
		}

		if res.Code != 200 {
			allCompleted = false
			continue
		}

		resJson, _ := json.Marshal(res.Data)
		var dataMap map[string]interface{}
		if err := json.Unmarshal(resJson, &dataMap); err == nil {
			if s, ok := dataMap["status"].(float64); ok {
				tasksRaw[i]["status"] = s
				status = s
			}
			if p, ok := dataMap["path"].(string); ok {
				tasksRaw[i]["path"] = p
			}
		}

		fileName, _ := task["fileName"].(string)
		path, _ := tasksRaw[i]["path"].(string)
		logger.LogInfo("[RPA] 设备 %d 云端下载进度, taskId=%s, fileName=%s, status=%.0f, path=%s", deviceID, taskIDStr, fileName, status, path)

		// Status: -1失败, 0排队中, 1正在下载, 2等待重试, 3下载完成
		if status == -1 {
			return rpa.StepResult{
				Completed: true,
				Success:   false,
				Error:     fmt.Sprintf("[云端下载/等待下载完成] 文件 %s 下载失败", fileName),
			}
		}

		if status == 3 {
			if path != "" {
				downloadedPaths = append(downloadedPaths, path)
			}
		} else {
			allCompleted = false
		}
	}

	if !allCompleted {
		ctx["tasks"] = tasksRaw
		return rpa.StepResult{
			Completed: false,
			NextSub:   1,
			Context:   ctx,
		}
	}

	// 所有文件下载完成，进入后处理
	logger.LogInfo("[RPA] 设备 %d 所有云端文件下载完成, 共 %d 个文件", deviceID, len(downloadedPaths))
	ctx["tasks"] = tasksRaw
	ctx["downloadedPaths"] = downloadedPaths

	return rpa.StepResult{
		Completed: false,
		NextSub:   2,
		Context:   ctx,
	}
}

func (s *DownloadCloudStep) postProcess(deviceID int, params map[string]interface{}, ctx database.StepContext) rpa.StepResult {
	downloadedPaths, _ := ctx["downloadedPaths"].([]interface{})
	if len(downloadedPaths) == 0 {
		// 尝试从 []string 转换
		if paths, ok := ctx["downloadedPaths"].([]string); ok {
			for _, p := range paths {
				downloadedPaths = append(downloadedPaths, p)
			}
		}
	}

	targetDir := getString(params, "targetDir")
	executable, _ := params["executable"].(bool)

	finalPaths := make([]string, 0)

	for _, pathRaw := range downloadedPaths {
		path, _ := pathRaw.(string)
		if path == "" {
			continue
		}

		finalPath := path

		// 移动到目标目录
		if targetDir != "" {
			// 提取文件名
			fileName := path
			for i := len(path) - 1; i >= 0; i-- {
				if path[i] == '/' {
					fileName = path[i+1:]
					break
				}
			}
			newPath := targetDir
			if newPath[len(newPath)-1] != '/' {
				newPath += "/"
			}
			newPath += fileName

			// 执行 mv 命令
			mvCmd := fmt.Sprintf("mkdir -p %s && mv %s %s", targetDir, path, newPath)
			logger.LogInfo("[RPA] 设备 %d 移动文件: %s", deviceID, mvCmd)

			req := &service.UnifiedRequest{
				Type: "execShell",
				Data: map[string]interface{}{
					"deviceId": float64(deviceID),
					"shell":    mvCmd,
				},
			}

			res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
			if err != nil {
				logger.LogInfo("[RPA] 设备 %d 移动文件失败: %v", deviceID, err)
			} else if res.Code == 200 {
				finalPath = newPath
				logger.LogInfo("[RPA] 设备 %d 文件已移动到: %s", deviceID, newPath)
			}
		}

		// 赋予运行权限
		if executable {
			chmodCmd := fmt.Sprintf("chmod +x %s", finalPath)
			logger.LogInfo("[RPA] 设备 %d 赋予运行权限: %s", deviceID, chmodCmd)

			req := &service.UnifiedRequest{
				Type: "execShell",
				Data: map[string]interface{}{
					"deviceId": float64(deviceID),
					"shell":    chmodCmd,
				},
			}

			res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
			if err != nil {
				logger.LogInfo("[RPA] 设备 %d chmod 失败: %v", deviceID, err)
			} else if res.Code == 200 {
				logger.LogInfo("[RPA] 设备 %d 已赋予运行权限: %s", deviceID, finalPath)
			}
		}

		finalPaths = append(finalPaths, finalPath)
	}

	logger.LogInfo("[RPA] 设备 %d 云端下载完成, 文件路径: %v", deviceID, finalPaths)

	return rpa.StepResult{
		Completed: true,
		Success:   true,
		Output: map[string]interface{}{
			"deviceId":   deviceID,
			"downloaded": true,
			"paths":      finalPaths,
			"count":      len(finalPaths),
		},
	}
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
