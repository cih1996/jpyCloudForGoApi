package devicews

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RegisterAPIRoutes 注册设备 WS 相关的 API 路由
func (s *Server) RegisterAPIRoutes(group *gin.RouterGroup) {
	// 获取设备列表
	group.GET("/devices", s.apiGetDevices)
	// 获取单个设备信息
	group.GET("/devices/:deviceId", s.apiGetDevice)
	// 获取统计信息
	group.GET("/stats", s.apiGetStats)
	// 向设备发送任务
	group.POST("/devices/:deviceId/task", s.apiSendTask)
	// 向设备发送命令
	group.POST("/devices/:deviceId/command", s.apiSendCommand)
	// 向设备发送调试执行
	group.POST("/devices/:deviceId/debug", s.apiSendDebug)
	// 获取调试执行结果
	group.GET("/devices/:deviceId/debug/:debugId", s.apiGetDebugResult)
	// 请求截图
	group.POST("/devices/:deviceId/screenshot", s.apiRequestScreenshot)
	// 获取截图数据
	group.GET("/devices/:deviceId/screenshot", s.apiGetScreenshot)
	// 请求节点信息
	group.POST("/devices/:deviceId/nodes", s.apiRequestNodes)
	// 获取节点信息
	group.GET("/devices/:deviceId/nodes", s.apiGetNodes)
	// 广播任务
	group.POST("/broadcast/task", s.apiBroadcastTask)
	// 广播命令
	group.POST("/broadcast/command", s.apiBroadcastCommand)
	// 推送资源到设备
	group.POST("/devices/:deviceId/resource-push", s.apiResourcePush)
	// 断开设备连接
	group.DELETE("/devices/:deviceId", s.apiDisconnectDevice)
}

// apiGetDevices 获取所有在线设备
func (s *Server) apiGetDevices(c *gin.Context) {
	devices := s.manager.GetAll()
	result := make([]map[string]interface{}, 0, len(devices))

	for _, dc := range devices {
		info := map[string]interface{}{
			"deviceId":    dc.DeviceID,
			"serialno":    dc.Serialno,
			"state":       dc.State,
			"connectedAt": dc.ConnectedAt.UnixMilli(),
			"lastSeen":    dc.LastSeen.UnixMilli(),
		}
		if dc.Info != nil {
			info["brand"] = dc.Info.Brand
			info["model"] = dc.Info.Model
			info["version"] = dc.Info.Version
			info["sdkVersion"] = dc.Info.SdkVersion
			info["screenWidth"] = dc.Info.ScreenWidth
			info["screenHeight"] = dc.Info.ScreenHeight
			info["androidVersion"] = dc.Info.AndroidVersion
		}
		result = append(result, info)
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": result,
	})
}

// apiGetDevice 获取单个设备信息
func (s *Server) apiGetDevice(c *gin.Context) {
	deviceIDStr := c.Param("deviceId")
	var deviceID uint32
	if _, err := parseHexOrDec(deviceIDStr, &deviceID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid deviceId"})
		return
	}

	dc, ok := s.manager.Get(deviceID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "device not found"})
		return
	}

	info := map[string]interface{}{
		"deviceId":    dc.DeviceID,
		"serialno":    dc.Serialno,
		"state":       dc.State,
		"connectedAt": dc.ConnectedAt.UnixMilli(),
		"lastSeen":    dc.LastSeen.UnixMilli(),
	}
	if dc.Info != nil {
		info["brand"] = dc.Info.Brand
		info["model"] = dc.Info.Model
		info["version"] = dc.Info.Version
		info["sdkVersion"] = dc.Info.SdkVersion
		info["screenWidth"] = dc.Info.ScreenWidth
		info["screenHeight"] = dc.Info.ScreenHeight
		info["androidVersion"] = dc.Info.AndroidVersion
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": info,
	})
}

// apiGetStats 获取统计信息
func (s *Server) apiGetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": s.manager.Stats(),
	})
}

