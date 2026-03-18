# JPY Cloud

集控平台本地代理服务，支持 Web 界面和 CLI 命令行操作。

## 核心特性

- **Web 管理界面**：设备管理、截图、Shell、OCR 识别
- **CLI 命令行工具**：独立运行，无需配置文件
- **单文件部署**：前端嵌入二进制，无需额外静态文件
- **跨平台支持**：Windows、macOS、Linux
- **统一 API 架构**：HTTP/WebSocket 双协议支持

## 快速开始

### 下载

从 [Releases](https://github.com/cih1996/jpyCloudForGoApi/releases) 下载对应平台的二进制文件。

### 启动服务

**Windows:**
```cmd
jpy-cloud.exe serve
```

**macOS / Linux:**
```bash
chmod +x jpy-cloud
./jpy-cloud serve
```

保持终端运行，服务启动后访问：http://localhost:1001

## CLI 命令

### 设备操作

所有设备命令需要 `-s`（服务器地址）和 `-k`（API 密钥）参数，且需要先启动服务：

```bash
# 获取设备列表
jpy-cloud devices -s https://example.com -k your-api-key
jpy-cloud devices -s https://example.com -k your-api-key -v          # 详细模式
jpy-cloud devices -s https://example.com -k your-api-key --json      # JSON 输出

# 执行 Shell 命令
jpy-cloud shell -s https://example.com -k your-api-key 12345678 "ls -la"

# 截图
jpy-cloud screenshot -s https://example.com -k your-api-key 12345678
```

### ADB 调试（一键开启）

```bash
# 开启 ADB WiFi 调试
jpy-cloud adb -s https://example.com -k your-api-key 12345678

# 完成后连接设备
adb connect 127.0.0.1:5555
adb devices

# 关闭 ADB WiFi
jpy-cloud adb stop -s https://example.com -k your-api-key 12345678
```

### 端口映射（隧道）

```bash
# 建立端口映射
jpy-cloud tunnel -s https://example.com -k your-api-key 12345678 5555 5555

# 查看当前映射
jpy-cloud mappings

# 断开映射
jpy-cloud disconnect -k your-api-key 5555
```

### 日志查看

```bash
jpy-cloud logs           # 显示日志文件路径
jpy-cloud logs -n 100    # 查看最近 100 行
jpy-cloud logs -f        # 实时查看
```

### 其他

```bash
jpy-cloud version  # 查看版本
jpy-cloud help     # 查看帮助
```

## 端口说明

| 端口 | 用途 |
|------|------|
| 1001 | HTTP API + Web 界面 |
| 1002 | WebSocket 通信 |
| 1003 | 设备连接 |

## 日志文件

运行目录下的 `./logs/system.log`

## 常见问题

### 服务未启动时使用 CLI？

会提示：
```
错误: 本地服务未运行
请先在另一个终端运行: jpy-cloud serve
```

### CLI 输出 debug 日志？

第三方库可能输出 debug 日志，可通过重定向过滤：
```bash
jpy-cloud devices -s https://example.com -k key 2>/dev/null
```

### 服务无法启动？

1. 检查端口占用：`lsof -i :1001`（macOS/Linux）或 `netstat -ano | findstr 1001`（Windows）
2. 查看日志：`jpy-cloud logs -n 50`

## 开发构建

```bash
# 开发构建（前端 + 后端）
make dev

# 多平台打包
make dist-all
```

## License

MIT
