package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// FileItem 文件管理器中的文件
type FileItem struct {
	FileID   int64  `json:"fileId"`
	FileName string `json:"fileName"`
	Size     int64  `json:"size"`
	Hash     string `json:"hash"`
	AddTime  int64  `json:"addTime"`
	URL      string `json:"url"`
}

// FileCommand 文件管理 CLI 入口
func FileCommand(args []string) {
	if len(args) == 0 {
		printFileUsage()
		return
	}

	switch args[0] {
	case "list", "ls":
		fileList(args[1:])
	case "install":
		fileInstall(args[1:])
	case "status":
		fileStatus(args[1:])
	case "help", "-h", "--help":
		printFileUsage()
	default:
		fmt.Printf("未知子命令: %s\n", args[0])
		printFileUsage()
	}
}

func printFileUsage() {
	fmt.Println("文件管理命令:")
	fmt.Println("  jpy-cloud file list [--json]                          列出文件管理器中的文件")
	fmt.Println("  jpy-cloud file install <设备ID> <fileId> [--json]     安装文件到设备（自动轮询进度）")
	fmt.Println("  jpy-cloud file status <设备ID> <taskId> [--json]      查询安装进度")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  jpy-cloud file list                                   查看所有文件")
	fmt.Println("  jpy-cloud file install 112230 42                      安装 fileId=42 的APK到设备")
	fmt.Println("  jpy-cloud file status 112230 3                        查询任务ID=3的安装进度")
}

func fileList(args []string) {
	jsonOutput := false
	for _, a := range args {
		if a == "--json" {
			jsonOutput = true
		}
	}

	result, err := CallUnified("", "", "getUserFiles", map[string]interface{}{})
	if err != nil {
		fmt.Printf("获取文件列表失败: %v\n", err)
		os.Exit(1)
	}

	files := parseFileList(result.Data)

	if jsonOutput {
		OutputJSON(files)
		return
	}

	if len(files) == 0 {
		fmt.Println("文件管理器中暂无文件")
		return
	}

	fmt.Printf("共 %d 个文件:\n\n", len(files))
	fmt.Printf("  %-8s %-30s %-12s %s\n", "FileID", "文件名", "大小", "下载URL")
	fmt.Printf("  %s\n", strings.Repeat("-", 100))
	for _, f := range files {
		sizeStr := formatFileSize(f.Size)
		name := f.FileName
		if len(name) > 28 {
			name = name[:25] + "..."
		}
		fmt.Printf("  %-8d %-30s %-12s %s\n", f.FileID, name, sizeStr, f.URL)
	}
}

func fileInstall(args []string) {
	if len(args) < 2 {
		fmt.Println("用法: jpy-cloud file install <设备ID> <fileId> [--json]")
		return
	}

	deviceIDStr := args[0]
	fileIDStr := args[1]
	jsonOutput := false
	for _, a := range args[2:] {
		if a == "--json" {
			jsonOutput = true
		}
	}

	deviceID, err := strconv.ParseInt(deviceIDStr, 10, 64)
	if err != nil {
		fmt.Printf("无效的设备ID: %s\n", deviceIDStr)
		return
	}

	fileID, err := strconv.ParseInt(fileIDStr, 10, 64)
	if err != nil {
		fmt.Printf("无效的fileId: %s\n", fileIDStr)
		return
	}

	// 1. 获取文件列表，找到 fileId 对应的下载 URL
	if !jsonOutput {
		fmt.Printf("正在查找 fileId=%d 的下载地址...\n", fileID)
	}

	listResult, err := CallUnified("", "", "getUserFiles", map[string]interface{}{})
	if err != nil {
		fmt.Printf("获取文件列表失败: %v\n", err)
		os.Exit(1)
	}

	files := parseFileList(listResult.Data)
	var downloadURL, fileName string
	for _, f := range files {
		if f.FileID == fileID {
			downloadURL = f.URL
			fileName = f.FileName
			break
		}
	}

	if downloadURL == "" {
		fmt.Printf("未找到 fileId=%d 的文件\n", fileID)
		if len(files) > 0 {
			fmt.Println("可用文件:")
			for _, f := range files {
				fmt.Printf("  fileId=%-6d %s\n", f.FileID, f.FileName)
			}
		}
		return
	}

	if !jsonOutput {
		fmt.Printf("找到文件: %s\n", fileName)
		fmt.Printf("正在安装到设备 %d...\n", deviceID)
	}

	// 2. 下发安装任务
	installResult, err := CallUnifiedWithTimeout("", "", "downLoadInstallApp", map[string]interface{}{
		"devices": []int64{deviceID},
		"url":     downloadURL,
		"install": true,
	}, 180*time.Second)
	if err != nil {
		fmt.Printf("安装任务下发失败: %v\n", err)
		os.Exit(1)
	}

	// 3. 提取原始任务 ID（保持 number 类型，不转 string）
	rawTaskID := extractRawTaskID(installResult.Data)
	if rawTaskID == nil {
		if jsonOutput {
			OutputJSON(installResult.Data)
		} else {
			fmt.Println("⚠ 安装任务已下发，但未返回任务ID，无法轮询进度")
		}
		return
	}

	if !jsonOutput {
		fmt.Printf("任务已下发 (taskId=%v)，轮询安装进度...\n", rawTaskID)
	}

	// 4. 轮询安装进度（透传原始 id 类型）
	finalResult := pollInstallProgress(deviceID, rawTaskID, jsonOutput)
	if jsonOutput {
		OutputJSON(finalResult)
	}
}

