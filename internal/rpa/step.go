package rpa

import (
	"port-mapping-demo/internal/database"
)

// StepResult 步骤执行结果
type StepResult struct {
	Completed bool                   // 是否完成（true=完成，false=需要继续轮询）
	Success   bool                   // 是否成功（仅 Completed=true 时有效）
	Error     string                 // 错误信息
	NextSub   int                    // 下一个子步骤（仅 Completed=false 时有效）
	Context   database.StepContext   // 更新后的上下文
	Output    map[string]interface{} // 输出变量
}

// StepExecutor 步骤执行器接口
// 每个模块化步骤都需要实现这个接口
type StepExecutor interface {
	// Type 返回步骤类型标识
	Type() string

	// Name 返回步骤显示名称
	Name() string

	// SubSteps 返回子步骤列表（用于进度展示）
	SubSteps() []string

	// Execute 执行步骤
	// deviceID: 设备ID
	// params: 步骤参数
	// subStep: 当前子步骤索引
	// context: 步骤上下文（存储中间状态）
	// 返回: 执行结果
	Execute(deviceID int, params map[string]interface{}, subStep int, context database.StepContext) StepResult
}

// 步骤注册表
var stepRegistry = make(map[string]StepExecutor)

// RegisterStep 注册步骤执行器
func RegisterStep(executor StepExecutor) {
	stepRegistry[executor.Type()] = executor
}

// GetStepExecutor 获取步骤执行器
func GetStepExecutor(stepType string) StepExecutor {
	return stepRegistry[stepType]
}

// GetAllStepTypes 获取所有已注册的步骤类型
func GetAllStepTypes() []string {
	types := make([]string, 0, len(stepRegistry))
	for t := range stepRegistry {
		types = append(types, t)
	}
	return types
}

// StepInfo 步骤信息（用于前端展示）
type StepInfo struct {
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	SubSteps    []string `json:"subSteps"`
	Description string   `json:"description"`
}

// GetStepInfoList 获取所有步骤信息列表
func GetStepInfoList() []StepInfo {
	list := make([]StepInfo, 0, len(stepRegistry))
	for _, executor := range stepRegistry {
		list = append(list, StepInfo{
			Type:     executor.Type(),
			Name:     executor.Name(),
			SubSteps: executor.SubSteps(),
		})
	}
	return list
}