// apiSendTask 向设备发送任务
func (s *Server) apiSendTask(c *gin.Context) {
	deviceIDStr := c.Param("deviceId")
	var deviceID uint32
	if _, err := parseHexOrDec(deviceIDStr, &deviceID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid deviceId"})
		return
	}

	var task TaskPushPayload
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	if err := s.SendTaskToDevice(deviceID, &task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok"})
}

// apiSendCommand 向设备发送命令
func (s *Server) apiSendCommand(c *gin.Context) {
	deviceIDStr := c.Param("deviceId")
	var deviceID uint32
	if _, err := parseHexOrDec(deviceIDStr, &deviceID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid deviceId"})
		return
	}

	var req struct {
		Cmd    string                 `json:"cmd"`
		Params map[string]interface{} `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	if err := s.SendCommandToDevice(deviceID, req.Cmd, req.Params); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok"})
}

// apiSendDebug 向设备发送调试执行
func (s *Server) apiSendDebug(c *gin.Context) {
	deviceIDStr := c.Param("deviceId")
	var deviceID uint32
	if _, err := parseHexOrDec(deviceIDStr, &deviceID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid deviceId"})
		return
	}

	var req struct {
		DebugID string `json:"debugId"`
		Code    string `json:"code"`
		Timeout int64  `json:"timeout"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	if req.Timeout == 0 {
		req.Timeout = 30000
	}

	if err := s.SendDebugToDevice(deviceID, req.DebugID, req.Code, req.Timeout); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok"})
}

// apiBroadcastTask 广播任务
func (s *Server) apiBroadcastTask(c *gin.Context) {
	var task TaskPushPayload
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	sent := s.BroadcastTask(&task)
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "sent": sent})
}

