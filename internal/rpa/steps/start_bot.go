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

// StartBotStep 启动脚本步骤
// 1. 写入配置文件到设备
// 2. 启动脚本 APK
// 3. 等待设备通过 WebSocket 连接
type StartBotStep struct{}

func init() {
	rpa.RegisterStep(&StartBotStep{})
}

func (s *StartBotStep) Type() string {
	return "start_bot"
}

func (s *StartBotStep) Name() string {
	return "启动脚本"
}

func (s *StartBotStep) SubSteps() []string {
	return []string{"启动accSys", "写入配置", "启动测试框架", "等待连接"}
}

func (s *StartBotStep) Execute(deviceID int, params map[string]interface{}, subStep int, ctx database.StepContext) rpa.StepResult {
	switch subStep {
	case 0:
		return s.startAccSys(deviceID, params, ctx)
	case 1:
		return s.writeConfig(deviceID, params, ctx)
	case 2:
		return s.startApp(deviceID, params, ctx)
	case 3:
		return s.waitConnection(deviceID, params, ctx)
	default:
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("未知子步骤: %d", subStep),
		}
	}
}

// startAccSys 启动 accSys 服务
func (s *StartBotStep) startAccSys(deviceID int, params map[string]interface{}, ctx database.StepContext) rpa.StepResult {
	// 获取包名，默认 com.jpy.bot
	packageName, _ := params["packageName"].(string)
	if packageName == "" {
		packageName = "com.jpy.bot"
	}

	// 构建 shell 命令：复制 accSys 到 /data/local/tmp 并后台启动
	shellCmd := fmt.Sprintf(
		"cp /sdcard/Android/data/%s/cache/assets/sys/accSys /data/local/tmp/accSys && cd /data/local/tmp/ && chmod +x ./accSys && nohup ./accSys >./accSys.log 2>&1 &",
		packageName,
	)

	req := &service.UnifiedRequest{
		Type: "execShell",
		Seq:  int(time.Now().Unix()),
		Data: map[string]interface{}{
			"deviceId": float64(deviceID),
			"shell":    shellCmd,
		},
	}

	res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
	if err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("启动accSys失败: %v", err),
		}
	}

	if res.Code != 200 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("启动accSys失败: %s", res.Msg),
		}
	}

	logger.LogInfo("[RPA] 设备 %d 启动accSys，等待3秒...", deviceID)

	// 等待 3 秒让 accSys 启动
	time.Sleep(3 * time.Second)

	// 进入下一步：写入配置
	return rpa.StepResult{
		Completed: false,
		NextSub:   1,
		Context:   ctx,
	}
}

// writeConfig 写入配置文件到设备
func (s *StartBotStep) writeConfig(deviceID int, params map[string]interface{}, ctx database.StepContext) rpa.StepResult {
	// 获取服务器地址参数
	serverUrl, _ := params["serverUrl"].(string)
	if serverUrl == "" {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "缺少必要参数: serverUrl（服务器地址）",
		}
	}

	// 获取可选参数
	deviceName, _ := params["deviceName"].(string)
	reconnectInterval := 5000
	if ri, ok := params["reconnectInterval"].(float64); ok && ri > 0 {
		reconnectInterval = int(ri)
	}

	// 构建配置 JSON
	configData := map[string]interface{}{
		"serverUrl":         serverUrl,
		"autoReconnect":     true,
		"reconnectInterval": reconnectInterval,
	}
	if deviceName != "" {
		configData["deviceName"] = deviceName
	}
	// apkPkg：目标应用包名
	packageName, _ := params["packageName"].(string)
	if packageName == "" {
		packageName = "com.jpy.bot"
	}
	configData["apkPkg"] = packageName

	configJSON, err := json.Marshal(configData)
	if err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("生成配置JSON失败: %v", err),
		}
	}

	// 通过 shell 命令写入配置文件
	// 先创建目录，再写入文件
	mkdirCmd := "mkdir -p /sdcard/accbot"
	writeCmd := fmt.Sprintf("echo '%s' > /sdcard/accbot/config.json", string(configJSON))

	// 执行 mkdir
	req := &service.UnifiedRequest{
		Type: "execShell",
		Seq:  int(time.Now().Unix()),
		Data: map[string]interface{}{
			"deviceId": float64(deviceID),
			"shell":    mkdirCmd,
		},
	}
	_, err = service.HandleUnifiedRequestHTTP(context.Background(), req)
	if err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("创建目录失败: %v", err),
		}
	}

	// 执行写入配置
	req = &service.UnifiedRequest{
		Type: "execShell",
		Seq:  int(time.Now().Unix()),
		Data: map[string]interface{}{
			"deviceId": float64(deviceID),
			"shell":    writeCmd,
		},
	}
	res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
	if err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("写入配置失败: %v", err),
		}
	}

	if res.Code != 200 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("写入配置失败: %s", res.Msg),
		}
	}

	logger.LogInfo("[RPA] 设备 %d 写入脚本配置: %s", deviceID, string(configJSON))

	// 保存上下文，进入下一步
	newCtx := make(database.StepContext)
	newCtx["serverUrl"] = serverUrl
	newCtx["configWritten"] = true

	return rpa.StepResult{
		Completed: false,
		NextSub:   2,
		Context:   newCtx,
	}
}

