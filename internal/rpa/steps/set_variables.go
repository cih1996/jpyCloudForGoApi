package steps

import (
	"fmt"
	"port-mapping-demo/internal/database"
	"port-mapping-demo/internal/rpa"
	"port-mapping-demo/pkg/logger"
)

// SetVariablesStep 设置流程变量步骤
// 用于在流程中设置变量，供后续步骤使用
type SetVariablesStep struct{}

func init() {
	rpa.RegisterStep(&SetVariablesStep{})
}

func (s *SetVariablesStep) Type() string {
	return "set_variables"
}

func (s *SetVariablesStep) Name() string {
	return "设置变量"
}

func (s *SetVariablesStep) SubSteps() []string {
	return []string{"设置变量"}
}

func (s *SetVariablesStep) Execute(deviceID int, params map[string]interface{}, subStep int, ctx database.StepContext) rpa.StepResult {
	// 获取要设置的变量
	variables, ok := params["variables"].(map[string]interface{})
	if !ok || len(variables) == 0 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "缺少必要参数: variables（变量映射）",
		}
	}

	// 合并变量到流程变量
	if err := database.MergeFlowVariables(deviceID, variables); err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("设置变量失败: %v", err),
		}
	}

	// 记录设置的变量
	varNames := make([]string, 0, len(variables))
	for k := range variables {
		varNames = append(varNames, k)
	}
	logger.LogInfo("[RPA] 设备 %d 设置变量: %v", deviceID, varNames)

	return rpa.StepResult{
		Completed: true,
		Success:   true,
		Output:    variables,
	}
}
