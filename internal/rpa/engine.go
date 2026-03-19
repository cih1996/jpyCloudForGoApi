package rpa

import (
	"encoding/json"
	"fmt"
	"port-mapping-demo/internal/database"
	"port-mapping-demo/pkg/logger"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Engine RPA 执行引擎
type Engine struct {
	running    bool
	stopCh     chan struct{}
	wg         sync.WaitGroup
	interval   time.Duration // 轮询间隔
	mu         sync.RWMutex
	onProgress func(deviceID int, status EngineStatus) // 进度回调

	// 执行记录追踪
	historyMap   map[int]uint // deviceID -> historyID
	stepMap      map[string]uint // "deviceID:stepIndex" -> stepID
	historyMu    sync.Mutex
}

// EngineStatus 引擎状态（用于前端展示）
type EngineStatus struct {
	DeviceID       int        `json:"deviceId"`
	RpaID          uint       `json:"rpaId"`
	RpaName        string     `json:"rpaName"`
	Status         string     `json:"status"`
	CurrentStep    int        `json:"currentStep"`
	TotalSteps     int        `json:"totalSteps"`
	StepName       string     `json:"stepName"`
	SubStep        int        `json:"subStep"`
	SubStepName    string     `json:"subStepName"`
	LoopCount      int        `json:"loopCount"`
	TotalTime      int64      `json:"totalTime"`
	LoopStartAt    *time.Time `json:"loopStartAt"`
	ScriptStatus   string     `json:"scriptStatus"`
	ScriptProgress int        `json:"scriptProgress"`
	LastError      string     `json:"lastError"`
}

var (
	engineInstance *Engine
	engineOnce     sync.Once
)

// GetEngine 获取引擎单例
func GetEngine() *Engine {
	engineOnce.Do(func() {
		engineInstance = &Engine{
			interval:   2 * time.Second,
			stopCh:     make(chan struct{}),
			historyMap: make(map[int]uint),
			stepMap:    make(map[string]uint),
		}
	})
	return engineInstance
}

// SetProgressCallback 设置进度回调
func (e *Engine) SetProgressCallback(fn func(deviceID int, status EngineStatus)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onProgress = fn
}

// SetInterval 设置轮询间隔
func (e *Engine) SetInterval(d time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.interval = d
}

// ========== 执行记录追踪 ==========

func (e *Engine) historyKey(deviceID, stepIndex int) string {
	return fmt.Sprintf("%d:%d", deviceID, stepIndex)
}

// StartHistory 创建执行记录
func (e *Engine) StartHistory(deviceID int, rpaID uint, rpaName string, totalSteps int) {
	e.historyMu.Lock()
	defer e.historyMu.Unlock()

	history, err := database.CreateExecutionHistory(deviceID, rpaID, rpaName, totalSteps)
	if err != nil {
		logger.LogError("[RPA History] 创建执行记录失败: %v", err)
		return
	}
	e.historyMap[deviceID] = history.ID
	logger.LogInfo("[RPA History] 设备 %d 创建执行记录 #%d", deviceID, history.ID)
}

// CompleteHistory 完成执行记录
func (e *Engine) CompleteHistory(deviceID int, status database.ExecutionStatus, errorMsg string) {
	e.historyMu.Lock()
	defer e.historyMu.Unlock()

	historyID, ok := e.historyMap[deviceID]
	if !ok {
		return
	}

	// 统计步骤成功/失败数
	steps, _ := database.GetExecutionSteps(historyID)
	successCount, failCount := 0, 0
	for _, s := range steps {
		if s.Status == database.ExecStatusSuccess {
			successCount++
		} else if s.Status == database.ExecStatusFailed {
			failCount++
		}
	}

	database.CompleteExecution(historyID, status, successCount, failCount, errorMsg)
	delete(e.historyMap, deviceID)
	logger.LogInfo("[RPA History] 设备 %d 执行记录 #%d 完成: %s", deviceID, historyID, status)
}

// StartStepRecord 记录步骤开始
func (e *Engine) StartStepRecord(deviceID, stepIndex int, stepName, stepType string) {
	e.historyMu.Lock()
	defer e.historyMu.Unlock()

	historyID, ok := e.historyMap[deviceID]
	if !ok {
		return
	}

	step, err := database.CreateExecutionStep(historyID, deviceID, stepIndex, stepName, stepType)
	if err != nil {
		logger.LogError("[RPA History] 创建步骤记录失败: %v", err)
		return
	}
	e.stepMap[e.historyKey(deviceID, stepIndex)] = step.ID
}

// CompleteStepRecord 记录步骤完成
func (e *Engine) CompleteStepRecord(deviceID, stepIndex int, status database.ExecutionStatus, errorMsg string) {
	e.historyMu.Lock()
	defer e.historyMu.Unlock()

	key := e.historyKey(deviceID, stepIndex)
	stepID, ok := e.stepMap[key]
	if !ok {
		return
	}

	database.CompleteExecutionStep(stepID, status, errorMsg)
	delete(e.stepMap, key)

	// 同步更新 history 的进度
	if historyID, ok := e.historyMap[deviceID]; ok {
		database.UpdateExecutionProgress(historyID, stepIndex, 0)
	}
}

// Start 启动引擎
func (e *Engine) Start() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.stopCh = make(chan struct{})
	e.mu.Unlock()

	e.wg.Add(1)
	go e.loop()
	logger.LogInfo("[RPA Engine] 启动成功，轮询间隔: %v", e.interval)
}

