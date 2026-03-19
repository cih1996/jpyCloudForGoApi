package database

import (
	"time"

	"gorm.io/gorm"
)

// ========== DeviceRpaConfig 操作 ==========

// GetDeviceConfig 获取设备配置
func GetDeviceConfig(deviceID int) (*DeviceRpaConfig, error) {
	var config DeviceRpaConfig
	err := db.Where("device_id = ?", deviceID).First(&config).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &config, err
}

// GetOrCreateDeviceConfig 获取或创建设备配置
func GetOrCreateDeviceConfig(deviceID int) (*DeviceRpaConfig, error) {
	var config DeviceRpaConfig
	err := db.Where("device_id = ?", deviceID).First(&config).Error
	if err == gorm.ErrRecordNotFound {
		config = DeviceRpaConfig{
			DeviceID:    deviceID,
			Status:      StatusIdle,
			Mode:        ModeSingle,
			StepContext: make(StepContext),
		}
		if err := db.Create(&config).Error; err != nil {
			return nil, err
		}
		return &config, nil
	}
	return &config, err
}

// UpdateDeviceConfig 更新设备配置
func UpdateDeviceConfig(config *DeviceRpaConfig) error {
	return db.Save(config).Error
}

// SetDeviceRpa 设置设备关联的 RPA
func SetDeviceRpa(deviceID int, rpaID uint, mode RunMode) error {
	config, err := GetOrCreateDeviceConfig(deviceID)
	if err != nil {
		return err
	}
	config.RpaID = rpaID
	config.Mode = mode
	config.Status = StatusIdle
	config.CurrentStep = 0
	config.SubStep = 0
	config.StepContext = make(StepContext)
	config.LoopCount = 0
	config.TotalTime = 0 // 重置总耗时
	config.LoopStartAt = nil
	config.LastError = ""
	return db.Save(config).Error
}

// StartDevice 启动设备执行
func StartDevice(deviceID int) error {
	config, err := GetDeviceConfig(deviceID)
	if err != nil || config == nil {
		return err
	}
	now := time.Now()
	config.Status = StatusRunning
	config.StartedAt = &now
	config.LastActiveAt = &now
	config.LoopStartAt = &now // 记录循环开始时间
	config.CurrentStep = 0
	config.SubStep = 0
	config.StepContext = make(StepContext)
	config.FlowVariables = make(StepContext)
	// 注入设备ID到流程变量，供脚本使用
	config.FlowVariables["deviceId"] = config.DeviceID
	config.LastError = ""
	return db.Save(config).Error
}

// PauseDevice 暂停设备执行
func PauseDevice(deviceID int) error {
	return db.Model(&DeviceRpaConfig{}).
		Where("device_id = ?", deviceID).
		Update("status", StatusPaused).Error
}

// ResumeDevice 恢复设备执行
func ResumeDevice(deviceID int) error {
	now := time.Now()
	return db.Model(&DeviceRpaConfig{}).
		Where("device_id = ?", deviceID).
		Updates(map[string]interface{}{
			"status":         StatusRunning,
			"last_active_at": now,
		}).Error
}

// StopDevice 停止设备执行
func StopDevice(deviceID int) error {
	return db.Model(&DeviceRpaConfig{}).
		Where("device_id = ?", deviceID).
		Updates(map[string]interface{}{
			"status":       StatusIdle,
			"current_step": 0,
			"sub_step":     0,
			"step_context": "{}",
		}).Error
}

// UpdateDeviceProgress 更新设备执行进度
func UpdateDeviceProgress(deviceID int, step, subStep int, context StepContext) error {
	now := time.Now()
	return db.Model(&DeviceRpaConfig{}).
		Where("device_id = ?", deviceID).
		Updates(map[string]interface{}{
			"current_step":   step,
			"sub_step":       subStep,
			"step_context":   context,
			"last_active_at": now,
		}).Error
}

// UpdateFlowVariables 更新流程变量（步骤间传递数据）
func UpdateFlowVariables(deviceID int, vars StepContext) error {
	return db.Model(&DeviceRpaConfig{}).
		Where("device_id = ?", deviceID).
		Update("flow_variables", vars).Error
}

// MergeFlowVariables 合并流程变量（追加新变量）
func MergeFlowVariables(deviceID int, newVars map[string]interface{}) error {
	config, err := GetDeviceConfig(deviceID)
	if err != nil || config == nil {
		return err
	}
	if config.FlowVariables == nil {
		config.FlowVariables = make(StepContext)
	}
	for k, v := range newVars {
		config.FlowVariables[k] = v
	}
	return db.Model(&DeviceRpaConfig{}).
		Where("device_id = ?", deviceID).
		Update("flow_variables", config.FlowVariables).Error
}

