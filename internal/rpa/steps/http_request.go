package steps

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"port-mapping-demo/internal/database"
	"port-mapping-demo/internal/rpa"
	"port-mapping-demo/pkg/logger"
	"strings"
	"time"
)

// HttpRequestStep HTTP 请求步骤
// 可以发送 HTTP 请求并将响应输出到流程变量
type HttpRequestStep struct{}

func init() {
	rpa.RegisterStep(&HttpRequestStep{})
}

func (s *HttpRequestStep) Type() string {
	return "http_request"
}

func (s *HttpRequestStep) Name() string {
	return "HTTP请求"
}

func (s *HttpRequestStep) SubSteps() []string {
	return []string{"发送请求"}
}

func (s *HttpRequestStep) Execute(deviceID int, params map[string]interface{}, subStep int, ctx database.StepContext) rpa.StepResult {
	// 获取参数
	url, _ := params["url"].(string)
	method, _ := params["method"].(string)
	body, _ := params["body"].(string)
	outputVar, _ := params["outputVar"].(string) // 输出变量名
	timeout, _ := params["timeout"].(float64)

	if url == "" {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     "缺少必要参数: url",
		}
	}

	if method == "" {
		method = "GET"
	}

	if timeout == 0 {
		timeout = 30
	}

	if outputVar == "" {
		outputVar = "httpResponse"
	}

	// 获取请求头
	headers := make(map[string]string)
	if h, ok := params["headers"].(map[string]interface{}); ok {
		for k, v := range h {
			headers[k] = fmt.Sprintf("%v", v)
		}
	}

	// 创建请求
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	req, err := http.NewRequest(strings.ToUpper(method), url, reqBody)
	if err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("创建请求失败: %v", err),
		}
	}

	// 设置请求头
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" && body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// 发送请求
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	logger.LogInfo("[RPA] 设备 %d 发送 HTTP 请求: %s %s", deviceID, method, url)

	resp, err := client.Do(req)
	if err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("请求失败: %v", err),
		}
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("读取响应失败: %v", err),
		}
	}

	// 解析响应
	var respData interface{}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		// 如果不是 JSON，直接作为字符串
		respData = string(respBody)
	}

	logger.LogInfo("[RPA] 设备 %d HTTP 响应: %d, 输出到变量: %s", deviceID, resp.StatusCode, outputVar)

	// 构建输出
	output := map[string]interface{}{
		outputVar: respData,
		outputVar + "_status": resp.StatusCode,
		outputVar + "_raw":    string(respBody),
	}

	// 检查状态码
	if resp.StatusCode >= 400 {
		return rpa.StepResult{
			Completed: true,
			Success:   false,
			Error:     fmt.Sprintf("HTTP 错误: %d", resp.StatusCode),
			Output:    output, // 即使失败也输出响应
		}
	}

	return rpa.StepResult{
		Completed: true,
		Success:   true,
		Output:    output,
	}
}
