package service

import (
	"net/http"
	"port-mapping-demo/pkg/logger"
	"strconv"

	"github.com/gin-gonic/gin"
)

// HandleLogQuery 日志查询接口
// GET /api/logs/query?type=devicews&lines=100&keyword=心跳
func HandleLogQuery(c *gin.Context) {
	logType := c.DefaultQuery("type", "devicews")
	linesStr := c.DefaultQuery("lines", "100")
	keyword := c.DefaultQuery("keyword", "")

	lines, err := strconv.Atoi(linesStr)
	if err != nil || lines <= 0 {
		lines = 100
	}
	if lines > 2000 {
		lines = 2000
	}

	switch logType {
	case "devicews":
		result, err := logger.QueryDeviceWSLogs(lines, keyword)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": "查询日志失败: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"data": gin.H{
				"type":    "devicews",
				"lines":   len(result),
				"keyword": keyword,
				"logs":    result,
			},
		})
	default:
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": "不支持的日志类型: " + logType})
	}
}