func fileStatus(args []string) {
	if len(args) < 2 {
		fmt.Println("用法: jpy-cloud file status <设备ID> <taskId> [--json]")
		return
	}

	deviceIDStr := args[0]
	taskIDStr := args[1]
	jsonOutput := false
	for _, a := range args[2:] {
		if a == "--json" {
			jsonOutput = true
		}
	}

	deviceID, err := strconv.ParseInt(deviceIDStr, 10, 64)
	if err != nil {
		fmt.Printf("无效的设备ID: %s\n", deviceIDStr)
		return
	}

	// taskId 尝试转为数字传递（与 getDownloadProgress 后端 extractUint32Field 匹配）
	var taskIDVal interface{}
	if v, err := strconv.ParseFloat(taskIDStr, 64); err == nil {
		taskIDVal = v
	} else {
		taskIDVal = taskIDStr
	}

	result, err := CallUnified("", "", "getDownloadProgress", map[string]interface{}{
		"deviceId": deviceID,
		"id":       taskIDVal,
	})
	if err != nil {
		fmt.Printf("查询进度失败: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		OutputJSON(result.Data)
	} else {
		dataBytes, _ := json.Marshal(result.Data)
		var progress map[string]interface{}
		json.Unmarshal(dataBytes, &progress)
		printProgress(progress)
	}
}

// pollInstallProgress 轮询安装进度，最长180s
func pollInstallProgress(deviceID int64, rawTaskID interface{}, jsonOutput bool) interface{} {
	deadline := time.Now().Add(180 * time.Second)
	lastPct := -1

	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)

		result, err := CallUnified("", "", "getDownloadProgress", map[string]interface{}{
			"deviceId": deviceID,
			"id":       rawTaskID, // 透传原始类型（number），不转 string
		})
		if err != nil {
			if !jsonOutput {
				fmt.Printf("  查询进度失败: %v（重试中...）\n", err)
			}
			continue
		}

		dataBytes, _ := json.Marshal(result.Data)
		var progress map[string]interface{}
		if err := json.Unmarshal(dataBytes, &progress); err != nil {
			continue
		}

		// 提取进度百分比和状态
		pct := 0
		if v, ok := progress["process"].(float64); ok {
			pct = int(v)
		}
		// status 可能是 string("success") 或 number(-2)
		statusStr := ""
		statusNum := 0
		switch v := progress["status"].(type) {
		case string:
			statusStr = v
		case float64:
			statusNum = int(v)
		}

		// 进度有变化时打印
		if !jsonOutput && pct != lastPct {
			if statusStr != "" {
				fmt.Printf("  进度: %d%% (%s)\n", pct, statusStr)
			} else if statusNum != 0 {
				fmt.Printf("  进度: %d%% (status=%d)\n", pct, statusNum)
			} else {
				fmt.Printf("  进度: %d%%\n", pct)
			}
			lastPct = pct
		}

		// 完成判断
		if pct >= 100 {
			if !jsonOutput {
				fmt.Printf("✓ 安装完成\n")
			}
			return progress
		}
		if statusStr == "success" || statusStr == "completed" || statusStr == "done" {
			if !jsonOutput {
				fmt.Printf("✓ 安装完成\n")
			}
			return progress
		}
		// status 为负数 → 设备端失败（如 -2 = 下载完成但安装失败）
		if statusNum < 0 {
			if !jsonOutput {
				errMsg := ""
				if v, ok := progress["error"].(string); ok {
					errMsg = v
				}
				if errMsg != "" {
					fmt.Printf("✗ 安装失败 (status=%d): %s\n", statusNum, errMsg)
				} else {
					fmt.Printf("✗ 安装失败 (status=%d)\n", statusNum)
				}
			}
			return progress
		}
		if statusStr == "failed" || statusStr == "error" {
			if !jsonOutput {
				errMsg := ""
				if v, ok := progress["error"].(string); ok {
					errMsg = v
				}
				fmt.Printf("✗ 安装失败: %s\n", errMsg)
			}
			return progress
		}
	}

	if !jsonOutput {
		fmt.Printf("⚠ 轮询超时（180s），可手动查询: jpy-cloud file status %d %v\n", deviceID, rawTaskID)
	}
	return map[string]interface{}{"timeout": true, "taskId": rawTaskID}
}

