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

// SetProxyAndWaitStep 设置代理步骤
// 参数:
//   - s5Url: S5 代理地址，支持变量引用如 {{httpResponse.proxy}}
//   - nOutSwID: 出口线路 ID
//   - lineType: 线路类型
//
// 变量引用示例:
//   先用 http_request 步骤获取代理，outputVar 设为 "proxyData"
//   然后在此步骤中 s5Url 填写 {{proxyData.s5Url}} 即可动态获取
type SetProxyAndWaitStep struct{}

func init() {
	rpa.RegisterStep(&SetProxyAndWaitStep{})
}

func (s *SetProxyAndWaitStep) Type() string {
	return "set_proxy_and_wait"
}

func (s *SetProxyAndWaitStep) Name() string {
	return "设置代理"
}

func (s *SetProxyAndWaitStep) SubSteps() []string {
	return []string{"发送代理设置", "等待生效"}
}

func (s *SetProxyAndWaitStep) Execute(deviceID int, params map[string]interface{}, subStep int, ctx database.StepContext) rpa.StepResult {
	switch subStep {
	case 0:
		return s.setProxy(deviceID, params, ctx)
	case 1:
		return s.waitEffect(deviceID, ctx)
	default:
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("未知子步骤: %d", subStep),
		}
	}
}

func (s *SetProxyAndWaitStep) setProxy(deviceID int, params map[string]interface{}, ctx database.StepContext) rpa.StepResult {
	s5Url, _ := params["s5Url"].(string)
	nOutSwID, _ := params["nOutSwID"].(float64)
	lineType, _ := params["lineType"].(float64)

	// 出口线路必填
	if nOutSwID == 0 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "[设置代理/发送代理设置] 缺少必要参数: nOutSwID（出口线路）",
		}
	}

	// s5Url 可以为空，表示取消代理
	action := "设置代理"
	if s5Url == "" {
		action = "取消代理"
	}

	req := &service.UnifiedRequest{
		Type: "setSocket5",
		Seq:  int(time.Now().Unix()),
		Data: map[string]interface{}{
			"deviceId": deviceID,
			"s5Url":    s5Url,
			"nOutSwID": int(nOutSwID),
			"lineType": int(lineType),
		},
	}

	res, err := service.HandleUnifiedRequestHTTP(context.Background(), req)
	if err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[设置代理/发送代理设置] %v", err),
		}
	}

	if res.Code != 200 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("[设置代理/发送代理设置] %s", res.Msg),
		}
	}

	logger.LogInfo("[RPA] 设备 %d %s: s5Url=%s, nOutSwID=%d", deviceID, action, s5Url, int(nOutSwID))

	newCtx := make(database.StepContext)
	newCtx["startTime"] = time.Now().Unix()
	newCtx["action"] = action

	return rpa.StepResult{
		Completed: false,
		NextSub:   1,
		Context:   newCtx,
	}
}

func (s *SetProxyAndWaitStep) waitEffect(deviceID int, ctx database.StepContext) rpa.StepResult {
	startTime, _ := ctx["startTime"].(float64)
	action, _ := ctx["action"].(string)
	if action == "" {
		action = "代理设置"
	}

	// 等待 5 秒让代理生效
	if time.Now().Unix()-int64(startTime) > 5 {
		logger.LogInfo("[RPA] 设备 %d %s完成", deviceID, action)
		return rpa.StepResult{
			Completed: true,
			Success:   true,
			Output: map[string]interface{}{
				"deviceId": deviceID,
				"action":   action,
			},
		}
	}

	return rpa.StepResult{
		Completed: false,
		NextSub:   1,
		Context:   ctx,
	}
}
