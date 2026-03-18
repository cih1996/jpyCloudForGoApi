# JPY Cloud CLI 使用手册

## 概述

JPY Cloud 是集控平台的本地代理服务，支持：
- 作为后台服务运行（支持开机自启）
- CLI 命令行操作（设备管理、RPA 自动化、Shell 执行等）
- Web 管理界面
- 跨平台支持（Windows、macOS、Linux）

## 快速开始

### 1. 启动服务

```bash
# 前台运行（调试用）
jpy-cloud serve

# 或安装为系统服务（推荐）
jpy-cloud service install
jpy-cloud service start
```

服务启动后：
- HTTP API + Web 界面：http://localhost:1001
- WebSocket 通信：ws://localhost:1002
- 设备连接：端口 1003

### 2. 连接集控平台

```bash
# 获取设备列表（验证连接）
jpy-cloud devices -s https://114.67.244.162 -k 100064d75558efc80ba36c644b6aca1be76501773238050662

# 参数说明：
# -s, --server  集控平台地址
# -k, --key     API 密钥（从集控平台获取）
```

### 3. 基本操作

```bash
# 查看设备列表
jpy-cloud devices -s <服务器> -k <密钥>

# 执行 Shell 命令
jpy-cloud shell -s <服务器> -k <密钥> <设备ID> "ls -la"

# 截图
jpy-cloud screenshot -s <服务器> -k <密钥> <设备ID>
```

---

## 命令参考

### 安装/卸载

```bash
jpy-cloud install      # 安装程序到系统
jpy-cloud uninstall    # 卸载程序和服务
jpy-cloud upgrade      # 升级到新版本（用新版本执行）
jpy-cloud version      # 查看版本
```

### 服务管理

```bash
jpy-cloud serve              # 前台运行服务
jpy-cloud service install    # 安装为系统服务
jpy-cloud service uninstall  # 卸载系统服务
jpy-cloud service start      # 启动服务
jpy-cloud service stop       # 停止服务
jpy-cloud service restart    # 重启服务
jpy-cloud service status     # 查看状态
```

### 设备操作

所有设备命令都需要 `-s`（服务器地址）和 `-k`（API 密钥）参数。

```bash
# 获取设备列表
jpy-cloud devices -s <服务器> -k <密钥>
jpy-cloud devices -s <服务器> -k <密钥> --json  # JSON 格式输出

# 执行 Shell 命令
jpy-cloud shell -s <服务器> -k <密钥> <设备ID> "<命令>"

# 截图
jpy-cloud screenshot -s <服务器> -k <密钥> <设备ID> [输出文件]

# 示例
jpy-cloud devices -s https://114.67.244.162 -k your-api-key
jpy-cloud shell -s https://114.67.244.162 -k your-api-key 12345678 "ls -la /sdcard/"
jpy-cloud screenshot -s https://114.67.244.162 -k your-api-key 12345678 screen.png
```

---

## RPA 命令

RPA 命令用于管理和执行自动化流程。**注意：RPA 命令需要本地服务运行。**

### 流程管理

```bash
# 列出所有 RPA 流程
jpy-cloud rpa list
jpy-cloud rpa list --json  # JSON 格式

# 查看 RPA 详情
jpy-cloud rpa show <id>
jpy-cloud rpa show <id> --json

# 创建 RPA 流程
jpy-cloud rpa create --name "流程名称" [--desc "描述"]

# 删除 RPA 流程
jpy-cloud rpa delete <id>
```

### 步骤管理

```bash
# 列出步骤
jpy-cloud rpa step list <rpa_id>

# 添加步骤
jpy-cloud rpa step add <rpa_id> --type <类型> --name <名称> [--params <JSON>]

# 删除步骤（index 从 0 开始）
jpy-cloud rpa step remove <rpa_id> <index>

# 移动步骤位置
jpy-cloud rpa step move <rpa_id> <from> <to>
```

**步骤类型：**

| 类型 | 说明 | 参数示例 |
|------|------|----------|
| shell | 执行 Shell 命令 | `{"command": "ls -la"}` |
| start_bot | 启动脚本 | `{"scriptId": 1}` |
| change_os | 改机重启 | `{}` |
| download_url | 下载安装应用 | `{"url": "http://...", "package": "com.app"}` |
| set_proxy | 设置代理 | `{"host": "127.0.0.1", "port": 1080}` |
| set_location | 设置定位 | `{"lat": 39.9, "lng": 116.4}` |
| get_root | 获取 Root 权限 | `{}` |
| http_request | HTTP 请求 | `{"url": "http://...", "method": "GET"}` |
| condition | 条件判断 | `{"condition": "..."}` |
| set_variables | 设置变量 | `{"key": "value"}` |