// Stop 停止引擎
func (e *Engine) Stop() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}
	e.running = false
	close(e.stopCh)
	e.mu.Unlock()

	e.wg.Wait()
	logger.LogInfo("[RPA Engine] 已停止")
}

// IsRunning 检查引擎是否运行中
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// loop 主循环
func (e *Engine) loop() {
	defer e.wg.Done()

	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.tick()
		}
	}
}

// tick 单次轮询
func (e *Engine) tick() {
	// 检查数据库是否已初始化
	if !database.IsInitialized() {
		return
	}

	// 获取所有运行中的设备
	configs, err := database.GetRunningDevices()
	if err != nil {
		logger.LogError("[RPA Engine] 获取运行中设备失败: %v", err)
		return
	}

	for _, config := range configs {
		e.processDevice(&config)
	}
}

// processDevice 处理单个设备
func (e *Engine) processDevice(config *database.DeviceRpaConfig) {
	// 获取 RPA 流程
	flow, err := database.GetRpaFlow(config.RpaID)
	if err != nil || flow == nil {
		logger.LogError("[RPA Engine] 设备 %d 的 RPA 流程 %d 不存在", config.DeviceID, config.RpaID)
		database.SetDeviceError(config.DeviceID, "RPA 流程不存在")
		return
	}

	if len(flow.Steps) == 0 {
		logger.LogInfo("[RPA Engine] 设备 %d 的 RPA 流程 %d 没有步骤", config.DeviceID, config.RpaID)
		database.SetDeviceCompleted(config.DeviceID)
		return
	}

	// 检查是否已完成所有步骤
	if config.CurrentStep >= len(flow.Steps) {
		e.handleFlowComplete(config, flow)
		return
	}

	// 获取当前步骤
	step := flow.Steps[config.CurrentStep]
	executor := GetStepExecutor(step.Type)
	if executor == nil {
		errMsg := fmt.Sprintf("未知步骤类型: %s", step.Type)
		logger.LogError("[RPA Engine] 设备 %d: %s", config.DeviceID, errMsg)
		database.SetDeviceError(config.DeviceID, errMsg)
		database.AddLog(config.DeviceID, config.RpaID, config.CurrentStep, config.SubStep, database.LogError, errMsg, "")
		return
	}

	// 解析参数中的变量引用 {{varName}}
	resolvedParams := e.resolveParams(step.Params, config.FlowVariables, config.DeviceID)

	// 记录步骤开始（仅子步骤 0 时记录，避免重复）
	if config.SubStep == 0 {
		e.StartStepRecord(config.DeviceID, config.CurrentStep, step.Name, step.Type)
	}

	// 执行步骤
	result := executor.Execute(config.DeviceID, resolvedParams, config.SubStep, config.StepContext)

	// 处理执行结果
	e.handleStepResult(config, flow, &step, executor, result)
}

// resolveParams 解析参数中的变量引用
// 支持 {{varName}} 格式引用流程变量
// 内置变量: {{deviceId}}
func (e *Engine) resolveParams(params map[string]interface{}, flowVars database.StepContext, deviceID int) map[string]interface{} {
	if params == nil {
		return params
	}

	// 合并内置变量
	allVars := make(map[string]interface{})
	if flowVars != nil {
		for k, v := range flowVars {
			allVars[k] = v
		}
	}
	allVars["deviceId"] = deviceID

	resolved := make(map[string]interface{})
	for k, v := range params {
		resolved[k] = e.resolveValue(v, allVars)
	}
	return resolved
}

// resolveValue 递归解析值中的变量引用
func (e *Engine) resolveValue(value interface{}, vars map[string]interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return e.resolveString(v, vars)
	case map[string]interface{}:
		resolved := make(map[string]interface{})
		for k, val := range v {
			resolved[k] = e.resolveValue(val, vars)
		}
		return resolved
	case []interface{}:
		resolved := make([]interface{}, len(v))
		for i, val := range v {
			resolved[i] = e.resolveValue(val, vars)
		}
		return resolved
	default:
		return value
	}
}

