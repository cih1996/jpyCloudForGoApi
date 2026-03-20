package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"port-mapping-demo/internal/database"
	"port-mapping-demo/internal/rpa"
	"port-mapping-demo/internal/service"
	"port-mapping-demo/pkg/logger"
	"strings"
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
	return []string{"写入配置", "启动accSys", "启动测试框架", "等待连接"}
}

func (s *StartBotStep) Execute(deviceID int, params map[string]interface{}, subStep int, ctx database.StepContext) rpa.StepResult {
	switch subStep {
	case 0:
		return s.writeConfig(deviceID, params, ctx)
	case 1:
		return s.startAccSys(deviceID, params, ctx)
	case 2:
		return s.startTestFramework(deviceID, params, ctx)
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

	// 探测 shell 通道是否真正可用（改机重启后设备端 shell 服务可能还没就绪）
	if !ProbeShellChannel(deviceID, 0, 0) {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "shell通道探测超时，设备可能未就绪",
		}
	}

	// TD-008: 截断过大的日志文件（executor.log 是 append 模式，长时间运行会膨胀）
	truncLogReq := &service.UnifiedRequest{
		Type: "execShell",
		Data: map[string]interface{}{
			"deviceId": float64(deviceID),
			"shell":    "[ $(stat -c%s /sdcard/accbot/executor.log 2>/dev/null || echo 0) -gt 1048576 ] && echo '' > /sdcard/accbot/executor.log && echo TRUNCATED || echo SKIP",
		},
	}
	truncRes, _ := service.HandleUnifiedRequestHTTP(context.Background(), truncLogReq)
	if truncRes != nil && truncRes.Code == 200 && truncRes.Data != nil {
		if strings.Contains(fmt.Sprintf("%v", truncRes.Data), "TRUNCATED") {
			logger.LogInfo("[RPA] 设备 %d executor.log 超过1MB，已截断", deviceID)
		}
	}

	// 检查 accSys 源文件是否存在（改机重装后 APK 未启动过，assets 未解压）
	accSysSource := fmt.Sprintf("/sdcard/Android/data/%s/cache/assets/sys/accSys", packageName)
	checkFileReq := &service.UnifiedRequest{
		Type: "execShell",
		Data: map[string]interface{}{
			"deviceId": float64(deviceID),
			"shell":    fmt.Sprintf("ls -l %s 2>&1", accSysSource),
		},
	}
	checkFileRes, _ := service.HandleUnifiedRequestHTTP(context.Background(), checkFileReq)
	fileExists := false
	if checkFileRes != nil && checkFileRes.Code == 200 && checkFileRes.Data != nil {
		dataStr := fmt.Sprintf("%v", checkFileRes.Data)
		if !strings.Contains(dataStr, "No such file") && strings.Contains(dataStr, "accSys") {
			fileExists = true
		}
	}

	if !fileExists {
		// 源文件不存在，先拉起 APK 触发 assets 解压
		logger.LogInfo("[RPA] 设备 %d accSys源文件不存在，先启动APK触发解压...", deviceID)
		launchReq := &service.UnifiedRequest{
			Type: "execShell",
			Data: map[string]interface{}{
				"deviceId": float64(deviceID),
				"shell":    fmt.Sprintf("monkey -p %s -c android.intent.category.LAUNCHER 1 2>/dev/null", packageName),
			},
		}
		service.HandleUnifiedRequestHTTP(context.Background(), launchReq)

		// 轮询等待 accSys 文件出现（最多60秒，每3秒检查一次）
		maxWait := 20
		found := false
		for i := 1; i <= maxWait; i++ {
			time.Sleep(3 * time.Second)
			pollReq := &service.UnifiedRequest{
				Type: "execShell",
				Data: map[string]interface{}{
					"deviceId": float64(deviceID),
					"shell":    fmt.Sprintf("ls %s 2>/dev/null && echo FILE_OK", accSysSource),
				},
			}
			pollRes, pollErr := service.HandleUnifiedRequestHTTP(context.Background(), pollReq)
			if pollErr == nil && pollRes.Code == 200 && pollRes.Data != nil {
				if strings.Contains(fmt.Sprintf("%v", pollRes.Data), "FILE_OK") {
					found = true
					logger.LogInfo("[RPA] 设备 %d accSys源文件已就绪（等待%d秒）", deviceID, i*3)
					break
				}
			}
			logger.LogInfo("[RPA] 设备 %d 等待accSys解压... (%d/%d)", deviceID, i, maxWait)
		}
		if !found {
			return rpa.StepResult{
				Completed: true,
				Success:   false,
				Error:     fmt.Sprintf("等待accSys源文件超时（60秒），APK可能未正确安装: %s", accSysSource),
			}
		}

		// 解压完成后关闭 APK（避免干扰后续 am instrument）
		stopReq := &service.UnifiedRequest{
			Type: "execShell",
			Data: map[string]interface{}{
				"deviceId": float64(deviceID),
				"shell":    fmt.Sprintf("am force-stop %s", packageName),
			},
		}
		service.HandleUnifiedRequestHTTP(context.Background(), stopReq)
		time.Sleep(1 * time.Second)
	}

	// 确保 /data 可写，复制 accSys 到 /data/local/tmp 并后台启动
	shellCmd := fmt.Sprintf(
		"mount -o remount,rw /data 2>/dev/null; cp %s /data/local/tmp/accSys && cd /data/local/tmp/ && chmod +x ./accSys && nohup ./accSys >./accSys.log 2>&1 &",
		accSysSource,
	)

	// 通道已确认可用，发送实际命令
	req := &service.UnifiedRequest{
		Type: "execShell",
		Data: map[string]interface{}{
			"deviceId": float64(deviceID),
			"shell":    shellCmd,
		},
	}

	logger.LogInfo("[RPA] 设备 %d 发送shell(startAccSys): %s", deviceID, shellCmd)
	res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
	if err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("启动accSys失败: %v", err),
		}
	}
	logger.LogInfo("[RPA] 设备 %d shell返回(startAccSys): code=%d msg=%s data=%v", deviceID, res.Code, res.Msg, res.Data)
	if res.Code != 200 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("启动accSys失败: %s", res.Msg),
		}
	}

	logger.LogInfo("[RPA] 设备 %d 启动accSys完成，等待3秒后检查进程...", deviceID)

	// 等待 3 秒让 accSys 充分启动
	time.Sleep(3 * time.Second)

	// 检查 accSys 进程是否存活
	checkReq := &service.UnifiedRequest{
		Type: "execShell",
		Data: map[string]interface{}{
			"deviceId": float64(deviceID),
			"shell":    "ps | grep accSys | grep -v grep",
		},
	}
	checkRes, checkErr := service.HandleUnifiedRequestHTTP(context.Background(), checkReq)
	if checkErr == nil && checkRes.Code == 200 {
		dataStr := fmt.Sprintf("%v", checkRes.Data)
		if dataStr == "" || dataStr == "<nil>" {
			// 进程不存在，读取崩溃日志
			logReq := &service.UnifiedRequest{
				Type: "execShell",
				Data: map[string]interface{}{
					"deviceId": float64(deviceID),
					"shell":    "cat /data/local/tmp/accSys.log 2>/dev/null | tail -20",
				},
			}
			logRes, logErr := service.HandleUnifiedRequestHTTP(context.Background(), logReq)
			crashLog := ""
			if logErr == nil && logRes.Code == 200 && logRes.Data != nil {
				crashLog = fmt.Sprintf("%v", logRes.Data)
			}
			logger.LogInfo("[RPA] 设备 %d accSys 启动后崩溃，日志: %s", deviceID, crashLog)
			return rpa.StepResult{
				Completed: true,
				Success:   false,
				Error:     fmt.Sprintf("accSys启动后崩溃（进程不存在），日志: %s", crashLog),
			}
		}
		logger.LogInfo("[RPA] 设备 %d accSys 进程存活确认: %s", deviceID, dataStr)
	}

	// 再等 2 秒确保 accSys 完全就绪
	time.Sleep(2 * time.Second)

	// 进入下一步：启动测试框架
	return rpa.StepResult{
		Completed: false,
		NextSub:   2,
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
		NextSub:   1,
		Context:   newCtx,
	}
}