// extractRawTaskID 从安装响应中提取原始任务ID（保持 number 类型）
func extractRawTaskID(data interface{}) interface{} {
	if data == nil {
		return nil
	}
	dataBytes, _ := json.Marshal(data)

	// 可能是单个对象 {"id": 3, ...}
	var obj map[string]interface{}
	if err := json.Unmarshal(dataBytes, &obj); err == nil {
		if v, ok := obj["id"]; ok && v != nil {
			return v // 保持原始类型（float64 / string）
		}
	}

	// 可能是数组 [{"id": 3, ...}]
	var arr []map[string]interface{}
	if err := json.Unmarshal(dataBytes, &arr); err == nil && len(arr) > 0 {
		if v, ok := arr[0]["id"]; ok && v != nil {
			return v
		}
	}

	return nil
}

// parseFileList 解析文件列表响应
func parseFileList(data interface{}) []FileItem {
	dataBytes, _ := json.Marshal(data)
	var files []FileItem
	if err := json.Unmarshal(dataBytes, &files); err == nil && len(files) > 0 {
		return files
	}

	var rawFiles []map[string]interface{}
	if err := json.Unmarshal(dataBytes, &rawFiles); err != nil {
		return nil
	}
	for _, rf := range rawFiles {
		f := FileItem{}
		if v, ok := rf["fileId"].(float64); ok {
			f.FileID = int64(v)
		}
		if v, ok := rf["fileName"].(string); ok {
			f.FileName = v
		}
		if v, ok := rf["size"].(float64); ok {
			f.Size = int64(v)
		}
		if v, ok := rf["hash"].(string); ok {
			f.Hash = v
		}
		if v, ok := rf["addTime"].(float64); ok {
			f.AddTime = int64(v)
		}
		if v, ok := rf["url"].(string); ok {
			f.URL = v
		}
		files = append(files, f)
	}
	return files
}

func printProgress(progress map[string]interface{}) {
	pct := 0
	if v, ok := progress["process"].(float64); ok {
		pct = int(v)
	}
	status := "unknown"
	if v, ok := progress["status"].(string); ok && v != "" {
		status = v
	}
	fmt.Printf("进度: %d%%, 状态: %s\n", pct, status)
}

func fileName(progress map[string]interface{}) string {
	if v, ok := progress["fileName"].(string); ok {
		return v
	}
	return ""
}

// formatFileSize 格式化文件大小
func formatFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	} else if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
}