// resolveString 解析字符串中的变量引用
func (e *Engine) resolveString(s string, vars map[string]interface{}) interface{} {
	// 匹配 {{varName}} 或 {{varName.subKey}}
	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)

	// 如果整个字符串就是一个变量引用，直接返回变量值（保持类型）
	if matches := re.FindStringSubmatch(s); len(matches) == 2 && matches[0] == s {
		return e.getVarValue(matches[1], vars)
	}

	// 否则进行字符串替换
	result := re.ReplaceAllStringFunc(s, func(match string) string {
		varName := strings.Trim(match, "{}")
		val := e.getVarValue(varName, vars)
		if val == nil {
			return match // 保留原样
		}
		return fmt.Sprintf("%v", val)
	})
	return result
}

// getVarValue 获取变量值，支持点号访问嵌套属性
func (e *Engine) getVarValue(path string, vars map[string]interface{}) interface{} {
	parts := strings.Split(path, ".")
	var current interface{} = vars

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if m, ok := current.(map[string]interface{}); ok {
			current = m[part]
		} else {
			return nil
		}
	}
	return current
}

// handleStepResult 处理步骤执行结果
func (e *Engine) handleStepResult(config *database.DeviceRpaConfig, flow *database.RpaFlow, step *database.RpaStep, executor StepExecutor, result StepResult) {
	if result.Completed {
		if result.Success {
			// 步骤成功，进入下一步
			logger.LogInfo("[RPA Engine] 设备 %d 步骤 %d (%s) 完成", config.DeviceID, config.CurrentStep, step.Name)
			database.AddLog(config.DeviceID, config.RpaID, config.CurrentStep, config.SubStep, database.LogSuccess, fmt.Sprintf("步骤完成: %s", step.Name), "")

			// 记录步骤完成
			e.CompleteStepRecord(config.DeviceID, config.CurrentStep, database.ExecStatusSuccess, "")

			// 保存步骤输出到流程变量（排除控制指令）
			if result.Output != nil && len(result.Output) > 0 {
				// 复制输出，排除控制指令
				outputToSave := make(map[string]interface{})
				for k, v := range result.Output {
					if k != "skipNext" && k != "jumpToStep" && k != "stopFlow" && k != "stopSuccess" {
						outputToSave[k] = v
					}
				}
				if len(outputToSave) > 0 {
					database.MergeFlowVariables(config.DeviceID, outputToSave)
					logger.LogInfo("[RPA Engine] 设备 %d 保存步骤输出: %v", config.DeviceID, outputToSave)
				}
			}

			// 处理条件判断步骤的特殊控制指令
			nextStep := config.CurrentStep + 1

			if result.Output != nil {
				// 检查是否需要停止流程
				if stopFlow, ok := result.Output["stopFlow"].(bool); ok && stopFlow {
					if stopSuccess, ok := result.Output["stopSuccess"].(bool); ok && stopSuccess {
						// 成功停止
						database.SetDeviceCompleted(config.DeviceID)
						logger.LogInfo("[RPA Engine] 设备 %d 条件判断触发成功停止", config.DeviceID)
						database.AddLog(config.DeviceID, config.RpaID, config.CurrentStep, config.SubStep, database.LogSuccess, "条件判断触发流程成功停止", "")
						e.notifyProgress(config.DeviceID, flow, len(flow.Steps), 0, "")
						return
					}
				}

				// 检查是否需要跳转到指定步骤
				if jumpTo, ok := result.Output["jumpToStep"].(int); ok {
					if jumpTo >= 0 && jumpTo < len(flow.Steps) {
						nextStep = jumpTo
						logger.LogInfo("[RPA Engine] 设备 %d 条件判断跳转到步骤 %d", config.DeviceID, jumpTo)
						database.AddLog(config.DeviceID, config.RpaID, config.CurrentStep, config.SubStep, database.LogInfo, fmt.Sprintf("条件判断跳转到步骤 %d", jumpTo), "")
					}
				} else if jumpToFloat, ok := result.Output["jumpToStep"].(float64); ok {
					jumpTo := int(jumpToFloat)
					if jumpTo >= 0 && jumpTo < len(flow.Steps) {
						nextStep = jumpTo
						logger.LogInfo("[RPA Engine] 设备 %d 条件判断跳转到步骤 %d", config.DeviceID, jumpTo)
						database.AddLog(config.DeviceID, config.RpaID, config.CurrentStep, config.SubStep, database.LogInfo, fmt.Sprintf("条件判断跳转到步骤 %d", jumpTo), "")
					}
				}

				// 检查是否需要跳过下一步
				if skipNext, ok := result.Output["skipNext"].(bool); ok && skipNext {
					nextStep = config.CurrentStep + 2
					logger.LogInfo("[RPA Engine] 设备 %d 条件判断跳过下一步", config.DeviceID)
					database.AddLog(config.DeviceID, config.RpaID, config.CurrentStep, config.SubStep, database.LogInfo, "条件判断跳过下一步", "")
				}
			}

			// 更新进度到下一步
			database.UpdateDeviceProgress(config.DeviceID, nextStep, 0, make(database.StepContext))

			// 通知进度
			e.notifyProgress(config.DeviceID, flow, nextStep, 0, "")
		} else {
			// 步骤失败
			logger.LogError("[RPA Engine] 设备 %d 步骤 %d (%s) 失败: %s", config.DeviceID, config.CurrentStep, step.Name, result.Error)
			database.SetDeviceError(config.DeviceID, result.Error)
			database.AddLog(config.DeviceID, config.RpaID, config.CurrentStep, config.SubStep, database.LogError, result.Error, "")

			// 记录步骤失败 + 完成执行记录
			e.CompleteStepRecord(config.DeviceID, config.CurrentStep, database.ExecStatusFailed, result.Error)
			e.CompleteHistory(config.DeviceID, database.ExecStatusFailed, result.Error)

			// 通知进度
			e.notifyProgress(config.DeviceID, flow, config.CurrentStep, config.SubStep, result.Error)
		}
	} else {
		// 步骤未完成，更新子步骤
		ctx := result.Context
		if ctx == nil {
			ctx = config.StepContext
		}
		database.UpdateDeviceProgress(config.DeviceID, config.CurrentStep, result.NextSub, ctx)

		// 记录子步骤进度
		subSteps := executor.SubSteps()
		subStepName := ""
		if result.NextSub < len(subSteps) {
			subStepName = subSteps[result.NextSub]
		}
		logger.LogInfo("[RPA Engine] 设备 %d 步骤 %d 子步骤 %d (%s)", config.DeviceID, config.CurrentStep, result.NextSub, subStepName)

		// 通知进度
		e.notifyProgress(config.DeviceID, flow, config.CurrentStep, result.NextSub, "")
	}
}

