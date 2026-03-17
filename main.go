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

const VERSION = "1.0.3"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := os.Args[1]

	switch cmd {
	case "serve", "server", "run":
		runServer()
	case "install":
		handleInstall()
	case "uninstall":
		handleUninstall()
	case "upgrade", "update":
		handleUpgrade()
	case "service":
		handleService()
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
	case "version", "-v", "--version":
		fmt.Printf("jpy-server version %s (%s/%s)\n", VERSION, runtime.GOOS, runtime.GOARCH)
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
  jpy-server <命令> [参数]

安装命令:
  install                  安装程序到系统并添加到 PATH
  uninstall                卸载程序和服务
  upgrade                  升级到当前版本

服务命令:
  serve                    启动服务（前台运行）
  service install          安装为系统服务（开机自启）
  service uninstall        卸载系统服务
  service start            启动服务
  service stop             停止服务
  service status           查看服务状态
  service restart          重启服务

设备命令（需要 -s 和 -k 参数，本地服务必须已启动，支持 --json 输出）:
  devices -s <集控平台> -k <密钥> [-v] [--json]           获取设备列表
  shell -s <集控平台> -k <密钥> <设备ID> <命令> [--json]   执行 Shell 命令
  screenshot -s <集控平台> -k <密钥> <设备ID>             截图

隧道命令（端口映射/打洞，支持 --json 输出）:
  tunnel -s <集控平台> -k <密钥> <设备ID> <本地端口> <远程端口> [--json]  建立端口映射
  disconnect -k <密钥> <本地端口> [--json]                               断开端口映射
  mappings [--json]                                                     查看当前所有端口映射

日志命令:
  logs                     查看日志文件路径
  logs -f                  实时查看日志（tail -f）
  logs -n <行数>           查看最近 N 行日志

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
  使用设备命令前，请确保本地服务已启动：jpy-server service start

示例:
  # 安装程序
  sudo ./jpy-server install

  # 安装并启动服务
  jpy-server service install
  jpy-server service start

  # 获取设备列表
  jpy-server devices -s https://114.67.244.162 -k your-api-key

  # 执行 Shell 命令
  jpy-server shell -s https://114.67.244.162 -k your-api-key 12345678 "ls -la"

  # 查看日志
  jpy-server logs -f
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

func handleInstall() {
	binPath, _ := os.Executable()
	binPath, _ = filepath.Abs(binPath)

	if err := cli.Install(binPath); err != nil {
		fmt.Printf("安装失败: %v\n", err)
		os.Exit(1)
	}
}

func handleUninstall() {
	if err := cli.Uninstall(); err != nil {
		fmt.Printf("卸载失败: %v\n", err)
		os.Exit(1)
	}
}

func handleUpgrade() {
	binPath, _ := os.Executable()
	binPath, _ = filepath.Abs(binPath)

	if err := cli.Upgrade(binPath); err != nil {
		fmt.Printf("升级失败: %v\n", err)
		os.Exit(1)
	}
}

func handleService() {
	if len(os.Args) < 3 {
		fmt.Println("用法: jpy-server service <install|uninstall|start|stop|status|restart>")
		return
	}

	action := os.Args[2]
	binPath := cli.GetInstallPath()
	workDir := cli.GetDataDir()

	// 确保工作目录存在
	os.MkdirAll(workDir, 0755)

	switch action {
	case "install":
		// 检查程序是否已安装
		if _, err := os.Stat(binPath); os.IsNotExist(err) {
			fmt.Printf("错误: 程序未安装到 %s\n", binPath)
			fmt.Println("请先执行: sudo ./jpy-server install")
			os.Exit(1)
		}
		if err := cli.ServiceInstall(binPath, workDir); err != nil {
			fmt.Printf("安装失败: %v\n", err)
		}
	case "uninstall":
		if err := cli.ServiceUninstall(); err != nil {
			fmt.Printf("卸载失败: %v\n", err)
		}
	case "start":
		// 检查程序是否已安装
		if _, err := os.Stat(binPath); os.IsNotExist(err) {
			fmt.Printf("错误: 程序未安装到 %s\n", binPath)
			fmt.Println("请先执行: sudo ./jpy-server install")
			os.Exit(1)
		}
		if err := cli.ServiceStart(); err != nil {
			fmt.Printf("启动失败: %v\n", err)
		} else {
			fmt.Println("服务已启动")
		}
	case "stop":
		if err := cli.ServiceStop(); err != nil {
			fmt.Printf("停止失败: %v\n", err)
		} else {
			fmt.Println("服务已停止")
		}
	case "restart":
		// 检查程序是否已安装
		if _, err := os.Stat(binPath); os.IsNotExist(err) {
			fmt.Printf("错误: 程序未安装到 %s\n", binPath)
			fmt.Println("请先执行: sudo ./jpy-server install")
			os.Exit(1)
		}
		cli.ServiceStop()
		if err := cli.ServiceStart(); err != nil {
			fmt.Printf("重启失败: %v\n", err)
		} else {
			fmt.Println("服务已重启")
		}
	case "status":
		cli.ServiceStatus()
	default:
		fmt.Printf("未知操作: %s\n", action)
	}
}

func handleDevices() {
	server, key, remaining := parseServerKey(os.Args[2:])

	if server == "" {
		fmt.Println("错误: 缺少服务器地址")
		fmt.Println("用法: jpy-server devices -s <服务器> -k <密钥> [-v] [--json]")
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
		fmt.Println("用法: jpy-server shell -s <服务器> -k <密钥> <设备ID> <命令> [--json]")
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
		fmt.Println("用法: jpy-server shell -s <服务器> -k <密钥> <设备ID> <命令> [--json]")
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
		fmt.Println("用法: jpy-server screenshot -s <服务器> -k <密钥> <设备ID> [输出文件]")
		os.Exit(1)
	}

	if len(remaining) < 1 {
		fmt.Println("错误: 缺少设备ID")
		fmt.Println("用法: jpy-server screenshot -s <服务器> -k <密钥> <设备ID> [输出文件]")
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
	dataDir := cli.GetDataDir()
	stdoutLog := filepath.Join(dataDir, "stdout.log")
	stderrLog := filepath.Join(dataDir, "stderr.log")

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
		fmt.Printf("  标准输出: %s\n", stdoutLog)
		fmt.Printf("  错误输出: %s\n", stderrLog)
		fmt.Println("\n使用方法:")
		fmt.Println("  jpy-server logs -f        实时查看日志")
		fmt.Println("  jpy-server logs -n 100    查看最近 100 行")
		return
	}

	// 检查日志文件是否存在
	if _, err := os.Stat(stdoutLog); os.IsNotExist(err) {
		fmt.Printf("日志文件不存在: %s\n", stdoutLog)
		fmt.Println("服务可能尚未启动过")
		return
	}

	if follow {
		// 实时查看日志
		fmt.Printf("实时查看日志: %s\n", stdoutLog)
		fmt.Println("按 Ctrl+C 退出\n")
		cli.TailFollow(stdoutLog)
	} else {
		// 查看最近 N 行
		cli.TailLines(stdoutLog, lines)
	}
}

func handleTunnel() {
	server, key, remaining := parseServerKey(os.Args[2:])

	if server == "" || key == "" {
		fmt.Println("错误: 缺少服务器地址或密钥")
		fmt.Println("用法: jpy-server tunnel -s <集控平台> -k <密钥> <设备ID> <本地端口> <远程端口> [--json]")
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
		fmt.Println("用法: jpy-server tunnel -s <集控平台> -k <密钥> <设备ID> <本地端口> <远程端口> [--json]")
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
		fmt.Println("用法: jpy-server disconnect -k <密钥> <本地端口> [--json]")
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
		fmt.Println("用法: jpy-server disconnect -k <密钥> <本地端口> [--json]")
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
