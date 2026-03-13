package main

import (
	"io/fs"
	"net/http"
	"port-mapping-demo/internal/config"
	"port-mapping-demo/internal/database"
	"port-mapping-demo/internal/rpa"
	_ "port-mapping-demo/internal/rpa/steps" // 注册步骤模块
	"port-mapping-demo/internal/service"
	"port-mapping-demo/pkg/framework"
	"port-mapping-demo/pkg/logger"

	"github.com/ghp3000/logs"
	"github.com/gin-gonic/gin"
)

func main() {
	config.LoadConfig()
	logs.SetLevel("INFO", logs.LevelInfo)

	// Add File Adapter for global logging
	fileConfig := &logs.FileConfig{
		Dir:         "logs",
		FileName:    "system.log",
		FileMaxSize: 10 * 1024 * 1024, // 10MB
		FileMaxNum:  10,
		RollType:    logs.RollingFile,
	}
	fileAdapter, err := logs.NewFileLog(logs.LevelInfo, fileConfig, 1000, "")
	if err != nil {
		logs.Error("Failed to create file logger: %v", err)
	} else {
		logs.AddAdapter(fileAdapter)
	}

	// Initialize Custom Logger
	if err := logger.InitUnifiedLogger(); err != nil {
		logs.Error("Failed to initialize unified logger: %v", err)
	}

	// Initialize Database
	if err := database.Init("./data"); err != nil {
		logs.Error("Failed to initialize database: %v", err)
	} else {
		logs.Info("数据库初始化成功")
	}

	// Start RPA Engine
	rpa.GetEngine().Start()

	// Register Routes
	framework.Register("POST", "/api/devices", "Get list of devices", service.GetDevices)
	framework.Register("POST", "/api/mappings", "Get active mappings", service.GetMappings)
	framework.Register("POST", "/api/connect", "Connect to a device", service.ConnectDevice)
	framework.Register("POST", "/api/disconnect", "Disconnect from a device", service.DisconnectDevice)
	framework.Register[service.MiddleCommandRequest, interface{}]("POST", "/api/middle", "Execute middleware command", service.MiddleExecuteCommand)
	framework.Register("POST", "/api/config/update", "Update WebSocket configuration", service.UpdateConfig)
	framework.Register("POST", "/api/config/get", "Get WebSocket configuration", service.GetConfig)
	framework.Register("POST", "/api/unified", "Unified Request Handler", service.HandleUnifiedRequestHTTP)

	// Add Log Download Endpoint
	r := gin.Default()

	// CORS Middleware - 必须在路由注册之前
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

	r.GET("/api/logs/download", func(c *gin.Context) {
		logFile := "logs/unified_service.log"
		c.Header("Content-Disposition", "attachment; filename=unified_service.log")
		c.Header("Content-Type", "application/octet-stream")
		c.File(logFile)
	})

	// WebSocket support for Unified Request
	r.GET("/api/unified/ws", service.UnifiedWSHandler)

	// RPA Routes
	rpa.RegisterRoutes(r.Group("/api"))

	// Bind registered routes to Gin
	framework.BindHTTP(r)

	// Documentation Endpoint
	r.GET("/doc", func(c *gin.Context) {
		html := framework.RenderDocHTML()
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, html)
	})

	// Static files - 使用嵌入的文件系统
	staticFS := GetStaticFS()
	staticSubFS := GetStaticSubFS()

	r.GET("/assets/*filepath", func(c *gin.Context) {
		c.FileFromFS(c.Request.URL.Path, staticFS)
	})

	r.GET("/", func(c *gin.Context) {
		data, err := fs.ReadFile(staticSubFS, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	r.GET("/vite.svg", func(c *gin.Context) {
		data, err := fs.ReadFile(staticSubFS, "vite.svg")
		if err != nil {
			c.String(http.StatusNotFound, "Not Found")
			return
		}
		c.Data(http.StatusOK, "image/svg+xml", data)
	})

	r.GET("/全球时区.xml", func(c *gin.Context) {
		data, err := fs.ReadFile(staticSubFS, "全球时区.xml")
		if err != nil {
			c.String(http.StatusNotFound, "Not Found")
			return
		}
		c.Data(http.StatusOK, "application/xml; charset=utf-8", data)
	})

	r.GET("/全球语言.xml", func(c *gin.Context) {
		data, err := fs.ReadFile(staticSubFS, "全球语言.xml")
		if err != nil {
			c.String(http.StatusNotFound, "Not Found")
			return
		}
		c.Data(http.StatusOK, "application/xml; charset=utf-8", data)
	})

	// SPA fallback: 所有未匹配的路由返回 index.html
	r.NoRoute(func(c *gin.Context) {
		// 如果是 API 请求，返回 404
		if len(c.Request.URL.Path) > 4 && c.Request.URL.Path[:4] == "/api" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not Found"})
			return
		}
		// 读取嵌入的 index.html
		data, err := fs.ReadFile(staticSubFS, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

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
