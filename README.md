# JPY Server

集控平台本地代理服务，支持 Web 界面和 CLI 命令行操作。

## ✨ 核心特性

- 🖥️ **Web 管理界面**：设备管理、截图、Shell、OCR 识别
- 🔧 **CLI 命令行工具**：独立运行，无需配置文件
- 🚀 **系统服务**：支持开机自启（macOS/Linux/Windows）
- 📦 **单文件部署**：前端嵌入二进制，无需额外静态文件
- 🌍 **跨平台支持**：Windows、macOS、Linux
- 🔌 **统一 API 架构**：HTTP/WebSocket 双协议支持

## 🛠️ 快速开始

### 下载安装

从 [Releases](https://github.com/cih1996/jpyCloudForGoApi/releases) 下载对应平台的二进制文件。

**macOS / Linux:**
```bash
chmod +x jpy-cloud-darwin-arm64
sudo ./jpy-cloud-darwin-arm64 install
```

**Windows (管理员权限):**
```cmd
.\jpy-cloud-windows-amd64.exe install
```

### 启动服务

```bash
# 安装为系统服务（开机自启）
jpy-cloud service install
jpy-cloud service start

# 或前台运行（调试用）
jpy-cloud serve
```

### 访问 Web 界面

启动后访问：http://localhost:1001

## 📖 CLI 命令

### 安装/卸载

```bash
jpy-cloud install        # 安装程序到系统
jpy-cloud uninstall      # 卸载程序和服务
./jpy-cloud-new upgrade  # 升级到新版本
```

### 服务管理

```bash
jpy-cloud service install    # 安装为系统服务
jpy-cloud service uninstall  # 卸载系统服务
jpy-cloud service start      # 启动服务
jpy-cloud service stop       # 停止服务
jpy-cloud service restart    # 重启服务
jpy-cloud service status     # 查看状态
```

### 设备操作

所有设备命令需要 `-s`（服务器地址）和 `-k`（API 密钥）参数：

```bash
# 获取设备列表
jpy-cloud devices -s https://example.com -k your-api-key

# 执行 Shell 命令
jpy-cloud shell -s https://example.com -k your-api-key 12345678 "ls -la"

# 截图
jpy-cloud screenshot -s https://example.com -k your-api-key 12345678
jpy-cloud screenshot -s https://example.com -k your-api-key 12345678 output.png
```

### 其他

```bash
jpy-cloud version  # 查看版本
jpy-cloud help     # 查看帮助
```

## 🔌 端口说明

| 端口 | 用途 |
|------|------|
| 1001 | HTTP API + Web 界面 |
| 1002 | WebSocket 通信 |
| 1003 | 设备连接 |

## 📁 安装路径

| 系统 | 程序路径 | 数据目录 |
|------|----------|----------|
| macOS | `/usr/local/bin/jpy-cloud` | `~/.jpy-cloud/` |
| Linux | `/usr/local/bin/jpy-cloud` | `~/.jpy-cloud/` |
| Windows | `%LOCALAPPDATA%\jpy-cloud\jpy-cloud.exe` | `%LOCALAPPDATA%\jpy-cloud\` |

## ⚙️ 系统服务配置

### macOS (launchd)

配置文件：`~/Library/LaunchAgents/com.jpy.server.plist`

```bash
launchctl list | grep jpy  # 查看状态
```

### Linux (systemd)

配置文件：`~/.config/systemd/user/jpy-cloud.service`

```bash
journalctl --user -u jpy-cloud -f  # 查看日志
```

### Windows

```cmd
sc query jpy-cloud  # 查看状态
```

## 📝 日志文件

| 系统 | 路径 |
|------|------|
| macOS/Linux | `~/.jpy-cloud/stdout.log`, `~/.jpy-cloud/stderr.log` |
| Windows | `%LOCALAPPDATA%\jpy-cloud\logs\` |

## 🔧 开发构建

### 前置条件

- Go 1.21+
- Node.js 18+ (前端)
- Docker (可选)

### 构建命令

```bash
# 开发构建（前端 + 后端 + 重启服务）
make dev

# 仅构建后端
make build

# 多平台打包
make dist-all
```

### 🐳 Docker 部署

```bash
# 使用 Docker Compose
docker compose up -d --build

# 或手动构建
docker build -t go-port-trans .
docker run -d -p 1001:1001 -p 1002:1002 --name port-trans go-port-trans
```

## 📚 API 文档

访问 `http://localhost:1001/doc` 查看交互式 API 文档。

### WebSocket 协议

**连接地址**: `ws://<host>:1002`

**请求格式**:
```json
{
  "path": "/api/connect",
  "id": "unique-req-id",
  "data": { "key": "...", "deviceId": 123 }
}
```

**响应格式**:
```json
{
  "id": "unique-req-id",
  "path": "/api/connect",
  "success": true,
  "message": "",
  "data": { ... }
}
```

## ❓ 常见问题

### CLI 输出 debug 日志？

第三方库可能输出 debug 日志，可通过重定向过滤：

```bash
jpy-cloud devices -s https://example.com -k key 2>/dev/null
```

### 服务无法启动？

1. 检查端口占用：`lsof -i :1001`
2. 查看日志：`cat ~/.jpy-cloud/stderr.log`

### Windows 安装失败？

需要以管理员身份运行命令提示符或 PowerShell。

## 📂 项目结构

```
.
├── main.go                 # 应用入口与 CLI
├── cmd/cli/                # CLI 实现
├── internal/
│   ├── config/             # 配置管理
│   ├── service/            # 业务逻辑
│   ├── devicews/           # 设备 WebSocket
│   ├── rpa/                # RPA 引擎
│   └── database/           # 数据库
├── pkg/
│   ├── framework/          # Web/WS 框架
│   └── logger/             # 日志
├── vue-app/                # 前端源码
└── static/                 # 编译后的前端（embed）
```

## 🔗 关联项目

- [JpyApiAgent](https://github.com/cih1996/JpyApiAgent) - 通讯框架

## License

MIT