// handleFlowComplete 处理流程完成
func (e *Engine) handleFlowComplete(config *database.DeviceRpaConfig, flow *database.RpaFlow) {
	if config.Mode == database.ModeLoop {
		// 循环模式：完成本轮记录，重新开始
		e.CompleteHistory(config.DeviceID, database.ExecStatusSuccess, "")

		database.IncrementLoopCount(config.DeviceID)
		database.UpdateDeviceProgress(config.DeviceID, 0, 0, make(database.StepContext))
		logger.LogInfo("[RPA Engine] 设备 %d 完成一轮，开始第 %d 轮", config.DeviceID, config.LoopCount+1)
		database.AddLog(config.DeviceID, config.RpaID, -1, -1, database.LogInfo, fmt.Sprintf("完成第 %d 轮，开始下一轮", config.LoopCount+1), "")

		// 为下一轮创建新的执行记录
		e.StartHistory(config.DeviceID, config.RpaID, flow.Name, len(flow.Steps))
	} else {
		// 单次模式，标记完成
		e.CompleteHistory(config.DeviceID, database.ExecStatusSuccess, "")

		database.SetDeviceCompleted(config.DeviceID)
		logger.LogInfo("[RPA Engine] 设备 %d 流程执行完成", config.DeviceID)
		database.AddLog(config.DeviceID, config.RpaID, -1, -1, database.LogSuccess, "流程执行完成", "")
	}

	e.notifyProgress(config.DeviceID, flow, len(flow.Steps), 0, "")
}

// notifyProgress 通知进度更新
func (e *Engine) notifyProgress(deviceID int, flow *database.RpaFlow, currentStep, subStep int, lastError string) {
	e.mu.RLock()
	callback := e.onProgress
	e.mu.RUnlock()

	if callback == nil {
		return
	}

	// 构建状态
	status := EngineStatus{
		DeviceID:    deviceID,
		RpaID:       flow.ID,
		RpaName:     flow.Name,
		CurrentStep: currentStep,
		TotalSteps:  len(flow.Steps),
		SubStep:     subStep,
		LastError:   lastError,
	}

	// 获取当前步骤名称
	if currentStep < len(flow.Steps) {
		step := flow.Steps[currentStep]
		status.StepName = step.Name

		// 获取子步骤名称
		executor := GetStepExecutor(step.Type)
		if executor != nil {
			subSteps := executor.SubSteps()
			if subStep < len(subSteps) {
				status.SubStepName = subSteps[subStep]
			}
		}
	}

	// 获取设备配置
	config, _ := database.GetDeviceConfig(deviceID)
	if config != nil {
		status.Status = string(config.Status)
		status.LoopCount = config.LoopCount
		status.ScriptStatus = config.ScriptStatus
		status.ScriptProgress = config.ScriptProgress
	}

	callback(deviceID, status)
}

