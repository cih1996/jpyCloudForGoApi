# JPY Cloud CLI 操作手册（面向 AI）

> 通过 CLI 操作集控平台：管理设备、编排 RPA 流程、执行脚本、查看状态。
> 所有命令建议加 `--json` 输出，便于解析。

## 连接参数

所有设备相关命令必须携带：

| 参数 | 简写 | 说明 |
|------|------|------|
| `--server` | `-s` | 集控平台地址（如 `https://114.67.244.162`） |
| `--key` | `-k` | API 密钥（从集控平台获取） |
| `--json` | | JSON 格式输出 |

---

## 一、设备管理

```bash
# 获取设备列表
jpy-cloud devices -s <服务器> -k <密钥> --json

# 在设备上执行 Shell 命令
jpy-cloud shell -s <服务器> -k <密钥> <设备ID> "<命令>"

# 设备截图（保存到本地文件）
jpy-cloud screenshot -s <服务器> -k <密钥> <设备ID> [输出文件]
```

示例：
```bash
jpy-cloud devices -s https://114.67.244.162 -k YOUR_KEY --json
jpy-cloud shell -s https://114.67.244.162 -k YOUR_KEY 112230 "ls -la /sdcard/"
jpy-cloud screenshot -s https://114.67.244.162 -k YOUR_KEY 112230 screen.png
```

---

## 二、RPA 流程管理

> RPA 命令需要本地服务已启动（`jpy-cloud serve`）

### 流程 CRUD

```bash
jpy-cloud rpa list [--json]                            # 列出所有流程
jpy-cloud rpa show <id> [--json]                       # 查看流程详情（含步骤）
jpy-cloud rpa create --name "流程名" [--desc "描述"]    # 创建流程，返回 ID
jpy-cloud rpa delete <id>                              # 删除流程
```

### 步骤管理

```bash
jpy-cloud rpa step list <rpa_id>                                              # 列出步骤
jpy-cloud rpa step add <rpa_id> --type <类型> --name <名称> [--params <JSON>] # 添加步骤
jpy-cloud rpa step remove <rpa_id> <index>                                    # 删除步骤（index 从 0 开始）
jpy-cloud rpa step move <rpa_id> <from> <to>                                  # 调整步骤顺序
```

### 步骤类型

| 类型 | 说明 | 参数 |
|------|------|------|
| `change_os` | 改机重启 | `{}` 或 `{"country":"US","language":"en","timezone":"America/New_York"}` |
| `set_proxy` | 设置代理 | `{"host":"1.2.3.4","port":1080}` 或从变量：`{"fromVar":"proxyData","hostField":"ip","portField":"port"}` |
| `set_location` | 设置定位 | `{"lat":39.9,"lng":116.4}` 或从变量：`{"fromVar":"loc","latField":"lat","lngField":"lng"}` 可加 `"randomKm":5` |
| `download_url` | 下载安装应用 | `{"url":"http://...","package":"com.app"}` |
| `shell` | 执行 Shell 命令 | `{"command":"ls -la /sdcard/"}` |
| `start_bot` | 启动脚本 | `{"scriptId":1}` |
| `get_root` | 获取 Root | `{}` 或 `{"pkg":"com.target.app"}` |
| `http_request` | HTTP 请求 | `{"url":"http://...","method":"GET","outputVar":"result"}` |
| `condition` | 条件判断 | `{"condition":"{{result.status}} == 'ok'"}` |
| `set_variables` | 设置变量 | `{"key":"value","name":"test"}` |

### 变量传递

步骤之间通过变量传递数据：
- 前置步骤设置 `outputVar` 指定变量名（如 `http_request` 的响应存入变量）
- 后续步骤用 `{{变量名}}` 或 `{{变量名.字段}}` 引用
- 内置变量：`{{deviceId}}` 当前设备ID

示例链路：
```
步骤1: http_request → outputVar="proxy" → 调用代理接口
步骤2: set_proxy → fromVar="proxy" → 使用 {{proxy.ip}}:{{proxy.port}}
步骤3: change_os → 改机重启
步骤4: start_bot → 启动脚本
```

### 执行与监控

```bash
jpy-cloud rpa run <rpa_id> --device <设备ID> [--mode single|loop]  # 执行（single=单次, loop=循环）
jpy-cloud rpa stop --device <设备ID>                                # 停止
jpy-cloud rpa status --device <设备ID> --json                       # 实时状态
jpy-cloud rpa history [--device <设备ID>] [--limit <n>]             # 执行历史
```

---

## 三、脚本仓库

> 管理脚本代码，存入仓库后可在 RPA 步骤中引用

```bash
jpy-cloud script list [--json]                                    # 列出所有脚本
jpy-cloud script show <id> [--json]                               # 查看脚本详情（含代码）
jpy-cloud script create --name "脚本名" --code "代码" [--desc "描述"] [--timeout 60000]  # 创建
jpy-cloud script update <id> --name "脚本名" --code "代码" [--desc "描述"] [--timeout 60000]  # 更新
jpy-cloud script delete <id>                                      # 删除
```

