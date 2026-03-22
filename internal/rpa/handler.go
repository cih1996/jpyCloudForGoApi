package rpa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"port-mapping-demo/internal/database"
	"port-mapping-demo/pkg/logger"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RegisterRoutes 注册 RPA 相关路由
func RegisterRoutes(r *gin.RouterGroup) {
	rpa := r.Group("/rpa")
	{
		// 流程管理
		rpa.GET("/flows", listFlows)
		rpa.GET("/flows/:id", getFlow)
		rpa.POST("/flows", createFlow)
		rpa.PUT("/flows/:id", updateFlow)
		rpa.DELETE("/flows/:id", deleteFlow)
		rpa.GET("/flows/:id/export", exportFlow)
		rpa.POST("/flows/import", importFlow)

		// 设备配置
		rpa.GET("/devices", listDeviceConfigs)
		rpa.GET("/devices/:deviceId", getDeviceConfig)
		rpa.POST("/devices/:deviceId/bind", bindDeviceRpa)
		rpa.POST("/devices/:deviceId/start", startDevice)
		rpa.POST("/devices/:deviceId/pause", pauseDevice)
		rpa.POST("/devices/:deviceId/resume", resumeDevice)
		rpa.POST("/devices/:deviceId/stop", stopDevice)

		// 日志
		rpa.GET("/devices/:deviceId/logs", getDeviceLogs)
		rpa.DELETE("/devices/:deviceId/logs", clearDeviceLogs)

		// 引擎控制
		rpa.GET("/engine/status", getEngineStatus)
		rpa.POST("/engine/start", startEngine)
		rpa.POST("/engine/stop", stopEngine)

		// 步骤类型
		rpa.GET("/step-types", listStepTypes)

		// 单步临时执行
		rpa.POST("/run-step", runSingleStep)

		// 脚本反馈接口
		rpa.POST("/devices/:deviceId/feedback", updateFeedback)

		// 代码仓库
		rpa.GET("/scripts", listScripts)
		rpa.GET("/scripts/:id", getScript)
		rpa.POST("/scripts", createScript)
		rpa.PUT("/scripts/:id", updateScript)
		rpa.DELETE("/scripts/:id", deleteScript)
		rpa.POST("/scripts/:id/debug", debugScript)

		// 执行历史
		rpa.GET("/history", listExecutionHistory)
		rpa.GET("/history/:id", getExecutionHistory)
		rpa.GET("/history/:id/steps", getExecutionSteps)
		rpa.GET("/devices/:deviceId/history", getDeviceExecutionHistory)
	}
}

// ========== 流程管理 ==========

func listFlows(c *gin.Context) {
	flows, err := database.GetAllRpaFlows()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": flows})
}

func getFlow(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	flow, err := database.GetRpaFlow(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if flow == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "流程不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": flow})
}

func createFlow(c *gin.Context) {
	var flow database.RpaFlow
	if err := c.ShouldBindJSON(&flow); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := database.CreateRpaFlow(&flow); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": flow})
}

func updateFlow(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var flow database.RpaFlow
	if err := c.ShouldBindJSON(&flow); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	flow.ID = uint(id)
	if err := database.UpdateRpaFlow(&flow); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": flow})
}