// apiBroadcastCommand 广播命令
func (s *Server) apiBroadcastCommand(c *gin.Context) {
	var req struct {
		Cmd    string                 `json:"cmd"`
		Params map[string]interface{} `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	sent := s.BroadcastCommand(req.Cmd, req.Params)
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "sent": sent})
}

// apiDisconnectDevice 断开设备连接
func (s *Server) apiDisconnectDevice(c *gin.Context) {
	deviceIDStr := c.Param("deviceId")
	var deviceID uint32
	if _, err := parseHexOrDec(deviceIDStr, &deviceID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid deviceId"})
		return
	}

	s.manager.Remove(deviceID)
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok"})
}

// parseHexOrDec 解析十六进制或十进制数字
func parseHexOrDec(s string, result *uint32) (bool, error) {
	var v uint64

	// 尝试十六进制
	if len(s) > 2 && s[:2] == "0x" {
		_, err := fmt.Sscanf(s, "0x%x", &v)
		if err != nil {
			return false, err
		}
	} else {
		// 十进制
		_, err := fmt.Sscanf(s, "%d", &v)
		if err != nil {
			return false, err
		}
	}

	*result = uint32(v)
	return true, nil
}

// apiRequestScreenshot 请求设备截图
func (s *Server) apiRequestScreenshot(c *gin.Context) {
	deviceIDStr := c.Param("deviceId")
	var deviceID uint32
	if _, err := parseHexOrDec(deviceIDStr, &deviceID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid deviceId"})
		return
	}

	dc, ok := s.manager.Get(deviceID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "device not found"})
		return
	}

	// 发送截图命令
	if err := dc.SendCommand("screenshot", nil); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "screenshot requested"})
}

// apiGetScreenshot 获取设备截图数据
func (s *Server) apiGetScreenshot(c *gin.Context) {
	deviceIDStr := c.Param("deviceId")
	var deviceID uint32
	if _, err := parseHexOrDec(deviceIDStr, &deviceID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid deviceId"})
		return
	}

	dc, ok := s.manager.Get(deviceID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "device not found"})
		return
	}

	data, timestamp := dc.GetScreenshot()
	if data == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "no screenshot available"})
		return
	}

	// 返回完整的截图数据结构
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": map[string]interface{}{
			"width":     data.Width,
			"height":    data.Height,
			"data":      data.Data,
			"timestamp": timestamp.UnixMilli(),
		},
	})
}

// apiRequestNodes 请求设备节点信息
func (s *Server) apiRequestNodes(c *gin.Context) {
	deviceIDStr := c.Param("deviceId")
	var deviceID uint32
	if _, err := parseHexOrDec(deviceIDStr, &deviceID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid deviceId"})
		return
	}

	dc, ok := s.manager.Get(deviceID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "device not found"})
		return
	}

	// 解析请求参数
	var req struct {
		RequestID   string `json:"requestId"`
		WindowID    *int   `json:"windowId"`
		VisibleOnly *bool  `json:"visibleOnly"`
	}
	c.ShouldBindJSON(&req)

	// 构建命令参数
	params := map[string]interface{}{}
	if req.RequestID != "" {
		params["requestId"] = req.RequestID
	}
	// windowId 默认 -1（当前活动窗口）
	if req.WindowID != nil {
		params["windowId"] = *req.WindowID
	} else {
		params["windowId"] = -1
	}
	if req.VisibleOnly != nil {
		params["visibleOnly"] = *req.VisibleOnly
	} else {
		params["visibleOnly"] = true
	}

	// 发送 getNodes 命令
	if err := dc.SendCommand("getNodes", params); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "nodes requested"})
}

// apiGetNodes 获取设备节点信息
func (s *Server) apiGetNodes(c *gin.Context) {
	deviceIDStr := c.Param("deviceId")
	var deviceID uint32
	if _, err := parseHexOrDec(deviceIDStr, &deviceID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid deviceId"})
		return
	}

	dc, ok := s.manager.Get(deviceID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "device not found"})
		return
	}

	data, timestamp := dc.GetNodes()
	if data == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "no nodes data available"})
		return
	}

	// 处理 nodes 字段：可能是 JSON 对象或 JSON 字符串
	var nodesData interface{}
	if len(data.Nodes) > 0 {
		// 尝试判断是字符串还是对象
		if data.Nodes[0] == '"' {
			// 是 JSON 字符串，需要解析
			var nodesStr string
			if err := json.Unmarshal(data.Nodes, &nodesStr); err == nil {
				// 再解析字符串内容为对象
				var nodesObj map[string]interface{}
				if err := json.Unmarshal([]byte(nodesStr), &nodesObj); err == nil {
					nodesData = nodesObj
				} else {
					nodesData = nodesStr // 解析失败，返回原字符串
				}
			}
		} else {
			// 是 JSON 对象，直接解析
			var nodesObj map[string]interface{}
			if err := json.Unmarshal(data.Nodes, &nodesObj); err == nil {
				nodesData = nodesObj
			} else {
				nodesData = string(data.Nodes) // 解析失败，返回原始内容
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": map[string]interface{}{
			"requestId": data.RequestID,
			"nodes":     nodesData,
			"timestamp": timestamp.UnixMilli(),
		},
	})
}

// apiResourcePush 推送资源到设备
func (s *Server) apiResourcePush(c *gin.Context) {
	deviceIDStr := c.Param("deviceId")
	var deviceID uint32
	if _, err := parseHexOrDec(deviceIDStr, &deviceID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid deviceId"})
		return
	}

	dc, ok := s.manager.Get(deviceID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "device not found"})
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
		Hash string `json:"hash" binding:"required"`
		Data string `json:"data" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	// base64 数据大小检查：50MB 原始文件 ≈ 67MB base64
	const maxBase64Size = 67 * 1024 * 1024
	if len(req.Data) > maxBase64Size {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  fmt.Sprintf("文件过大（base64 %dMB），当前限制 50MB 原始文件", len(req.Data)/1024/1024),
		})
		return
	}

	if err := dc.SendResourcePush(req.Name, req.Hash, req.Data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "resource pushed"})
}

// apiGetDebugResult 获取调试执行结果
func (s *Server) apiGetDebugResult(c *gin.Context) {
	deviceIDStr := c.Param("deviceId")
	debugID := c.Param("debugId")

	var deviceID uint32
	if _, err := parseHexOrDec(deviceIDStr, &deviceID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid deviceId"})
		return
	}

	dc, ok := s.manager.Get(deviceID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "device not found"})
		return
	}

	result, hasResult := dc.GetDebugResult(debugID)
	if !hasResult {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "debug result not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": result,
	})
}