从文件读取代码创建脚本：
```bash
jpy-cloud script create --name "登录脚本" --code-file ./login.js --desc "自动登录流程"
```

在 RPA 中使用仓库脚本：
```bash
# start_bot 步骤引用脚本仓库 ID
jpy-cloud rpa step add <rpa_id> --type start_bot --name "运行登录脚本" --params '{"scriptId": 1}'
```

---

## 四、临时执行脚本（DebugExec）

> 不走 RPA，直接在设备上执行代码片段并拿结果。适合快速测试、调试脚本。

```bash
# 执行代码片段，等待返回结果
jpy-cloud debug <设备ID> --code "代码" [--timeout 30000] [--json]

# 从文件读取代码执行
jpy-cloud debug <设备ID> --code-file ./test.js [--timeout 30000] [--json]
```

示例：
```bash
# 快速测试：获取设备信息
jpy-cloud debug 112230 --code "let info = await android.app.getDeviceInfo(); return info" --json

# 快速测试：OCR 识别当前屏幕
jpy-cloud debug 112230 --code "let r = await android.ocr.ocr(); return r" --json

# 快速测试：点击坐标
jpy-cloud debug 112230 --code "await android.touch.click(500, 300); return {success:true}" --json

# 从文件执行完整脚本
jpy-cloud debug 112230 --code-file ./my_script.js --timeout 60000 --json
```

返回格式（--json）：
```json
{
  "success": true,
  "result": { "deviceInfo": "..." },
  "logs": ["日志1", "日志2"],
  "duration": 3500
}
```

---

## 五、完整工作流示例

### 示例：改机 + 代理 + 启动脚本

```bash
# 1. 查看可用设备
jpy-cloud devices -s https://114.67.244.162 -k YOUR_KEY --json

# 2. 创建 RPA 流程
jpy-cloud rpa create --name "海外养号流程"
# → 返回 ID: 29

# 3. 编排步骤
jpy-cloud rpa step add 29 --type http_request --name "获取代理" \
  --params '{"url":"https://api.proxy.com/get","method":"GET","outputVar":"proxy"}'

jpy-cloud rpa step add 29 --type set_proxy --name "设置代理" \
  --params '{"fromVar":"proxy","hostField":"ip","portField":"port"}'

jpy-cloud rpa step add 29 --type set_location --name "设置定位" \
  --params '{"lat":37.7749,"lng":-122.4194,"randomKm":3}'

jpy-cloud rpa step add 29 --type change_os --name "改机重启" \
  --params '{"country":"US"}'

jpy-cloud rpa step add 29 --type start_bot --name "启动脚本" \
  --params '{"scriptId":1}'

# 4. 确认流程
jpy-cloud rpa show 29 --json

# 5. 在设备上执行
jpy-cloud rpa run 29 --device 112230

# 6. 监控状态
jpy-cloud rpa status --device 112230 --json

# 7. 查看结果
jpy-cloud rpa history --device 112230 --limit 5
```

### 示例：写脚本 → 测试 → 存仓库 → 加入RPA

```bash
# 1. 先用 debug 临时跑一段代码，验证逻辑
jpy-cloud debug 112230 --code "
await android.app.runApp('com.tencent.mm')
await sleep(3000)
let node = await android.acc.findViewEx({text: '发现'}, 5000)
return {found: !!node}
" --json

# 2. 验证通过后，存入脚本仓库
jpy-cloud script create --name "打开微信" --code "
await log('启动微信')
await android.app.runApp('com.tencent.mm')
await sleep(3000)
let node = await android.acc.findViewEx({text: '发现'}, 5000)
if (!node) throw new Error('微信启动失败')
await log('微信已打开')
return {success: true}
" --desc "启动微信并验证"
# → 返回 scriptId: 5

# 3. 创建 RPA 流程，引用该脚本
jpy-cloud rpa create --name "微信养号"
# → 返回 ID: 30
jpy-cloud rpa step add 30 --type change_os --name "改机"
jpy-cloud rpa step add 30 --type start_bot --name "打开微信" --params '{"scriptId": 5}'

# 4. 执行 RPA 并监控
jpy-cloud rpa run 30 --device 112230
jpy-cloud rpa status --device 112230 --json

# 5. 查看执行历史和日志
jpy-cloud rpa history --device 112230 --limit 5
```

### 示例：批量 Shell 操作

```bash
# 在设备上查看已安装应用
jpy-cloud shell -s https://114.67.244.162 -k YOUR_KEY 112230 "pm list packages"

# 清理应用数据
jpy-cloud shell -s https://114.67.244.162 -k YOUR_KEY 112230 "pm clear com.target.app"

# 查看设备存储
jpy-cloud shell -s https://114.67.244.162 -k YOUR_KEY 112230 "df -h"

# 查看运行中的脚本日志
jpy-cloud shell -s https://114.67.244.162 -k YOUR_KEY 112230 "cat /sdcard/accbot/executor.log"
```
