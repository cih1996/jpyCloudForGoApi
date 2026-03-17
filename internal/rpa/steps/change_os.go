package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"port-mapping-demo/internal/database"
	"port-mapping-demo/internal/rpa"
	"port-mapping-demo/internal/service"
	"port-mapping-demo/pkg/logger"
	"time"
)

// ChangeOsAndWaitStep 改机重启步骤
// 子步骤：发送改机 -> 等待改机完成 -> 等待设备上线
type ChangeOsAndWaitStep struct{}

func init() {
	rpa.RegisterStep(&ChangeOsAndWaitStep{})
}

func (s *ChangeOsAndWaitStep) Type() string {
	return "change_os_and_wait"
}

func (s *ChangeOsAndWaitStep) Name() string {
	return "改机重启"
}

func (s *ChangeOsAndWaitStep) SubSteps() []string {
	return []string{"发送改机指令", "等待改机完成", "等待设备上线"}
}

func (s *ChangeOsAndWaitStep) Execute(deviceID int, params map[string]interface{}, subStep int, ctx database.StepContext) rpa.StepResult {
	switch subStep {
	case 0:
		return s.sendChangeOs(deviceID, params, ctx)
	case 1:
		return s.waitChangeOsComplete(deviceID, ctx)
	case 2:
		return s.waitDeviceOnline(deviceID, ctx)
	default:
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("未知子步骤: %d", subStep),
		}
	}
}

// sendChangeOs 发送改机指令（通过 WebSocket 统一接口）
func (s *ChangeOsAndWaitStep) sendChangeOs(deviceID int, params map[string]interface{}, ctx database.StepContext) rpa.StepResult {
	// 获取国家代码（用于随机生成）
	country := ""
	if c, ok := params["country"].(string); ok {
		country = c
	}

	// 处理 MSISDN（本机号码）
	msisdn := ""
	if randomMsisdn, ok := params["randomMsisdn"].(bool); ok && randomMsisdn {
		msisdn = generatePhoneNumber(country)
	} else if m, ok := params["msisdn"].(string); ok {
		msisdn = m
	}

	// 处理 SMSC（短信中心）
	smsc := ""
	if randomSmsc, ok := params["randomSmsc"].(bool); ok && randomSmsc {
		smsc = generateSMSC(country)
	} else if sm, ok := params["smsc"].(string); ok {
		smsc = sm
	}

	// 构建 Changephones 请求数据
	changeData := map[string]interface{}{
		"deviceId": float64(deviceID),
		"msisdn":   msisdn,
		"smsc":     smsc,
	}

	// 复制其他参数
	for _, key := range []string{"category", "bs", "operator", "timezone", "language", "version", "country", "operatorName", "mcc", "mnc"} {
		if v, ok := params[key].(string); ok && v != "" {
			changeData[key] = v
		}
	}

	// 通过 HandleUnifiedRequestHTTP 发送改机请求
	req := &service.UnifiedRequest{
		Type: "Changephones",
		Seq:  int(time.Now().UnixNano() % 1000000),
		Req:  true,
		Data: []map[string]interface{}{changeData},
	}

	logger.LogInfo("[RPA] 设备 %d 发送改机请求: %+v", deviceID, changeData)

	res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
	if err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("改机请求失败: %v", err),
		}
	}

	if res.Code != 200 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[第1步:改机/发送改机指令] 改机请求失败: %s", res.Msg),
		}
	}

	// 调试：打印返回数据
	resJson, _ := json.Marshal(res.Data)
	logger.LogInfo("[RPA] 设备 %d Changephones 返回: type=%T, data=%s", deviceID, res.Data, string(resJson))

	// 解析返回数据获取 taskId
	// 返回格式: [{id: 1001, deviceId: 12345, status: 0, data: "..."}]
	taskID := float64(0)

	// 尝试直接解析 JSON
	var dataList []map[string]interface{}
	if err := json.Unmarshal(resJson, &dataList); err == nil && len(dataList) > 0 {
		item := dataList[0]
		// 从 id 字段获取
		if id, ok := item["id"].(float64); ok && id > 0 {
			taskID = id
		}
		logger.LogInfo("[RPA] 设备 %d 解析到 id=%v", deviceID, item["id"])
	} else {
		logger.LogInfo("[RPA] 设备 %d JSON解析失败: %v", deviceID, err)
	}

	if taskID == 0 {
		resJson, _ := json.Marshal(res.Data)
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[第1步:改机/发送改机指令] 未获取到改机任务ID，返回数据: %s", string(resJson)),
		}
	}

	logger.LogInfo("[RPA] 设备 %d 改机任务已提交, taskId=%v", deviceID, taskID)

	// 更新上下文，进入下一个子步骤
	newCtx := make(database.StepContext)
	newCtx["taskId"] = taskID
	newCtx["startTime"] = float64(time.Now().Unix())

	return rpa.StepResult{
		Completed: false,
		NextSub:   1,
		Context:   newCtx,
	}
}

