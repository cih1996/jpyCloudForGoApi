package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"port-mapping-demo/cmd/cli"
	"port-mapping-demo/internal/config"
	"port-mapping-demo/internal/database"
	"port-mapping-demo/internal/devicews"
	"port-mapping-demo/internal/rpa"
	_ "port-mapping-demo/internal/rpa/steps"
	"port-mapping-demo/internal/service"
	"port-mapping-demo/pkg/framework"
	"port-mapping-demo/pkg/logger"
	"runtime"

	"github.com/ghp3000/logs"
	"github.com/gin-gonic/gin"
)

const VERSION = "1.0.23"

func init() {
	// CLI 模式下禁用 debug 日志输出到 stderr
	// 检查是否是 serve 命令，如果不是则丢弃 stderr
	if len(os.Args) < 2 || (os.Args[1] != "serve" && os.Args[1] != "server" && os.Args[1] != "run") {
		// 非 serve 命令，丢弃 stderr（第三方包的 debug 日志）
		devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err == nil {
			os.Stderr = devNull
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := os.Args[1]

	switch cmd {
	case "serve", "server", "run":
		runServer()
	case "install", "uninstall", "upgrade", "update":
		fmt.Println("该命令已移除，请直接运行: jpy-cloud serve")
	case "service":
		fmt.Println("service 命令已移除，请直接运行: jpy-cloud serve")
	case "devices", "device":
		handleDevices()
	case "shell", "sh":
		handleShell()
	case "screenshot", "ss":
		handleScreenshot()
	case "logs", "log":
		handleLogs()
	case "tunnel", "connect":
		handleTunnel()
	case "disconnect":
		handleDisconnect()
	case "mappings", "mapping":
		handleMappings()
	case "adb":
		handleAdb()
	case "rpa":
		cli.RpaCommand(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Printf("jpy-cloud version %s (%s/%s)\n", VERSION, runtime.GOOS, runtime.GOARCH)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("未知命令: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`
JPY Server v%s - 集控平台本地代理

用法:
  jpy-cloud <命令> [参数]

服务命令:
  serve                    启动后端服务（前台运行）

设备命令（需要 -s 和 -k 参数，本地服务必须已启动，支持 --json 输出）:
  devices -s <集控平台> -k <密钥> [-v] [--json]           获取设备列表
  shell -s <集控平台> -k <密钥> <设备ID> <命令> [--json]   执行 Shell 命令
  screenshot -s <集控平台> -k <密钥> <设备ID>             截图

隧道命令（端口映射/打洞，支持 --json 输出）:
  tunnel -s <集控平台> -k <密钥> <设备ID> <本地端口> <远程端口> [--json]  建立端口映射
  disconnect -k <密钥> <本地端口> [--json]                               断开端口映射
  mappings [--json]                                                     查看当前所有端口映射

ADB 调试命令:
  adb -s <集控平台> -k <密钥> <设备ID> [--json]        开启 ADB WiFi 调试（一键完成）
  adb stop -s <集控平台> -k <密钥> [设备ID] [--json]   关闭 ADB WiFi 并断开隧道

日志命令:
  logs                     查看日志文件路径
  logs -f                  实时查看日志（tail -f）
  logs -n <行数>           查看最近 N 行日志

RPA 命令（本地数据库操作，无需 -s -k 参数）:
  rpa list                 列出所有 RPA 流程
  rpa show <id>            查看 RPA 详情
  rpa create --name <名称> 创建 RPA 流程
  rpa step add <id> ...    添加步骤
  rpa run <id> --device <设备ID>  执行 RPA
  rpa status --device <设备ID>    查看执行状态
  rpa history              查看执行历史
  （更多命令请执行 jpy-cloud rpa help）

其他:
  version                  显示版本
  help                     显示帮助

参数说明:
  -s, --server    集控平台地址（如 https://114.67.244.162）
  -k, --key       集控平台 API 密钥
  -v, --verbose   显示详细信息
  --json          输出 JSON 格式（不截断，适合程序解析）

工作原理:
  CLI 命令通过本地后端服务（127.0.0.1:1001）转发到集控平台。
  使用设备命令前，请确保本地服务已启动：jpy-cloud serve

示例:
  # 启动后端服务（保持终端运行）
  jpy-cloud serve

  # 另开终端，获取设备列表
  jpy-cloud devices -s https://114.67.244.162 -k your-api-key

  # 执行 Shell 命令
  jpy-cloud shell -s https://114.67.244.162 -k your-api-key 12345678 "ls -la"

  # 查看日志
  jpy-cloud logs -f
`, VERSION)
}

// parseServerKey 解析 -s 和 -k 参数
func parseServerKey(args []string) (server, key string, remaining []string) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-s", "--server":
			if i+1 < len(args) {
				server = args[i+1]
				i++
			}
		case "-k", "--key":
			if i+1 < len(args) {
				key = args[i+1]
				i++
			}
		default:
			remaining = append(remaining, args[i])
		}
	}
	return
}

func handleDevices() {
	server, key, remaining := parseServerKey(os.Args[2:])

	if server == "" {
		fmt.Println("错误: 缺少服务器地址")
		fmt.Println("用法: jpy-cloud devices -s <服务器> -k <密钥> [-v] [--json]")
		os.Exit(1)
	}

	// 检查参数
	verbose := false
	jsonOutput := false
	for _, arg := range remaining {
		if arg == "-v" || arg == "--verbose" {
			verbose = true
		}
		if arg == "--json" {
			jsonOutput = true
		}
	}

	// 检查服务是否运行
	if !cli.CheckServiceAndGuide() {
		os.Exit(1)
	}

	if err := cli.GetDevices(server, key, verbose, jsonOutput); err != nil {
		if jsonOutput {
			cli.OutputJSON(map[string]interface{}{"error": err.Error()})
		} else {
			fmt.Printf("获取设备失败: %v\n", err)
		}
		os.Exit(1)
	}
}

func handleShell() {
	server, key, remaining := parseServerKey(os.Args[2:])

	if server == "" {
		fmt.Println("错误: 缺少服务器地址")
		fmt.Println("用法: jpy-cloud shell -s <服务器> -k <密钥> <设备ID> <命令> [--json]")
		os.Exit(1)
	}

	// 过滤 --json 参数
	jsonOutput := false
	var args []string
	for _, arg := range remaining {
		if arg == "--json" {
			jsonOutput = true
		} else {
			args = append(args, arg)
		}
	}

	if len(args) < 2 {
		fmt.Println("错误: 缺少设备ID或命令")
		fmt.Println("用法: jpy-cloud shell -s <服务器> -k <密钥> <设备ID> <命令> [--json]")
		os.Exit(1)
	}

	deviceID := args[0]
	command := args[1]

	if err := cli.ExecuteShell(server, key, deviceID, command, jsonOutput); err != nil {
		if jsonOutput {
			cli.OutputJSON(map[string]interface{}{"error": err.Error()})
		} else {
			fmt.Printf("执行失败: %v\n", err)
		}
		os.Exit(1)
	}
}

func handleScreenshot() {
	server, key, remaining := parseServerKey(os.Args[2:])

	if server == "" {
		fmt.Println("错误: 缺少服务器地址")
		fmt.Println("用法: jpy-cloud screenshot -s <服务器> -k <密钥> <设备ID> [输出文件]")
		os.Exit(1)
	}

	if len(remaining) < 1 {
		fmt.Println("错误: 缺少设备ID")
		fmt.Println("用法: jpy-cloud screenshot -s <服务器> -k <密钥> <设备ID> [输出文件]")
		os.Exit(1)
	}

	deviceID := remaining[0]
	output := ""
	if len(remaining) > 1 {
		output = remaining[1]
	}

	if err := cli.Screenshot(server, key, deviceID, output); err != nil {
		fmt.Printf("截图失败: %v\n", err)
		os.Exit(1)
	}
}

func handleLogs() {
	// 日志可能在多个位置
	// 1. 服务模式：~/.jpy-cloud/stdout.log
	// 2. 直接运行：./logs/system.log 或 ./logs/unified_service.log
	dataDir := cli.GetDataDir()
	possibleLogs := []string{
		filepath.Join(dataDir, "stdout.log"),           // 系统服务日志
		"logs/system.log",                               // 直接运行时的日志
		"logs/unified_service.log",                      // 统一服务日志
	}

	// 解析参数
	follow := false
	lines := 50

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-f", "--follow":
			follow = true
		case "-n", "--lines":
			if i+1 < len(os.Args) {
				fmt.Sscanf(os.Args[i+1], "%d", &lines)
				i++
			}
		}
	}

	// 如果没有参数，显示日志路径
	if len(os.Args) == 2 {
		fmt.Println("日志文件路径:")
		for _, logPath := range possibleLogs {
			if _, err := os.Stat(logPath); err == nil {
				fmt.Printf("  ✓ %s (存在)\n", logPath)
			} else {
				fmt.Printf("  - %s (不存在)\n", logPath)
			}
		}
		fmt.Println("\n使用方法:")
		fmt.Println("  jpy-cloud logs -f        实时查看日志")
		fmt.Println("  jpy-cloud logs -n 100    查看最近 100 行")
		return
	}

	// 查找存在的日志文件
	var logFile string
	for _, logPath := range possibleLogs {
		if _, err := os.Stat(logPath); err == nil {
			logFile = logPath
			break
		}
	}

	if logFile == "" {
		fmt.Println("未找到日志文件，可能的位置:")
		for _, logPath := range possibleLogs {
			fmt.Printf("  - %s\n", logPath)
		}
		fmt.Println("\n服务可能尚未启动过，或日志目录不在当前路径")
		return
	}

	if follow {
		// 实时查看日志
		fmt.Printf("实时查看日志: %s\n", logFile)
		fmt.Println("按 Ctrl+C 退出\n")
		cli.TailFollow(logFile)
	} else {
		// 查看最近 N 行
		fmt.Printf("日志文件: %s\n\n", logFile)
		cli.TailLines(logFile, lines)
	}
}

func handleTunnel() {
	server, key, remaining := parseServerKey(os.Args[2:])

	if server == "" || key == "" {
		fmt.Println("错误: 缺少服务器地址或密钥")
		fmt.Println("用法: jpy-cloud tunnel -s <集控平台> -k <密钥> <设备ID> <本地端口> <远程端口> [--json]")
		os.Exit(1)
	}

	// 过滤 --json 参数
	jsonOutput := false
	var args []string
	for _, arg := range remaining {
		if arg == "--json" {
			jsonOutput = true
		} else {
			args = append(args, arg)
		}
	}

	if len(args) < 3 {
		fmt.Println("错误: 缺少参数")
		fmt.Println("用法: jpy-cloud tunnel -s <集控平台> -k <密钥> <设备ID> <本地端口> <远程端口> [--json]")
		os.Exit(1)
	}

	var deviceID, localPort, phonePort int
	fmt.Sscanf(args[0], "%d", &deviceID)
	fmt.Sscanf(args[1], "%d", &localPort)
	fmt.Sscanf(args[2], "%d", &phonePort)

	if deviceID == 0 || localPort == 0 || phonePort == 0 {
		fmt.Println("错误: 无效的设备ID或端口号")
		os.Exit(1)
	}

	if err := cli.Connect(server, key, deviceID, localPort, phonePort, jsonOutput); err != nil {
		if jsonOutput {
			cli.OutputJSON(map[string]interface{}{"error": err.Error()})
		} else {
			fmt.Printf("建立隧道失败: %v\n", err)
		}
		os.Exit(1)
	}
}

func handleDisconnect() {
	_, key, remaining := parseServerKey(os.Args[2:])

	if key == "" {
		fmt.Println("错误: 缺少密钥")
		fmt.Println("用法: jpy-cloud disconnect -k <密钥> <本地端口> [--json]")
		os.Exit(1)
	}

	// 过滤 --json 参数
	jsonOutput := false
	var args []string
	for _, arg := range remaining {
		if arg == "--json" {
			jsonOutput = true
		} else {
			args = append(args, arg)
		}
	}

	if len(args) < 1 {
		fmt.Println("错误: 缺少本地端口")
		fmt.Println("用法: jpy-cloud disconnect -k <密钥> <本地端口> [--json]")
		os.Exit(1)
	}

	var localPort int
	fmt.Sscanf(args[0], "%d", &localPort)

	if localPort == 0 {
		fmt.Println("错误: 无效的端口号")
		os.Exit(1)
	}

	if err := cli.Disconnect(key, localPort, jsonOutput); err != nil {
		if jsonOutput {
			cli.OutputJSON(map[string]interface{}{"error": err.Error()})
		} else {
			fmt.Printf("断开隧道失败: %v\n", err)
		}
		os.Exit(1)
	}
}

func handleMappings() {
	// 检查服务是否运行
	if !cli.CheckServiceAndGuide() {
		os.Exit(1)
	}

	// 检查 --json 参数
	jsonOutput := false
	for _, arg := range os.Args[2:] {
		if arg == "--json" {
			jsonOutput = true
			break
		}
	}

	if err := cli.GetMappings(jsonOutput); err != nil {
		if jsonOutput {
			cli.OutputJSON(map[string]interface{}{"error": err.Error()})
		} else {
			fmt.Printf("获取映射列表失败: %v\n", err)
		}
		os.Exit(1)
	}
}

func handleAdb() {
	if len(os.Args) < 3 {
		fmt.Println("用法:")
		fmt.Println("  jpy-cloud adb -s <集控平台> -k <密钥> <设备ID> [--json]   开启 ADB WiFi")
		fmt.Println("  jpy-cloud adb stop -s <集控平台> -k <密钥> <设备ID> [--json]   关闭 ADB WiFi")
		os.Exit(1)
	}

	// 检查服务是否运行
	if !cli.CheckServiceAndGuide() {
		os.Exit(1)
	}

	// 检查是否是 stop 子命令
	if os.Args[2] == "stop" {
		server, key, remaining := parseServerKey(os.Args[3:])
		if key == "" {
			fmt.Println("错误: 缺少密钥")
			fmt.Println("用法: jpy-cloud adb stop -s <集控平台> -k <密钥> [设备ID] [--json]")
			os.Exit(1)
		}

		jsonOutput := false
		deviceID := 0
		for _, arg := range remaining {
			if arg == "--json" {
				jsonOutput = true
			} else {
				fmt.Sscanf(arg, "%d", &deviceID)
			}
		}

		if err := cli.DisableAdbWifi(server, key, deviceID, jsonOutput); err != nil {
			if jsonOutput {
				cli.OutputJSON(map[string]interface{}{"error": err.Error()})
			} else {
				fmt.Printf("关闭 ADB WiFi 失败: %v\n", err)
			}
			os.Exit(1)
		}
		return
	}

	// 开启 ADB WiFi
	server, key, remaining := parseServerKey(os.Args[2:])

	if server == "" || key == "" {
		fmt.Println("错误: 缺少服务器地址或密钥")
		fmt.Println("用法: jpy-cloud adb -s <集控平台> -k <密钥> <设备ID> [--json]")
		os.Exit(1)
	}

	// 过滤 --json 参数并获取设备ID
	jsonOutput := false
	var deviceIDStr string
	for _, arg := range remaining {
		if arg == "--json" {
			jsonOutput = true
		} else if deviceIDStr == "" {
			deviceIDStr = arg
		}
	}

	if deviceIDStr == "" {
		fmt.Println("错误: 缺少设备ID")
		fmt.Println("用法: jpy-cloud adb -s <集控平台> -k <密钥> <设备ID> [--json]")
		os.Exit(1)
	}

	var deviceID int
	fmt.Sscanf(deviceIDStr, "%d", &deviceID)
	if deviceID == 0 {
		fmt.Println("错误: 无效的设备ID")
		os.Exit(1)
	}

	if err := cli.EnableAdbWifi(server, key, deviceID, jsonOutput); err != nil {
		if jsonOutput {
			cli.OutputJSON(map[string]interface{}{"error": err.Error()})
		} else {
			fmt.Printf("开启 ADB WiFi 失败: %v\n", err)
		}
		os.Exit(1)
	}
}

func runServer() {
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

	r.GET("/api/logs/download", func(c *gin.Context) {
		logFile := "logs/unified_service.log"
		c.Header("Content-Disposition", "attachment; filename=unified_service.log")
		c.Header("Content-Type", "application/octet-stream")
		c.File(logFile)
	})

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "version": VERSION})
	})

	// Version API
	r.GET("/api/version", func(c *gin.Context) {
		c.JSON(200, gin.H{"version": VERSION})
	})

	// WebSocket support for Unified Request
	r.GET("/api/unified/ws", service.UnifiedWSHandler)

	// RPA Routes
	rpa.RegisterRoutes(r.Group("/api"))

	// Device WS API Routes
	deviceServer := devicews.NewServer("0.0.0.0:1003")
	deviceServer.RegisterAPIRoutes(r.Group("/api/devicews"))

	// Bind registered routes to Gin
	framework.BindHTTP(r)

	// Documentation Endpoint
	r.GET("/doc", func(c *gin.Context) {
		html := framework.RenderDocHTML()
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, html)
	})

	// Static files
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

	// SPA fallback
	r.NoRoute(func(c *gin.Context) {
		if len(c.Request.URL.Path) > 4 && c.Request.URL.Path[:4] == "/api" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not Found"})
			return
		}
		data, err := fs.ReadFile(staticSubFS, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	// Start WebSocket Server
	go framework.StartWS(1002)

	// Start Device WebSocket Server
	deviceServer.GetManager().SetOnConnect(func(dc *devicews.DeviceConn) {
		logs.Info("[DeviceWS] 设备上线: %08X (%s)", dc.DeviceID, dc.Serialno)
	})
	deviceServer.GetManager().SetOnDisconnect(func(dc *devicews.DeviceConn) {
		logs.Info("[DeviceWS] 设备离线: %08X (%s)", dc.DeviceID, dc.Serialno)
	})

	service.SetDeviceWSServer(deviceServer)

	go func() {
		if err := deviceServer.Start(); err != nil {
			logs.Error("[DeviceWS] 服务启动失败: %v", err)
		}
	}()

	// Start Server
	logs.Info("JPY Server v%s 已启动", VERSION)
	logs.Info("API服务端口: 1001")
	logs.Info("WebSocket端口: 1002")
	logs.Info("设备连接端口: 1003")

	if err := r.SetTrustedProxies(nil); err != nil {
		logs.Error("Failed to set trusted proxies", err)
	}

	if err := r.Run("0.0.0.0:1001"); err != nil {
		logs.Error("Web服务启动失败", err)
	}
}