// GetDeviceStatus 获取设备状态（供外部查询）
func (e *Engine) GetDeviceStatus(deviceID int) (*EngineStatus, error) {
	config, err := database.GetDeviceConfig(deviceID)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, fmt.Errorf("设备 %d 未配置", deviceID)
	}

	status := &EngineStatus{
		DeviceID:       deviceID,
		RpaID:          config.RpaID,
		Status:         string(config.Status),
		CurrentStep:    config.CurrentStep,
		SubStep:        config.SubStep,
		LoopCount:      config.LoopCount,
		TotalTime:      config.TotalTime,
		LoopStartAt:    config.LoopStartAt,
		ScriptStatus:   config.ScriptStatus,
		ScriptProgress: config.ScriptProgress,
		LastError:      config.LastError,
	}

	// 获取 RPA 流程信息
	if config.RpaID > 0 {
		flow, _ := database.GetRpaFlow(config.RpaID)
		if flow != nil {
			status.RpaName = flow.Name
			status.TotalSteps = len(flow.Steps)

			if config.CurrentStep < len(flow.Steps) {
				step := flow.Steps[config.CurrentStep]
				status.StepName = step.Name

				executor := GetStepExecutor(step.Type)
				if executor != nil {
					subSteps := executor.SubSteps()
					if config.SubStep < len(subSteps) {
						status.SubStepName = subSteps[config.SubStep]
					}
				}
			}
		}
	}

	return status, nil
}

// GetAllDeviceStatus 获取所有设备状态
func (e *Engine) GetAllDeviceStatus() ([]EngineStatus, error) {
	configs, err := database.GetAllDeviceConfigs()
	if err != nil {
		return nil, err
	}

	// 预加载所有 RPA 流程
	flows, _ := database.GetAllRpaFlows()
	flowMap := make(map[uint]*database.RpaFlow)
	for i := range flows {
		flowMap[flows[i].ID] = &flows[i]
	}

	var statuses []EngineStatus
	for _, config := range configs {
		status := EngineStatus{
			DeviceID:       config.DeviceID,
			RpaID:          config.RpaID,
			Status:         string(config.Status),
			CurrentStep:    config.CurrentStep,
			SubStep:        config.SubStep,
			LoopCount:      config.LoopCount,
			TotalTime:      config.TotalTime,
			LoopStartAt:    config.LoopStartAt,
			ScriptStatus:   config.ScriptStatus,
			ScriptProgress: config.ScriptProgress,
			LastError:      config.LastError,
		}

		if flow, ok := flowMap[config.RpaID]; ok {
			status.RpaName = flow.Name
			status.TotalSteps = len(flow.Steps)

			if config.CurrentStep < len(flow.Steps) {
				step := flow.Steps[config.CurrentStep]
				status.StepName = step.Name

				executor := GetStepExecutor(step.Type)
				if executor != nil {
					subSteps := executor.SubSteps()
					if config.SubStep < len(subSteps) {
						status.SubStepName = subSteps[config.SubStep]
					}
				}
			}
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}

// UpdateScriptFeedback 更新脚本反馈（供脚本回调使用）
func UpdateScriptFeedback(deviceID int, status string, progress int) error {
	return database.UpdateScriptFeedback(deviceID, status, progress)
}

// ExportFlowJSON 导出流程为 JSON
func ExportFlowJSON(flowID uint) (string, error) {
	flow, err := database.GetRpaFlow(flowID)
	if err != nil || flow == nil {
		return "", fmt.Errorf("流程不存在")
	}
	data, err := json.MarshalIndent(flow, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ImportFlowJSON 从 JSON 导入流程
func ImportFlowJSON(jsonStr string) (*database.RpaFlow, error) {
	var flow database.RpaFlow
	if err := json.Unmarshal([]byte(jsonStr), &flow); err != nil {
		return nil, err
	}
	flow.ID = 0 // 清除 ID，创建新记录
	if err := database.CreateRpaFlow(&flow); err != nil {
		return nil, err
	}
	return &flow, nil
}