// ClearFlowVariables 清空流程变量
func ClearFlowVariables(deviceID int) error {
	return db.Model(&DeviceRpaConfig{}).
		Where("device_id = ?", deviceID).
		Update("flow_variables", "{}").Error
}

// SetDeviceError 设置设备错误状态
func SetDeviceError(deviceID int, errMsg string) error {
	// 先获取当前配置，计算耗时
	config, _ := GetDeviceConfig(deviceID)
	updates := map[string]interface{}{
		"status":     StatusError,
		"last_error": errMsg,
		"fail_count": gorm.Expr("fail_count + 1"),
	}

	// 累加耗时（即使出错也要保留耗时）
	if config != nil && config.LoopStartAt != nil {
		elapsed := int64(time.Now().Sub(*config.LoopStartAt).Seconds())
		updates["total_time"] = config.TotalTime + elapsed
	}

	return db.Model(&DeviceRpaConfig{}).
		Where("device_id = ?", deviceID).
		Updates(updates).Error
}

// SetDeviceCompleted 设置设备完成状态
func SetDeviceCompleted(deviceID int) error {
	config, err := GetDeviceConfig(deviceID)
	if err != nil || config == nil {
		return err
	}

	now := time.Now()

	// 累加最后一次循环的耗时
	if config.LoopStartAt != nil {
		elapsed := int64(now.Sub(*config.LoopStartAt).Seconds())
		config.TotalTime += elapsed
	}

	config.Status = StatusCompleted
	config.SuccessCount++

	return db.Save(config).Error
}

// IncrementLoopCount 增加循环次数并累加耗时
func IncrementLoopCount(deviceID int) error {
	config, err := GetDeviceConfig(deviceID)
	if err != nil || config == nil {
		return err
	}

	now := time.Now()
	config.LoopCount++

	// 计算本次循环耗时并累加
	if config.LoopStartAt != nil {
		elapsed := int64(now.Sub(*config.LoopStartAt).Seconds())
		config.TotalTime += elapsed
	}

	// 重置循环开始时间
	config.LoopStartAt = &now

	return db.Save(config).Error
}

// UpdateScriptFeedback 更新脚本反馈（用于前端展示）
func UpdateScriptFeedback(deviceID int, status string, progress int) error {
	return db.Model(&DeviceRpaConfig{}).
		Where("device_id = ?", deviceID).
		Updates(map[string]interface{}{
			"script_status":   status,
			"script_progress": progress,
		}).Error
}

// GetRunningDevices 获取所有运行中的设备
func GetRunningDevices() ([]DeviceRpaConfig, error) {
	if db == nil {
		return nil, ErrNotInitialized
	}
	var configs []DeviceRpaConfig
	err := db.Where("status = ?", StatusRunning).Find(&configs).Error
	return configs, err
}

// GetAllDeviceConfigs 获取所有设备配置
func GetAllDeviceConfigs() ([]DeviceRpaConfig, error) {
	if db == nil {
		return []DeviceRpaConfig{}, nil // 数据库未初始化时返回空列表
	}
	var configs []DeviceRpaConfig
	err := db.Find(&configs).Error
	return configs, err
}

// ========== RpaFlow 操作 ==========

// CreateRpaFlow 创建 RPA 流程
func CreateRpaFlow(flow *RpaFlow) error {
	return db.Create(flow).Error
}

// GetRpaFlow 获取 RPA 流程
func GetRpaFlow(id uint) (*RpaFlow, error) {
	var flow RpaFlow
	err := db.First(&flow, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &flow, err
}

// UpdateRpaFlow 更新 RPA 流程
func UpdateRpaFlow(flow *RpaFlow) error {
	return db.Save(flow).Error
}

// DeleteRpaFlow 删除 RPA 流程
func DeleteRpaFlow(id uint) error {
	return db.Delete(&RpaFlow{}, id).Error
}

// GetAllRpaFlows 获取所有 RPA 流程
func GetAllRpaFlows() ([]RpaFlow, error) {
	if db == nil {
		return []RpaFlow{}, nil
	}
	var flows []RpaFlow
	err := db.Find(&flows).Error
	return flows, err
}

// GetEnabledRpaFlows 获取所有启用的 RPA 流程
func GetEnabledRpaFlows() ([]RpaFlow, error) {
	var flows []RpaFlow
	err := db.Where("enabled = ?", true).Find(&flows).Error
	return flows, err
}

// ========== RpaLog 操作 ==========

// AddLog 添加日志
func AddLog(deviceID int, rpaID uint, stepIndex, subStep int, level LogLevel, message, context string) error {
	log := RpaLog{
		DeviceID:  deviceID,
		RpaID:     rpaID,
		StepIndex: stepIndex,
		SubStep:   subStep,
		Level:     level,
		Message:   message,
		Context:   context,
	}
	return db.Create(&log).Error
}

// GetDeviceLogs 获取设备日志
func GetDeviceLogs(deviceID int, limit int) ([]RpaLog, error) {
	var logs []RpaLog
	query := db.Where("device_id = ?", deviceID).Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&logs).Error
	return logs, err
}

