package rpa

import (
	"net/http"
	"port-mapping-demo/internal/database"
	"strconv"

	"github.com/gin-gonic/gin"
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

		// 脚本反馈接口
		rpa.POST("/devices/:deviceId/feedback", updateFeedback)

		// 代码仓库
		rpa.GET("/scripts", listScripts)
		rpa.GET("/scripts/:id", getScript)
		rpa.POST("/scripts", createScript)
		rpa.PUT("/scripts/:id", updateScript)
		rpa.DELETE("/scripts/:id", deleteScript)

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
