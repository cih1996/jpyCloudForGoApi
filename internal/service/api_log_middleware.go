package service

import (
	"bytes"
	"io"
	"port-mapping-demo/pkg/logger"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// responseWriter 包装 gin.ResponseWriter 以捕获响应体
type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// APILogMiddleware API 收发明细日志中间件
func APILogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 跳过非 API 路径、静态资源、WebSocket 升级
		path := c.Request.URL.Path
		if !strings.HasPrefix(path, "/api/") {
			c.Next()
			return
		}
		// 跳过 WS 升级和日志查询自身（避免递归）
		if strings.Contains(path, "/ws") || strings.Contains(path, "/logs/") {
			c.Next()
			return
		}

		start := time.Now()
		method := c.Request.Method
		query := c.Request.URL.RawQuery

		// 读取请求体（限制大小避免内存爆炸）
		// 跳过 multipart/form-data（文件上传），读取会破坏流式 body
		var reqBody string
		contentType := c.GetHeader("Content-Type")
		isMultipart := strings.HasPrefix(contentType, "multipart/")
		if c.Request.Body != nil && (method == "POST" || method == "PUT") && !isMultipart {
			bodyBytes, _ := io.ReadAll(io.LimitReader(c.Request.Body, 2048))
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			reqBody = string(bodyBytes)
			if len(reqBody) > 200 {
				reqBody = reqBody[:200] + "..."
			}
		}

		// 包装 ResponseWriter 捕获响应体
		rw := &responseWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = rw

		// 执行请求
		c.Next()

		// 记录日志
		duration := time.Since(start)
		status := c.Writer.Status()

		// 响应体摘要
		respBody := rw.body.String()
		if len(respBody) > 200 {
			respBody = respBody[:200] + "..."
		}

		// 构建日志行
		if query != "" {
			path = path + "?" + query
		}

		if reqBody != "" {
			logger.APIInfo("← %s %s %dms status=%d req=%s resp=%s",
				method, path, duration.Milliseconds(), status, reqBody, respBody)
		} else {
			logger.APIInfo("← %s %s %dms status=%d resp=%s",
				method, path, duration.Milliseconds(), status, respBody)
		}
	}
}
