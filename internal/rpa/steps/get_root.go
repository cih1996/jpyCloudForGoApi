package steps

import (
	"context"
	"fmt"
	"port-mapping-demo/internal/database"
	"port-mapping-demo/internal/rpa"
	"port-mapping-demo/internal/service"
	"port-mapping-demo/pkg/logger"
	"time"
)

// GetRootStep 应用提权步骤
type GetRootStep struct{}

func init() {
	rpa.RegisterStep(&GetRootStep{})
}

func (s *GetRootStep) Type() string {
	return "get_root"
}

func (s *GetRootStep) Name() string {
	return "应用提权"
}

func (s *GetRootStep) SubSteps() []string {
	return []string{"发送提权指令"}
}

func (s *GetRootStep) Execute(deviceID int, params map[string]interface{}, subStep int, ctx database.StepContext) rpa.StepResult {
	switch subStep {
	case 0:
		return s.sendGetRoot(deviceID, params)
	default:
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("未知子步骤: %d", subStep),
		}
	}
}

func (s *GetRootStep) sendGetRoot(deviceID int, params map[string]interface{}) rpa.StepResult {
	// 获取包名，默认 com.android.shell
	packageName, _ := params["packageName"].(string)
	if packageName == "" {
		packageName = "com.android.shell"
	}

	logger.LogInfo("[RPA] 设备 %d 发送提权指令: pkg=%s", deviceID, packageName)

	req := &service.UnifiedRequest{
		Type: "getRoot",
		Seq:  int(time.Now().UnixNano() % 1000000),
		Data: map[string]interface{}{
			"deviceId": float64(deviceID),
			"pkg":      packageName,
		},
	}

	res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
	if err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[应用提权/发送提权指令] %v", err),
		}
	}

	if res.Code != 200 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[应用提权/发送提权指令] %s", res.Msg),
		}
	}

	logger.LogInfo("[RPA] 设备 %d 提权成功: pkg=%s", deviceID, packageName)

	return rpa.StepResult{
		Completed: true,
		Success:   true,
		Output: map[string]interface{}{
			"deviceId":    deviceID,
			"packageName": packageName,
			"rooted":      true,
		},
	}
}