// startTestFramework 通过 am instrument 启动测试框架（替代直接启动 APK）
func (s *StartBotStep) startTestFramework(deviceID int, params map[string]interface{}, ctx database.StepContext) rpa.StepResult {
	// 获取包名，默认 com.jpy.bot
	packageName, _ := params["packageName"].(string)
	if packageName == "" {
		packageName = "com.jpy.bot"
	}

	// 通过 am instrument 启动测试框架（脚本级权限）
	shellCmd := fmt.Sprintf(
		"nohup am instrument -w -r -e debug false -e class 'run.RunTest#useAppContext' %s/androidx.test.runner.AndroidJUnitRunner >/storage/emulated/0/Download/log_u2.txt 2>&1 &",
		packageName,
	)

	// 改机后连接不稳定，再次探测确认通道可用
	if !ProbeShellChannel(deviceID, 0, 0) {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "启动测试框架前shell通道探测超时",
		}
	}

	req := &service.UnifiedRequest{
		Type: "execShell",
		Data: map[string]interface{}{
			"deviceId": float64(deviceID),
			"shell":    shellCmd,
		},
	}

	logger.LogInfo("[RPA] 设备 %d 发送shell(startTestFramework): %s", deviceID, shellCmd)
	res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
	if err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("启动测试框架失败: %v", err),
		}
	}
	logger.LogInfo("[RPA] 设备 %d shell返回(startTestFramework): code=%d msg=%s data=%v", deviceID, res.Code, res.Msg, res.Data)
	if res.Code != 200 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("启动测试框架失败: %s", res.Msg),
		}
	}

	logger.LogInfo("[RPA] 设备 %d 通过 am instrument 启动: %s", deviceID, packageName)

	// 等待 15 秒让 APK 充分启动（am instrument 需要时间初始化测试框架并建立 WS 连接）
	logger.LogInfo("[RPA] 设备 %d 等待15秒让APK启动...", deviceID)
	time.Sleep(15 * time.Second)

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

	// 最多重试 60 次（约120秒超时），每次间隔由引擎轮询控制（2秒）
	maxRetries := 60
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

	// 检查设备是否已通过 WebSocket 连接（复用 service 层的统一匹配逻辑）
	connected := service.IsDeviceWSConnected(deviceID)

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
