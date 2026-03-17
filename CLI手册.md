# JPY Server CLI 使用手册

## 概述

JPY Server 是集控平台的本地代理服务，支持：
- 作为后台服务运行（支持开机自启）
- CLI 命令行操作（设备管理、Shell 执行等）
- 跨平台支持（Windows、macOS、Linux）

## 安装

### 方式一：自动安装（推荐）

下载对应平台的程序后，运行 install 命令：

```bash
# macOS / Linux
./jpy-server install

# Windows (管理员权限)
.\jpy-server.exe install
```

程序会自动安装到系统目录并添加到 PATH。

### 方式二：手动安装

**macOS / Linux:**
```bash
sudo cp jpy-server /usr/local/bin/
sudo chmod +x /usr/local/bin/jpy-server
```

**Windows:**
1. 将程序放到 `%LOCALAPPDATA%\jpy-server\`
2. 添加该目录到系统 PATH 环境变量

### 安装路径

| 系统 | 安装路径 |
|------|----------|
| macOS | `/usr/local/bin/jpy-server` |
| Linux | `/usr/local/bin/jpy-server` |
| Windows | `%LOCALAPPDATA%\jpy-server\jpy-server.exe` |

## 命令参考

### 安装/卸载

```bash
# 安装程序到系统
jpy-server install

# 卸载程序和服务
jpy-server uninstall

# 升级到新版本
./jpy-server-new upgrade
```

### 服务管理

```bash
# 启动服务（前台运行，用于调试）
jpy-server serve

# 安装为系统服务（开机自启）
jpy-server service install

# 卸载系统服务
jpy-server service uninstall

# 启动/停止/重启服务
jpy-server service start
jpy-server service stop
jpy-server service restart

# 查看服务状态
jpy-server service status
```

### 设备操作

所有设备命令都需要 `-s` 和 `-k` 参数：

```bash
# 获取设备列表
jpy-server devices -s <服务器地址> -k <API密钥>

# 示例
jpy-server devices -s https://114.67.244.162 -k your-api-key
```

```bash
# 执行 Shell 命令
jpy-server shell -s <服务器> -k <密钥> <设备ID> "<命令>"

# 示例
jpy-server shell -s https://114.67.244.162 -k your-api-key 12345678 "ls -la /sdcard/"
```

```bash
# 截图
jpy-server screenshot -s <服务器> -k <密钥> <设备ID> [输出文件]

# 示例
jpy-server screenshot -s https://114.67.244.162 -k your-api-key 12345678
jpy-server screenshot -s https://114.67.244.162 -k your-api-key 12345678 screen.png
```

### 其他

```bash
# 查看版本
jpy-server version

# 查看帮助
jpy-server help
```

## 参数说明

| 参数 | 简写 | 说明 |
|------|------|------|
| `--server` | `-s` | 服务器地址（如 https://example.com） |
| `--key` | `-k` | API 密钥 |

## 服务端口

| 端口 | 用途 |
|------|------|
| 1001 | HTTP API + Web 界面 |
| 1002 | WebSocket 通信 |
| 1003 | 设备连接 |

## 系统服务配置

### macOS (launchd)

配置文件：`~/Library/LaunchAgents/com.jpy.server.plist`

```bash
# 手动加载/卸载
launchctl load ~/Library/LaunchAgents/com.jpy.server.plist
launchctl unload ~/Library/LaunchAgents/com.jpy.server.plist

# 查看状态
launchctl list | grep jpy
```

### Linux (systemd)

配置文件：`~/.config/systemd/user/jpy-server.service`

```bash
# 重载配置
systemctl --user daemon-reload

# 启用/禁用开机自启
systemctl --user enable jpy-server
systemctl --user disable jpy-server

# 查看日志
journalctl --user -u jpy-server -f
```

### Windows

使用 Windows 服务管理器或命令行（需要管理员权限）：

```cmd
# 启动/停止
sc start jpy-server
sc stop jpy-server

# 查看状态
sc query jpy-server
```

## 日志文件

| 系统 | 日志路径 |
|------|----------|
| macOS | `~/.jpy-server/stdout.log`, `~/.jpy-server/stderr.log` |
| Linux | `~/.jpy-server/stdout.log`, `~/.jpy-server/stderr.log` |
| Windows | `%LOCALAPPDATA%\jpy-server\logs\` |

## 常见问题

### Q: CLI 命令输出 debug 日志？

CLI 命令可能会输出一些第三方库的 debug 日志，可以通过重定向 stderr 过滤：

```bash
# macOS / Linux
jpy-server devices -s https://example.com -k key 2>/dev/null

# 创建别名
alias jpy='jpy-server 2>/dev/null'
```

### Q: 服务无法启动？

1. 检查端口是否被占用：
   ```bash
   # macOS / Linux
   lsof -i :1001

   # Windows
   netstat -ano | findstr :1001
   ```

2. 查看日志：
   ```bash
   cat ~/.jpy-server/stderr.log
   ```

### Q: Windows 安装失败？

需要以管理员身份运行命令提示符或 PowerShell。

### Q: 如何完全卸载？

```bash
jpy-server uninstall
```

这会自动停止服务、卸载服务配置、删除程序和数据目录。

## API 文档

启动服务后访问：`http://localhost:1001/doc`