### 执行控制

```bash
# 执行 RPA（在指定设备上）
jpy-cloud rpa run <rpa_id> --device <device_id> [--mode single|loop]

# 停止执行
jpy-cloud rpa stop --device <device_id>

# 查看执行状态
jpy-cloud rpa status --device <device_id>
jpy-cloud rpa status --device <device_id> --json

# 查看执行历史
jpy-cloud rpa history [--device <device_id>] [--limit <n>]
```

### RPA 使用示例

```bash
# 1. 创建一个 RPA 流程
jpy-cloud rpa create --name "自动化测试"
# 输出: 创建成功，ID: 29

# 2. 添加步骤
jpy-cloud rpa step add 29 --type shell --name "清理缓存" --params '{"command":"rm -rf /data/cache/*"}'
jpy-cloud rpa step add 29 --type change_os --name "改机重启"
jpy-cloud rpa step add 29 --type start_bot --name "启动脚本" --params '{"scriptId":1}'

# 3. 查看流程详情
jpy-cloud rpa show 29

# 4. 在设备上执行
jpy-cloud rpa run 29 --device 12345678

# 5. 查看执行状态
jpy-cloud rpa status --device 12345678

# 6. 查看执行历史
jpy-cloud rpa history --limit 10
```

---

## 参数说明

| 参数 | 简写 | 说明 |
|------|------|------|
| `--server` | `-s` | 集控平台地址（如 https://example.com） |
| `--key` | `-k` | API 密钥 |
| `--json` | | JSON 格式输出 |
| `--verbose` | `-v` | 详细输出 |

## 服务端口

| 端口 | 用途 |
|------|------|
| 1001 | HTTP API + Web 界面 |
| 1002 | WebSocket 通信 |
| 1003 | 设备连接 |

## 安装路径

| 系统 | 安装路径 |
|------|----------|
| macOS | `/usr/local/bin/jpy-cloud` |
| Linux | `/usr/local/bin/jpy-cloud` |
| Windows | `%LOCALAPPDATA%\jpy-cloud\jpy-cloud.exe` |

## 日志文件

| 系统 | 日志路径 |
|------|----------|
| macOS/Linux | `~/.jpy-cloud/stdout.log`, `~/.jpy-cloud/stderr.log` |
| Windows | `%LOCALAPPDATA%\jpy-cloud\logs\` |

## 系统服务配置

### macOS (launchd)

配置文件：`~/Library/LaunchAgents/com.jpy.server.plist`

```bash
launchctl load ~/Library/LaunchAgents/com.jpy.server.plist
launchctl unload ~/Library/LaunchAgents/com.jpy.server.plist
launchctl list | grep jpy
```

### Linux (systemd)

配置文件：`~/.config/systemd/user/jpy-cloud.service`

```bash
systemctl --user daemon-reload
systemctl --user enable jpy-cloud
systemctl --user disable jpy-cloud
journalctl --user -u jpy-cloud -f
```

### Windows

```cmd
sc start jpy-cloud
sc stop jpy-cloud
sc query jpy-cloud
```

---

## 常见问题

### Q: RPA 命令报错"请求失败（本地服务是否已启动？）"

RPA 命令需要本地服务运行。先启动服务：

```bash
jpy-cloud serve
# 或
jpy-cloud service start
```

### Q: CLI 命令输出 debug 日志？

可以通过重定向 stderr 过滤：

```bash
jpy-cloud devices -s https://example.com -k key 2>/dev/null

# 创建别名
alias jpy='jpy-cloud 2>/dev/null'
```

### Q: 服务无法启动？

1. 检查端口是否被占用：
   ```bash
   lsof -i :1001  # macOS/Linux
   netstat -ano | findstr :1001  # Windows
   ```

2. 查看日志：
   ```bash
   cat ~/.jpy-cloud/stderr.log
   ```

### Q: 如何完全卸载？

```bash
jpy-cloud uninstall
```

---

## API 文档

启动服务后访问：http://localhost:1001/doc
