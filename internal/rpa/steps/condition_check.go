package steps

import (
	"fmt"
	"port-mapping-demo/internal/database"
	"port-mapping-demo/internal/rpa"
	"port-mapping-demo/pkg/logger"
	"strconv"
	"strings"
)

// ConditionCheckStep 条件判断步骤
// 根据条件判断结果决定流程走向
type ConditionCheckStep struct{}

func init() {
	rpa.RegisterStep(&ConditionCheckStep{})
}

func (s *ConditionCheckStep) Type() string {
	return "condition_check"
}

func (s *ConditionCheckStep) Name() string {
	return "条件判断"
}

func (s *ConditionCheckStep) SubSteps() []string {
	return []string{"评估条件"}
}

// Execute 执行条件判断
// 参数说明:
//   - variable: 要检查的变量名（从流程变量中获取）
//   - operator: 比较运算符 (eq, ne, gt, lt, gte, lte, contains, not_contains, empty, not_empty)
//   - value: 比较值（可选，empty/not_empty 不需要）
//   - onTrue: 条件为真时的动作 (continue, skip_next, jump_to_step, stop_success, stop_error)
//   - onFalse: 条件为假时的动作 (continue, skip_next, jump_to_step, stop_success, stop_error)
//   - jumpStepTrue: onTrue=jump_to_step 时跳转的步骤索引
//   - jumpStepFalse: onFalse=jump_to_step 时跳转的步骤索引
func (s *ConditionCheckStep) Execute(deviceID int, params map[string]interface{}, subStep int, ctx database.StepContext) rpa.StepResult {
	// 获取参数
	variable, _ := params["variable"].(string)
	operator, _ := params["operator"].(string)
	compareValue := params["value"]
	onTrue, _ := params["onTrue"].(string)
	onFalse, _ := params["onFalse"].(string)

	if variable == "" {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "缺少必要参数: variable（变量名）",
		}
	}
	if operator == "" {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "缺少必要参数: operator（运算符）",
		}
	}

	// 默认动作
	if onTrue == "" {
		onTrue = "continue"
	}
	if onFalse == "" {
		onFalse = "continue"
	}

	// 获取设备配置以读取流程变量
	config, err := database.GetDeviceConfig(deviceID)
	if err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("获取设备配置失败: %v", err),
		}
	}

	// 从流程变量中获取值
	var actualValue interface{}
	if config != nil && config.FlowVariables != nil {
		actualValue = config.FlowVariables[variable]
	}

	// 执行条件判断
	conditionMet := evaluateCondition(actualValue, operator, compareValue)

	logger.LogInfo("[RPA] 设备 %d 条件判断: %s %s %v = %v (实际值: %v)",
		deviceID, variable, operator, compareValue, conditionMet, actualValue)

	// 根据条件结果决定动作
	var action string
	var jumpStep int
	if conditionMet {
		action = onTrue
		if js, ok := params["jumpStepTrue"].(float64); ok {
			jumpStep = int(js)
		}
	} else {
		action = onFalse
		if js, ok := params["jumpStepFalse"].(float64); ok {
			jumpStep = int(js)
		}
	}

	// 执行动作
	return executeAction(action, jumpStep, conditionMet, variable, actualValue)
}

// evaluateCondition 评估条件
func evaluateCondition(actual interface{}, operator string, expected interface{}) bool {
	switch operator {
	case "eq", "==", "equals":
		return compareEqual(actual, expected)
	case "ne", "!=", "not_equals":
		return !compareEqual(actual, expected)
	case "gt", ">":
		return compareNumeric(actual, expected) > 0
	case "lt", "<":
		return compareNumeric(actual, expected) < 0
	case "gte", ">=":
		return compareNumeric(actual, expected) >= 0
	case "lte", "<=":
		return compareNumeric(actual, expected) <= 0
	case "contains":
		return stringContains(actual, expected)
	case "not_contains":
		return !stringContains(actual, expected)
	case "empty":
		return isEmpty(actual)
	case "not_empty":
		return !isEmpty(actual)
	case "true", "is_true":
		return isTruthy(actual)
	case "false", "is_false":
		return !isTruthy(actual)
	default:
		return false
	}
}

// compareEqual 比较相等
func compareEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// compareNumeric 数值比较，返回 -1, 0, 1
func compareNumeric(a, b interface{}) int {
	aNum := toFloat64(a)
	bNum := toFloat64(b)
	if aNum < bNum {
		return -1
	}
	if aNum > bNum {
		return 1
	}
	return 0
}

// toFloat64 转换为 float64
func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	default:
		return 0
	}
}

// stringContains 字符串包含
func stringContains(a, b interface{}) bool {
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	return strings.Contains(aStr, bStr)
}

// isEmpty 判断是否为空
func isEmpty(v interface{}) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return val == ""
	case []interface{}:
		return len(val) == 0
	case map[string]interface{}:
		return len(val) == 0
	default:
		return false
	}
}

// isTruthy 判断是否为真值
func isTruthy(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val != "" && val != "false" && val != "0"
	case float64:
		return val != 0
	case int:
		return val != 0
	default:
		return true
	}
}

// executeAction 执行动作
func executeAction(action string, jumpStep int, conditionMet bool, variable string, actualValue interface{}) rpa.StepResult {
	output := map[string]interface{}{
		"conditionMet": conditionMet,
		"variable":     variable,
		"actualValue":  actualValue,
		"action":       action,
	}

	switch action {
	case "continue":
		// 继续执行下一步
		return rpa.StepResult{
			Completed: true,
			Success:   true,
			Output:    output,
		}
	case "skip_next":
		// 跳过下一步（通过 Output 传递信号给引擎）
		output["skipNext"] = true
		return rpa.StepResult{
			Completed: true,
			Success:   true,
			Output:    output,
		}
	case "jump_to_step":
		// 跳转到指定步骤
		output["jumpToStep"] = jumpStep
		return rpa.StepResult{
			Completed: true,
			Success:   true,
			Output:    output,
		}
	case "stop_success":
		// 成功停止流程
		output["stopFlow"] = true
		output["stopSuccess"] = true
		return rpa.StepResult{
			Completed: true,
			Success:   true,
			Output:    output,
		}
	case "stop_error":
		// 错误停止流程
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("条件判断触发停止: %s = %v", variable, actualValue),
			Output:    output,
		}
	default:
		return rpa.StepResult{
			Completed: true,
			Success:   true,
			Output:    output,
		}
	}
}