// startApp 通过 am instrument 启动测试框架（替代直接启动 APK）
func (s *StartBotStep) startApp(deviceID int, params map[string]interface{}, ctx database.StepContext) rpa.StepResult {
	// 获取包名，默认 com.jpy.bot
	packageName, _ := params["packageName"].(string)
	if packageName == "" {
		packageName = "com.jpy.bot"
	}

	// 等待 1 秒，让 accSys 充分启动
	time.Sleep(1 * time.Second)

	// 通过 am instrument 启动测试框架（脚本级权限）
	shellCmd := fmt.Sprintf(
		"nohup am instrument -w -r -e debug false -e class 'run.RunTest#useAppContext' %s/androidx.test.runner.AndroidJUnitRunner >/storage/emulated/0/Download/log_u2.txt 2>&1 &",
		packageName,
	)

	req := &service.UnifiedRequest{
		Type: "execShell",
		Seq:  int(time.Now().Unix()),
		Data: map[string]interface{}{
			"deviceId": float64(deviceID),
			"shell":    shellCmd,
		},
	}

	res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
	if err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("启动测试框架失败: %v", err),
		}
	}

	if res.Code != 200 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("启动测试框架失败: %s", res.Msg),
		}
	}

	logger.LogInfo("[RPA] 设备 %d 通过 am instrument 启动: %s", deviceID, packageName)

	// 保存上下文，进入等待连接步骤
	newCtx := make(database.StepContext)
	for k, v := range ctx {
		newCtx[k] = v
	}
	newCtx["packageName"] = packageName
	newCtx["startTime"] = time.Now().Unix()
	newCtx["retryCount"] = 0

	return rpa.StepResult{
		Completed: false,
		NextSub:   3,
		Context:   newCtx,
	}
}

// waitConnection 等待设备通过 WebSocket 连接
func (s *StartBotStep) waitConnection(deviceID int, params map[string]interface{}, ctx database.StepContext) rpa.StepResult {
	startTime, _ := ctx["startTime"].(float64)
	retryCount, _ := ctx["retryCount"].(float64)

	// 最多重试 10 次，每次间隔 1 秒
	maxRetries := 10
	if mr, ok := params["maxRetries"].(float64); ok && mr > 0 {
		maxRetries = int(mr)
	}

	if int(retryCount) >= maxRetries {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("等待脚本连接超时（重试 %d 次）", maxRetries),
		}
	}

	// 检查设备是否已通过 WebSocket 连接
	wsServer := service.GetDeviceWSServer()
	if wsServer == nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "WebSocket 服务器未初始化",
		}
	}

	// 获取设备的 UUID（serialno）用于匹配
	// 需要从设备信息中获取 UUID
	deviceUUID := getDeviceUUID(deviceID)
	if deviceUUID == "" {
		// 如果获取不到 UUID，尝试用 deviceID 的其他方式匹配
		logger.LogInfo("[RPA] 设备 %d 无法获取 UUID，尝试遍历所有连接", deviceID)
	}

	// 检查是否已连接
	connected := false
	if deviceUUID != "" {
		// 通过 serialno 查找
		if _, ok := wsServer.GetManager().GetBySerialNo(deviceUUID); ok {
			connected = true
		}
	}

	// 如果通过 UUID 没找到，遍历所有连接检查
	if !connected {
		devices := wsServer.GetManager().GetAll()
		for _, dc := range devices {
			// 可以通过其他方式匹配，比如设备信息中的某些字段
			if dc.Serialno == deviceUUID || dc.Serialno == fmt.Sprintf("%d", deviceID) {
				connected = true
				break
			}
		}
	}

	if connected {
		elapsed := time.Now().Unix() - int64(startTime)
		logger.LogInfo("[RPA] 设备 %d 脚本已连接（耗时 %d 秒）", deviceID, elapsed)
		return rpa.StepResult{
			Completed: true,
			Success:   true,
			Output: map[string]interface{}{
				"connected":  true,
				"retryCount": int(retryCount),
				"elapsed":    elapsed,
			},
		}
	}

	// 未连接，继续等待
	logger.LogInfo("[RPA] 设备 %d 等待脚本连接... (第 %d 次)", deviceID, int(retryCount)+1)

	newCtx := make(database.StepContext)
	for k, v := range ctx {
		newCtx[k] = v
	}
	newCtx["retryCount"] = retryCount + 1

	return rpa.StepResult{
		Completed: false,
		NextSub:   3,
		Context:   newCtx,
	}
}

// getDeviceUUID 获取设备的 UUID
func getDeviceUUID(deviceID int) string {
	// 通过 unified handler 获取设备详情
	req := &service.UnifiedRequest{
		Type: "getDeviceDetail",
		Seq:  int(time.Now().Unix()),
		Data: map[string]interface{}{
			"deviceId": float64(deviceID),
		},
	}

	res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
	if err != nil {
		return ""
	}

	if res.Code != 200 || res.Data == nil {
		return ""
	}

	// 解析返回数据获取 UUID
	if dataMap, ok := res.Data.(map[string]interface{}); ok {
		if deviceInfo, ok := dataMap["deviceInfo"].(map[string]interface{}); ok {
			if uuid, ok := deviceInfo["uuid"].(string); ok {
				return uuid
			}
		}
	}

	return ""
}
