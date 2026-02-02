package main

import (
	"net/http"
	"port-mapping-demo/internal/config"
	"port-mapping-demo/internal/service"
	"port-mapping-demo/pkg/framework"

	"github.com/ghp3000/logs"
	"github.com/gin-gonic/gin"
)

func main() {
	config.LoadConfig()
	logs.SetLevel("INFO", logs.LevelInfo)

	// Register Routes
	framework.Register("POST", "/api/devices", "Get list of devices", service.GetDevices)
	framework.Register("POST", "/api/mappings", "Get active mappings", service.GetMappings)
	framework.Register("POST", "/api/connect", "Connect to a device", service.ConnectDevice)
	framework.Register("POST", "/api/disconnect", "Disconnect from a device", service.DisconnectDevice)
	framework.Register[service.MiddleCommandRequest, interface{}]("POST", "/api/middle", "Execute middleware command", service.MiddleExecuteCommand)
	framework.Register("POST", "/api/config/update", "Update WebSocket configuration", service.UpdateConfig)
	framework.Register("POST", "/api/config/get", "Get WebSocket configuration", service.GetConfig)
	framework.Register("POST", "/api/unified", "Unified Request Handler", service.HandleUnifiedRequest)

	// Initialize Gin
	r := gin.Default()

	// WebSocket support for Unified Request
	r.GET("/api/unified/ws", service.UnifiedWSHandler)

	// CORS Middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Bind registered routes to Gin
	framework.BindHTTP(r)

	// Documentation Endpoint
	r.GET("/doc", func(c *gin.Context) {
		html := framework.RenderDocHTML()
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, html)
	})

	// Static files
	distPath := "./static"
	r.Static("/assets", distPath+"/assets")
	r.StaticFile("/", distPath+"/index.html")
	r.StaticFile("/vite.svg", distPath+"/vite.svg")

	// Start WebSocket Server
	go framework.StartWS(1002)

	// Start Server
	logs.Info("API服务已启动，端口: 1001")

	// Fix: Trusted proxies warning
	if err := r.SetTrustedProxies(nil); err != nil {
		logs.Error("Failed to set trusted proxies", err)
	}

	if err := r.Run("0.0.0.0:1001"); err != nil {
		logs.Error("Web服务启动失败", err)
	}
}
