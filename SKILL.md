# JPY Cloud 技能手册

> 本手册供 AI 智能体使用，包含集控平台操作的完整流程。

## 概述

JPY Cloud 是集控平台的本地代理服务，用于：
- 管理云手机设备
- 执行 RPA 自动化流程
- 远程执行 Shell 命令
- 设备截图等操作

## 环境要求

- 程序路径：`/Users/cih1996/work/jpy-zyd/ts-sdk/go-port-trans/dist/jpy-cloud-darwin-arm64`（macOS ARM）
- 服务端口：1001（HTTP）、1002（WebSocket）、1003（设备连接）

## 测试服务器

```
地址：https://114.67.244.162/
TOKEN：100064d75558efc80ba36c644b6aca1be76501773238050662
```

---

## 操作流程

### 1. 启动服务

RPA 命令需要本地服务运行，先检查并启动：

```bash
# 检查服务状态
curl -s http://127.0.0.1:1001/health | jq

# 如果服务未运行，启动服务（前台）
cd /Users/cih1996/work/jpy-zyd/ts-sdk/go-port-trans/dist
./jpy-cloud serve

# 或通过 ai-hub services 管理
ai-hub services start 集控平台
```

### 2. 连接集控平台

```bash
# 验证连接 - 获取设备列表
./jpy-cloud devices -s https://114.67.244.162 -k 100064d75558efc80ba36c644b6aca1be76501773238050662

# 成功输出示例：
# ID          状态    IP              名称
# 12345678    在线    192.168.1.100   测试设备1
```

### 3. 设备操作

```bash
# 获取设备列表
./jpy-cloud devices -s <服务器> -k <密钥>

# 执行 Shell 命令
./jpy-cloud shell -s <服务器> -k <密钥> <设备ID> "命令"

# 截图
./jpy-cloud screenshot -s <服务器> -k <密钥> <设备ID> [输出文件]
```

### 4. RPA 操作

#### 4.1 查看 RPA 列表

```bash
./jpy-cloud rpa list
```

#### 4.2 创建 RPA 流程

```bash
./jpy-cloud rpa create --name "流程名称" --desc "描述"
```

#### 4.3 添加步骤

```bash
# Shell 命令
./jpy-cloud rpa step add <rpa_id> --type shell --name "步骤名" --params '{"command":"ls -la"}'

# 改机重启
./jpy-cloud rpa step add <rpa_id> --type change_os --name "改机"

# 启动脚本
./jpy-cloud rpa step add <rpa_id> --type start_bot --name "运行脚本" --params '{"scriptId":1}'

# 设置代理
./jpy-cloud rpa step add <rpa_id> --type set_proxy --name "设置代理" --params '{"host":"127.0.0.1","port":1080}'
```

#### 4.4 执行 RPA

```bash
# 单次执行
./jpy-cloud rpa run <rpa_id> --device <device_id>

# 循环执行
./jpy-cloud rpa run <rpa_id> --device <device_id> --mode loop

# 查看状态
./jpy-cloud rpa status --device <device_id>

# 停止执行
./jpy-cloud rpa stop --device <device_id>
```

#### 4.5 查看执行历史

```bash
./jpy-cloud rpa history --limit 20
./jpy-cloud rpa history --device <device_id>
```

---

## 步骤类型参考

| 类型 | 说明 | 参数 |
|------|------|------|
| `shell` | 执行 Shell 命令 | `{"command": "命令"}` |
| `start_bot` | 启动脚本 | `{"scriptId": 脚本ID}` |
| `change_os` | 改机重启 | `{}` |
| `download_url` | 下载安装应用 | `{"url": "下载地址", "package": "包名"}` |
| `set_proxy` | 设置代理 | `{"host": "IP", "port": 端口}` |
| `set_location` | 设置定位 | `{"lat": 纬度, "lng": 经度}` |
| `get_root` | 获取 Root | `{}` |
| `http_request` | HTTP 请求 | `{"url": "地址", "method": "GET/POST"}` |

---

## 完整操作示例

### 示例：创建并执行一个自动化流程

```bash
# 1. 确保服务运行
curl -s http://127.0.0.1:1001/health

# 2. 获取设备列表，选择目标设备
./jpy-cloud devices -s https://114.67.244.162 -k 100064d75558efc80ba36c644b6aca1be76501773238050662

# 3. 查看现有 RPA
./jpy-cloud rpa list

# 4. 创建新 RPA
./jpy-cloud rpa create --name "测试流程"
# 假设返回 ID: 30

# 5. 添加步骤
./jpy-cloud rpa step add 30 --type shell --name "查看存储" --params '{"command":"df -h"}'
./jpy-cloud rpa step add 30 --type change_os --name "改机重启"

# 6. 查看流程详情
./jpy-cloud rpa show 30

# 7. 在设备上执行（假设设备ID为 12345678）
./jpy-cloud rpa run 30 --device 12345678

# 8. 查看执行状态
./jpy-cloud rpa status --device 12345678

# 9. 查看执行历史
./jpy-cloud rpa history --device 12345678
```

---

## 常见问题

### Q: RPA 命令报错"请求失败"

服务未运行，先启动：
```bash
./jpy-cloud serve
```

### Q: 设备命令报错"登录失败"

检查：
1. 服务器地址是否正确（需要 https://）
2. API 密钥是否有效
3. 网络是否可达

### Q: 如何查看详细日志？

```bash
# 查看服务日志
cat ~/.jpy-cloud/stderr.log

# CLI 命令加 --verbose
./jpy-cloud devices -s ... -k ... --verbose
```

---

## API 端点参考

服务运行后可用的 HTTP API：

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /health | 健康检查 |
| GET | /api/rpa/flows | 获取 RPA 列表 |
| GET | /api/rpa/flows/:id | 获取 RPA 详情 |
| POST | /api/rpa/flows | 创建 RPA |
| PUT | /api/rpa/flows/:id | 更新 RPA |
| DELETE | /api/rpa/flows/:id | 删除 RPA |
| GET | /api/rpa/devices/:id | 获取设备状态 |
| POST | /api/rpa/devices/:id/bind | 绑定 RPA |
| POST | /api/rpa/devices/:id/start | 启动执行 |
| POST | /api/rpa/devices/:id/stop | 停止执行 |
| GET | /api/rpa/history | 执行历史 |

Web 界面：http://localhost:1001