// waitChangeOsComplete 等待改机完成（通过 WebSocket 统一接口查询）
func (s *ChangeOsAndWaitStep) waitChangeOsComplete(deviceID int, ctx database.StepContext) rpa.StepResult {
	taskID, _ := ctx["taskId"].(float64)
	if taskID == 0 {
		ctxJson, _ := json.Marshal(ctx)
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[第1步:改机/等待改机完成] 上下文中缺少 taskId, ctx=%s", string(ctxJson)),
		}
	}

	startTime, _ := ctx["startTime"].(float64)
	timeout := 300.0 // 默认超时 300 秒（5分钟）
	if t, ok := ctx["timeout"].(float64); ok {
		timeout = t
	}

	// 检查超时
	if float64(time.Now().Unix())-startTime > timeout {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[第1步:改机/等待改机完成] 改机超时（%.0f秒）", timeout),
		}
	}

	// 通过 HandleUnifiedRequestHTTP 查询改机状态
	logger.LogInfo("[RPA] 设备 %d 查询改机状态, taskId=%d", deviceID, taskID)
	req := &service.UnifiedRequest{
		Type: "getTaskStatus",
		Seq:  int(time.Now().UnixNano() % 1000000),
		Data: map[string]interface{}{
			"tbChangeOsIds": []int64{int64(taskID)},
		},
	}

	res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
	if err != nil {
		logger.LogInfo("[RPA] 设备 %d 查询改机状态失败, taskId=%d, err=%v", deviceID, taskID, err)
		// 查询失败，继续等待重试
		return rpa.StepResult{
			Completed: false,
			NextSub:   1,
			Context:   ctx,
		}
	}

	if res.Code != 200 {
		logger.LogInfo("[RPA] 设备 %d 查询改机状态返回非200, taskId=%d, code=%d", deviceID, taskID, res.Code)
		// 继续等待
		return rpa.StepResult{
			Completed: false,
			NextSub:   1,
			Context:   ctx,
		}
	}

	// 调试：打印返回数据
	resJson, _ := json.Marshal(res.Data)
	logger.LogInfo("[RPA] 设备 %d getTaskStatus 返回, taskId=%d, data=%s", deviceID, taskID, string(resJson))

	// 解析返回数据 - 支持两种类型
	// 返回格式: [{id: 1001, status: 200, data: "..."}]
	var item map[string]interface{}

	// 尝试 []map[string]interface{} 类型
	if dataList, ok := res.Data.([]map[string]interface{}); ok && len(dataList) > 0 {
		item = dataList[0]
	} else if dataList, ok := res.Data.([]interface{}); ok && len(dataList) > 0 {
		// 尝试 []interface{} 类型
		if m, ok := dataList[0].(map[string]interface{}); ok {
			item = m
		}
	}

	if item == nil {
		logger.LogInfo("[RPA] 设备 %d getTaskStatus 返回数据解析失败, taskId=%d, type=%T", deviceID, taskID, res.Data)
		return rpa.StepResult{
			Completed: false,
			NextSub:   1,
			Context:   ctx,
		}
	}

	status := 0
	if s, ok := item["status"].(float64); ok {
		status = int(s)
	} else if s, ok := item["status"].(int); ok {
		status = s
	}

	logger.LogInfo("[RPA] 设备 %d getTaskStatus 解析结果, taskId=%d, status=%d", deviceID, taskID, status)

	// status: 200=成功，其他=进行中或失败
	if status == 200 {
		logger.LogInfo("[RPA] 设备 %d 改机完成，taskId=%d，进入等待设备上线阶段", deviceID, taskID)
		ctx["changeOsCompleteTime"] = float64(time.Now().Unix())
		return rpa.StepResult{
			Completed: false,
			NextSub:   2,
			Context:   ctx,
		}
	}

	// 继续等待
	return rpa.StepResult{
		Completed: false,
		NextSub:   1,
		Context:   ctx,
	}
}