// GetDeviceLogsByRpa 获取设备某次 RPA 执行的日志
func GetDeviceLogsByRpa(deviceID int, rpaID uint, limit int) ([]RpaLog, error) {
	var logs []RpaLog
	query := db.Where("device_id = ? AND rpa_id = ?", deviceID, rpaID).Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&logs).Error
	return logs, err
}

// ClearDeviceLogs 清除设备日志
func ClearDeviceLogs(deviceID int) error {
	return db.Where("device_id = ?", deviceID).Delete(&RpaLog{}).Error
}

// ClearOldLogs 清除旧日志（保留最近 N 天）
func ClearOldLogs(days int) error {
	cutoff := time.Now().AddDate(0, 0, -days)
	return db.Where("created_at < ?", cutoff).Delete(&RpaLog{}).Error
}

// ========== ScriptRepo 操作 ==========

// CreateScript 创建脚本
func CreateScript(script *ScriptRepo) error {
	return db.Create(script).Error
}

// GetScript 获取脚本
func GetScript(id uint) (*ScriptRepo, error) {
	var script ScriptRepo
	err := db.First(&script, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &script, err
}

// UpdateScript 更新脚本（只更新指定字段，保留 createdAt，返回完整数据）
func UpdateScript(script *ScriptRepo) error {
	err := db.Model(&ScriptRepo{}).Where("id = ?", script.ID).Updates(map[string]interface{}{
		"name":        script.Name,
		"description": script.Description,
		"code":        script.Code,
		"timeout":     script.Timeout,
	}).Error
	if err != nil {
		return err
	}
	// 重新查询完整数据
	return db.First(script, script.ID).Error
}

// DeleteScript 删除脚本
func DeleteScript(id uint) error {
	return db.Delete(&ScriptRepo{}, id).Error
}

// GetAllScripts 获取所有脚本
func GetAllScripts() ([]ScriptRepo, error) {
	var scripts []ScriptRepo
	err := db.Order("updated_at DESC").Find(&scripts).Error
	return scripts, err
}

// SearchScripts 搜索脚本
func SearchScripts(keyword string) ([]ScriptRepo, error) {
	var scripts []ScriptRepo
	err := db.Where("name LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%").
		Order("updated_at DESC").Find(&scripts).Error
	return scripts, err
}

// ========== RpaExecutionHistory 操作 ==========

// CreateExecutionHistory 创建执行历史记录
func CreateExecutionHistory(deviceID int, rpaID uint, rpaName string, totalSteps int) (*RpaExecutionHistory, error) {
	history := &RpaExecutionHistory{
		DeviceID:   deviceID,
		RpaID:      rpaID,
		RpaName:    rpaName,
		Status:     ExecStatusRunning,
		TotalSteps: totalSteps,
	}
	err := db.Create(history).Error
	return history, err
}

// UpdateExecutionProgress 更新执行进度
func UpdateExecutionProgress(historyID uint, currentStep, currentSubStep int) error {
	return db.Model(&RpaExecutionHistory{}).
		Where("id = ?", historyID).
		Updates(map[string]interface{}{
			"current_step":     currentStep,
			"current_sub_step": currentSubStep,
		}).Error
}

// CompleteExecution 完成执行
func CompleteExecution(historyID uint, status ExecutionStatus, successSteps, failedSteps int, errorMsg string) error {
	now := time.Now()

	// 先获取记录计算耗时
	var history RpaExecutionHistory
	if err := db.First(&history, historyID).Error; err != nil {
		return err
	}

	duration := int64(now.Sub(history.StartedAt).Seconds())

	return db.Model(&RpaExecutionHistory{}).
		Where("id = ?", historyID).
		Updates(map[string]interface{}{
			"status":        status,
			"success_steps": successSteps,
			"failed_steps":  failedSteps,
			"error_message": errorMsg,
			"completed_at":  now,
			"duration":      duration,
		}).Error
}

// GetExecutionHistory 获取单条执行历史
func GetExecutionHistory(id uint) (*RpaExecutionHistory, error) {
	var history RpaExecutionHistory
	err := db.First(&history, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &history, err
}

// GetDeviceExecutionHistory 获取设备的执行历史
func GetDeviceExecutionHistory(deviceID int, limit int) ([]RpaExecutionHistory, error) {
	var histories []RpaExecutionHistory
	query := db.Where("device_id = ?", deviceID).Order("started_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&histories).Error
	return histories, err
}

// GetAllExecutionHistory 获取所有执行历史
func GetAllExecutionHistory(limit int) ([]RpaExecutionHistory, error) {
	var histories []RpaExecutionHistory
	query := db.Order("started_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&histories).Error
	return histories, err
}

// GetFilteredExecutionHistory 带筛选的执行历史查询
func GetFilteredExecutionHistory(limit, deviceID int, rpaID uint, status string) ([]RpaExecutionHistory, error) {
	var histories []RpaExecutionHistory
	query := db.Order("started_at DESC")
	if deviceID > 0 {
		query = query.Where("device_id = ?", deviceID)
	}
	if rpaID > 0 {
		query = query.Where("rpa_id = ?", rpaID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&histories).Error
	return histories, err
}

// GetRunningExecutions 获取正在运行的执行记录
func GetRunningExecutions() ([]RpaExecutionHistory, error) {
	var histories []RpaExecutionHistory
	err := db.Where("status = ?", ExecStatusRunning).Find(&histories).Error
	return histories, err
}

// ClearOldExecutionHistory 清除旧的执行历史（保留最近 N 天）
func ClearOldExecutionHistory(days int) error {
	cutoff := time.Now().AddDate(0, 0, -days)
	// 先删除关联的步骤明细
	db.Where("history_id IN (SELECT id FROM rpa_execution_history WHERE started_at < ?)", cutoff).Delete(&RpaExecutionStep{})
	return db.Where("started_at < ?", cutoff).Delete(&RpaExecutionHistory{}).Error
}

// ========== RpaExecutionStep 操作 ==========

// CreateExecutionStep 创建步骤明细
func CreateExecutionStep(historyID uint, deviceID int, stepIndex int, stepName, stepType string) (*RpaExecutionStep, error) {
	step := &RpaExecutionStep{
		HistoryID: historyID,
		DeviceID:  deviceID,
		StepIndex: stepIndex,
		StepName:  stepName,
		StepType:  stepType,
		Status:    ExecStatusRunning,
	}
	err := db.Create(step).Error
	return step, err
}

// CompleteExecutionStep 完成步骤
func CompleteExecutionStep(stepID uint, status ExecutionStatus, errorMsg string) error {
	now := time.Now()
	var step RpaExecutionStep
	if err := db.First(&step, stepID).Error; err != nil {
		return err
	}
	duration := now.Sub(step.StartedAt).Milliseconds()
	return db.Model(&RpaExecutionStep{}).Where("id = ?", stepID).Updates(map[string]interface{}{
		"status":        status,
		"error_message": errorMsg,
		"completed_at":  now,
		"duration":      duration,
	}).Error
}

// GetExecutionSteps 获取执行历史的步骤明细
func GetExecutionSteps(historyID uint) ([]RpaExecutionStep, error) {
	var steps []RpaExecutionStep
	err := db.Where("history_id = ?", historyID).Order("step_index ASC, started_at ASC").Find(&steps).Error
	return steps, err
}

// GetLatestRunningHistory 获取设备最新的运行中执行记录
func GetLatestRunningHistory(deviceID int) (*RpaExecutionHistory, error) {
	var history RpaExecutionHistory
	err := db.Where("device_id = ? AND status = ?", deviceID, ExecStatusRunning).
		Order("started_at DESC").First(&history).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &history, err
}

// CancelRunningExecutions 取消设备所有运行中的执行记录
func CancelRunningExecutions(deviceID int) error {
	now := time.Now()
	return db.Model(&RpaExecutionHistory{}).
		Where("device_id = ? AND status = ?", deviceID, ExecStatusRunning).
		Updates(map[string]interface{}{
			"status":       ExecStatusCancelled,
			"completed_at": now,
		}).Error
}