func deleteFlow(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := database.DeleteRpaFlow(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

func exportFlow(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	jsonStr, err := ExportFlowJSON(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=rpa_flow.json")
	c.String(http.StatusOK, jsonStr)
}

func importFlow(c *gin.Context) {
	var req struct {
		JSON string `json:"json"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	flow, err := ImportFlowJSON(req.JSON)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": flow})
}

// ========== 设备配置 ==========

func listDeviceConfigs(c *gin.Context) {
	statuses, err := GetEngine().GetAllDeviceStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": statuses})
}

func getDeviceConfig(c *gin.Context) {
	deviceID, _ := strconv.Atoi(c.Param("deviceId"))
	status, err := GetEngine().GetDeviceStatus(deviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": status})
}

func bindDeviceRpa(c *gin.Context) {
	deviceID, _ := strconv.Atoi(c.Param("deviceId"))
	var req struct {
		RpaID uint   `json:"rpaId"`
		Mode  string `json:"mode"` // single / loop
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mode := database.ModeSingle
	if req.Mode == "loop" {
		mode = database.ModeLoop
	}

	if err := database.SetDeviceRpa(deviceID, req.RpaID, mode); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "绑定成功"})
}

func startDevice(c *gin.Context) {
	deviceID, _ := strconv.Atoi(c.Param("deviceId"))

	// 先取消之前可能残留的运行中记录
	database.CancelRunningExecutions(deviceID)

	if err := database.StartDevice(deviceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 创建执行记录
	config, _ := database.GetDeviceConfig(deviceID)
	if config != nil && config.RpaID > 0 {
		flow, _ := database.GetRpaFlow(config.RpaID)
		if flow != nil {
			GetEngine().StartHistory(deviceID, flow.ID, flow.Name, len(flow.Steps))
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "启动成功"})
}

func pauseDevice(c *gin.Context) {
	deviceID, _ := strconv.Atoi(c.Param("deviceId"))
	if err := database.PauseDevice(deviceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "暂停成功"})
}

func resumeDevice(c *gin.Context) {
	deviceID, _ := strconv.Atoi(c.Param("deviceId"))
	if err := database.ResumeDevice(deviceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "恢复成功"})
}

func stopDevice(c *gin.Context) {
	deviceID, _ := strconv.Atoi(c.Param("deviceId"))

	// 完成执行记录（标记为取消）
	GetEngine().CompleteHistory(deviceID, database.ExecStatusCancelled, "用户手动停止")

	if err := database.StopDevice(deviceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "停止成功"})
}

// ========== 日志 ==========

func getDeviceLogs(c *gin.Context) {
	deviceID, _ := strconv.Atoi(c.Param("deviceId"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	logs, err := database.GetDeviceLogs(deviceID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}

func clearDeviceLogs(c *gin.Context) {
	deviceID, _ := strconv.Atoi(c.Param("deviceId"))
	if err := database.ClearDeviceLogs(deviceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "清除成功"})
}

// ========== 引擎控制 ==========

func getEngineStatus(c *gin.Context) {
	engine := GetEngine()
	c.JSON(http.StatusOK, gin.H{
		"running": engine.IsRunning(),
	})
}

func startEngine(c *gin.Context) {
	GetEngine().Start()
	c.JSON(http.StatusOK, gin.H{"message": "引擎已启动"})
}

func stopEngine(c *gin.Context) {
	GetEngine().Stop()
	c.JSON(http.StatusOK, gin.H{"message": "引擎已停止"})
}

// ========== 步骤类型 ==========

func listStepTypes(c *gin.Context) {
	types := GetStepInfoList()
	c.JSON(http.StatusOK, gin.H{"data": types})
}

// ========== 单步临时执行 ==========

// runSingleStep 临时执行单个 RPA 步骤（不创建流程，不写数据库）
// 同步等待步骤完成，带超时
func runSingleStep(c *gin.Context) {
	var req struct {
		DeviceID int                    `json:"deviceId" binding:"required"`
		Type     string                 `json:"type" binding:"required"`
		Params   map[string]interface{} `json:"params"`
		Timeout  int                    `json:"timeout"` // 超时秒数，默认300
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("参数错误: %v", err)})
		return
	}

	// 获取步骤执行器
	executor := GetStepExecutor(req.Type)
	if executor == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("未知步骤类型: %s", req.Type)})
		return
	}

	if req.Params == nil {
		req.Params = make(map[string]interface{})
	}

	timeout := 300
	if req.Timeout > 0 {
		timeout = req.Timeout
	}

	logger.LogInfo("[RPA] 单步执行: 设备 %d, 类型 %s, 超时 %ds", req.DeviceID, req.Type, timeout)

	// 轮询执行直到完成或超时
	subStep := 0
	ctx := make(database.StepContext)
	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	pollInterval := 2 * time.Second
	var lastResult StepResult

	for {
		if time.Now().After(deadline) {
			c.JSON(http.StatusOK, gin.H{
				"data": gin.H{
					"completed": true,
					"success":   false,
					"error":     fmt.Sprintf("执行超时（%ds）", timeout),
					"stepType":  req.Type,
					"stepName":  executor.Name(),
				},
			})
			return
		}

		lastResult = executor.Execute(req.DeviceID, req.Params, subStep, ctx)

		if lastResult.Completed {
			// 步骤完成
			result := gin.H{
				"completed": true,
				"success":   lastResult.Success,
				"stepType":  req.Type,
				"stepName":  executor.Name(),
			}
			if lastResult.Error != "" {
				result["error"] = lastResult.Error
			}
			if lastResult.Output != nil {
				outputJSON, _ := json.Marshal(lastResult.Output)
				result["output"] = json.RawMessage(outputJSON)
			}
			c.JSON(http.StatusOK, gin.H{"data": result})
			return
		}

		// 未完成，更新子步骤和上下文，继续轮询
		subStep = lastResult.NextSub
		if lastResult.Context != nil {
			ctx = lastResult.Context
		}

		time.Sleep(pollInterval)
	}
}

// ========== 脚本反馈 ==========

func updateFeedback(c *gin.Context) {
	deviceID, _ := strconv.Atoi(c.Param("deviceId"))
	var req struct {
		Status   string `json:"status"`
		Progress int    `json:"progress"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := UpdateScriptFeedback(deviceID, req.Status, req.Progress); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// ========== 代码仓库 ==========

func listScripts(c *gin.Context) {
	keyword := c.Query("keyword")
	var scripts []database.ScriptRepo
	var err error
	if keyword != "" {
		scripts, err = database.SearchScripts(keyword)
	} else {
		scripts, err = database.GetAllScripts()
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": scripts})
}

func getScript(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	script, err := database.GetScript(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if script == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "脚本不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": script})
}

func createScript(c *gin.Context) {
	var script database.ScriptRepo
	if err := c.ShouldBindJSON(&script); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if script.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "脚本名称不能为空"})
		return
	}
	if err := database.CreateScript(&script); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": script})
}

func updateScript(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var script database.ScriptRepo
	if err := c.ShouldBindJSON(&script); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	script.ID = uint(id)
	if err := database.UpdateScript(&script); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": script})
}

func deleteScript(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := database.DeleteScript(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// debugScript 在设备上调试执行脚本
func debugScript(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		DeviceID int `json:"deviceId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. 获取脚本
	script, err := database.GetScript(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if script == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "脚本不存在"})
		return
	}

	// 2. 读取脚本代码（从文件）
	scriptPath := filepath.Join("./scripts", script.Name+".js")
	codeBytes, err := os.ReadFile(scriptPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取脚本文件失败: %v", err)})
		return
	}
	code := string(codeBytes)

	// 3. 解析设备 CRC32 ID
	crc32ID, err := resolveDeviceCRC32Internal(req.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 4. 执行脚本并获取结果
	timeout := script.Timeout
	if timeout == 0 {
		timeout = 30000 // 默认30秒
	}
	result, err := executeScriptOnDevice(crc32ID, code, timeout)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 5. 返回结果
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"success": result.Success,
			"logs":    result.Logs,
			"result":  result.Result,
		},
	})
}

// ========== 执行历史 ==========

func listExecutionHistory(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	deviceID, _ := strconv.Atoi(c.DefaultQuery("deviceId", "0"))
	rpaID, _ := strconv.ParseUint(c.DefaultQuery("rpaId", "0"), 10, 64)
	status := c.DefaultQuery("status", "")

	histories, err := database.GetFilteredExecutionHistory(limit, deviceID, uint(rpaID), status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": histories})
}

func getExecutionHistory(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	history, err := database.GetExecutionHistory(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if history == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	// 同时返回步骤明细
	steps, _ := database.GetExecutionSteps(uint(id))
	c.JSON(http.StatusOK, gin.H{"data": history, "steps": steps})
}

func getExecutionSteps(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	steps, err := database.GetExecutionSteps(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": steps})
}

func getDeviceExecutionHistory(c *gin.Context) {
	deviceID, _ := strconv.Atoi(c.Param("deviceId"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	histories, err := database.GetDeviceExecutionHistory(deviceID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": histories})
}

// ========== 脚本调试辅助函数 ==========

// DebugResult 调试执行结果
type DebugResult struct {
	DebugID   string        `json:"debugId"`
	Success   bool          `json:"success"`
	Result    interface{}   `json:"result,omitempty"`
	Error     string        `json:"error,omitempty"`
	Logs      []interface{} `json:"logs,omitempty"`
	Duration  int64         `json:"duration"`
	Timestamp int64         `json:"timestamp"`
}

// resolveDeviceCRC32Internal 通过设备编号查找对应的 CRC32 ID
func resolveDeviceCRC32Internal(deviceID int) (uint32, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get("http://127.0.0.1:1001/api/debug/ws-match")
	if err != nil {
		return 0, fmt.Errorf("查询设备映射失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Code int `json:"code"`
		Data struct {
			CloudDevices  []map[string]interface{} `json:"cloudDevices"`
			WsConnections []map[string]interface{} `json:"wsConnections"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.Code != 200 {
		return 0, fmt.Errorf("解析设备映射失败")
	}

	// 1. 从云平台设备列表找设备编号对应的 UUID
	var targetUUID string
	for _, dev := range result.Data.CloudDevices {
		if did, ok := dev["deviceId"].(float64); ok && int(did) == deviceID {
			targetUUID, _ = dev["uuid"].(string)
			break
		}
	}
	if targetUUID == "" {
		return 0, fmt.Errorf("设备 %d 不存在或未在平台注册", deviceID)
	}

	// 2. 从 WS 连接列表找 serialno == UUID 的，拿到 CRC32 ID
	for _, conn := range result.Data.WsConnections {
		sn, _ := conn["serialno"].(string)
		if sn == targetUUID {
			crc32Hex, _ := conn["deviceId"].(string)
			if crc32Hex == "" {
				continue
			}
			v, err := strconv.ParseUint(crc32Hex, 16, 32)
			if err != nil {
				return 0, fmt.Errorf("解析 CRC32 ID 失败: %s", crc32Hex)
			}
			return uint32(v), nil
		}
	}

	return 0, fmt.Errorf("设备 %d 的脚本APK未连接，请先执行 start_bot", deviceID)
}

// executeScriptOnDevice 在设备上执行脚本并等待结果
func executeScriptOnDevice(crc32ID uint32, code string, timeout int) (*DebugResult, error) {
	client := &http.Client{Timeout: time.Duration(timeout+5000) * time.Millisecond}

	// 生成 debugId
	debugID := uuid.New().String()[:8]

	// 发送调试执行请求
	sendURL := fmt.Sprintf("http://127.0.0.1:1001/api/devicews/devices/%d/debug", crc32ID)
	body := map[string]interface{}{
		"debugId": debugID,
		"code":    code,
		"timeout": int64(timeout),
	}

	jsonData, _ := json.Marshal(body)
	resp, err := client.Post(sendURL, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("发送调试请求失败: %v", err)
	}
	resp.Body.Close()

	logger.LogInfo("[RPA] 脚本调试已发送到设备 CRC32=%08X, debugId=%s", crc32ID, debugID)

	// 轮询等待结果
	resultURL := fmt.Sprintf("http://127.0.0.1:1001/api/devicews/devices/%d/debug/%s", crc32ID, debugID)
	deadline := time.Now().Add(time.Duration(timeout) * time.Millisecond)

	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)

		resp, err := client.Get(resultURL)
		if err != nil {
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var apiResp struct {
			Code int             `json:"code"`
			Msg  string          `json:"msg"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(respBody, &apiResp); err != nil {
			continue
		}

		// 404 = 结果还没回来
		if apiResp.Code == 404 {
			continue
		}

		if apiResp.Code != 200 || len(apiResp.Data) == 0 {
			continue
		}

		var result DebugResult
		if err := json.Unmarshal(apiResp.Data, &result); err != nil {
			continue
		}

		// 拿到结果了
		logger.LogInfo("[RPA] 脚本调试完成: debugId=%s, success=%v, duration=%dms", debugID, result.Success, result.Duration)
		return &result, nil
	}

	// 超时
	return nil, fmt.Errorf("等待结果超时（%dms）", timeout)
}