// waitDeviceOnline 等待设备上线
func (s *ChangeOsAndWaitStep) waitDeviceOnline(deviceID int, ctx database.StepContext) rpa.StepResult {
	completeTime, _ := ctx["changeOsCompleteTime"].(float64)
	onlineTimeout := 300.0 // 等待上线超时 300 秒
	if t, ok := ctx["onlineTimeout"].(float64); ok {
		onlineTimeout = t
	}

	// 检查超时
	if float64(time.Now().Unix())-completeTime > onlineTimeout {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[第%d步:改机/等待设备上线] 等待设备上线超时（%.0f秒）", 1, onlineTimeout),
		}
	}

	// 检查是否已经在等待启动完成
	bootWaitStart, hasBootWait := ctx["bootWaitStart"].(float64)
	bootWaitDuration := 180.0 // 等待启动完成 180 秒（3分钟）
	if t, ok := ctx["bootWaitDuration"].(float64); ok {
		bootWaitDuration = t
	}

	if hasBootWait {
		// 已经检测到上线，正在等待启动完成
		elapsed := float64(time.Now().Unix()) - bootWaitStart
		if elapsed < bootWaitDuration {
			logger.LogInfo("[RPA] 设备 %d 已上线，等待启动完成 %.0f/%.0f 秒", deviceID, elapsed, bootWaitDuration)
			return rpa.StepResult{
				Completed: false,
				NextSub:   2,
				Context:   ctx,
			}
		}
		// 等待完成
		logger.LogInfo("[RPA] 设备 %d 启动完成，可以进行下一步", deviceID)
		return rpa.StepResult{
			Completed: true,
			Success:   true,
			Output: map[string]interface{}{
				"deviceId": deviceID,
				"online":   true,
			},
		}
	}

	// 通过 API 接口检查设备是否在线（使用 DeviceInfo.Online 字段）
	online, err := service.CheckDeviceOnline(deviceID)
	if err != nil {
		logger.LogInfo("[RPA] 设备 %d 检查在线状态失败: %v", deviceID, err)
		// 查询失败，继续等待
		return rpa.StepResult{
			Completed: false,
			NextSub:   2,
			Context:   ctx,
		}
	}

	logger.LogInfo("[RPA] 设备 %d 在线状态: online=%v", deviceID, online)

	if online {
		// 检测到上线，开始等待启动完成
		logger.LogInfo("[RPA] 设备 %d 已上线，开始等待启动完成（%.0f秒）", deviceID, bootWaitDuration)
		ctx["bootWaitStart"] = float64(time.Now().Unix())
		return rpa.StepResult{
			Completed: false,
			NextSub:   2,
			Context:   ctx,
		}
	}

	// 继续等待
	return rpa.StepResult{
		Completed: false,
		NextSub:   2,
		Context:   ctx,
	}
}

// 随机生成指定长度的数字字符串
func randomDigits(length int) string {
	result := ""
	for i := 0; i < length; i++ {
		result += fmt.Sprintf("%d", rand.Intn(10))
	}
	return result
}

// 生成随机手机号
func generatePhoneNumber(countryCode string) string {
	switch countryCode {
	case "cn":
		prefixes := []string{"3", "4", "5", "6", "7", "8", "9"}
		prefix := "1" + prefixes[rand.Intn(len(prefixes))]
		return "+86" + prefix + randomDigits(9)
	case "us":
		areaCode := fmt.Sprintf("%d", rand.Intn(8)+2) + randomDigits(2)
		exchangeCode := fmt.Sprintf("%d", rand.Intn(8)+2) + randomDigits(2)
		return "+1" + areaCode + exchangeCode + randomDigits(4)
	case "my":
		if rand.Float32() < 0.3 {
			return "+6011" + randomDigits(8)
		}
		prefixes := []string{"0", "2", "3", "4", "6", "7", "8", "9"}
		return "+601" + prefixes[rand.Intn(len(prefixes))] + randomDigits(7)
	default:
		return ""
	}
}

// 生成随机短信中心号码
func generateSMSC(countryCode string) string {
	switch countryCode {
	case "cn":
		return "+8613800" + randomDigits(3) + "500"
	case "us":
		return generatePhoneNumber("us")
	case "my":
		smscs := []string{"+60120000015", "+60193900000", "+60162999902", "+60183800001", "+601138380000"}
		return smscs[rand.Intn(len(smscs))]
	default:
		return ""
	}
}
