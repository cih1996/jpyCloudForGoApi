# JPY Cloud CLI 操作手册（面向 AI）

> 通过 CLI 操作集控平台：管理设备、编排 RPA 流程、执行脚本、查看状态。
> 所有命令建议加 `--json` 输出，便于解析。

## 命令分类

CLI 命令分两类，注意区分：

| 类别 | 需要参数 | 说明 |
|------|----------|------|
| 云端命令 | `-s`（服务器）`-k`（密钥） | 直接调用集控平台 API：`devices`、`shell`、`screenshot` |
| 本地命令 | 无（走 localhost:1001） | 需要本地服务运行（`jpy-cloud serve`）：`rpa`、`script`、`debug`、`file` |

云端命令连接参数：

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

> **注意：类型名必须与下表完全一致，拼写错误会报"未知步骤类型"。**

| 类型 | 说明 | 参数 |
|------|------|------|
| `change_os_and_wait` | 改机重启（含等待上线） | `{}` 或 `{"country":"us","language":"en","timezone":"America/New_York"}` |
| `set_proxy_and_wait` | 设置代理（含等待生效） | `{"s5Url":"socks5://1.2.3.4:1080","nOutSwID":1}` s5Url 为空则取消代理 |
| `set_location` | 设置定位 | `{"lat":39.9,"lng":116.4}` 可加 `"randomRange":5`（随机公里数） |
| `network_check` | 网络检测（ping） | `{}` 或 `{"targetUrl":"8.8.8.8","maxRetries":20,"retryInterval":3}` |
| `install_app_and_wait` | 下载并安装应用 | `{"url":"http://...","name":"app.apk","install":true}` 可加 `"downloadTimeout":300,"installTimeout":120` |
| `download_url` | 仅下载文件（不安装） | `{"url":"http://...","name":"file_name"}` 可加 `"sha256":"hash"` |
| `download_cloud` | 从云端文件管理器下载 | `{"files":[{"fileName":"a.apk","fileId":1,"url":"..."}],"targetDir":"/sdcard/Download"}` |
| `shell` | 执行 Shell 命令 | `{"command":"ls -la /sdcard/"}` |
| `start_bot` | 启动脚本APK（连接1002） | `{"serverUrl":"ws://服务器IP:1002"}` 可加 `"packageName":"com.jpy.bot","maxRetries":60` |
| `execute_repo_script` | 执行脚本仓库中的脚本 | `{"scriptId":1}` 可加 `"timeout":60000` |
| `run_script` | 执行内联脚本代码 | `{"code":"await log('hello'); return {ok:true}"}` 可加 `"timeout":60000` |
| `get_root` | 应用提权 | `{"packageName":"com.android.shell"}` |
| `http_request` | HTTP 请求 | `{"url":"http://...","method":"GET","outputVar":"result"}` 可加 `"headers":{},"body":"","timeout":30` |
| `condition_check` | 条件判断 | `{"variable":"varName","operator":"eq","value":"ok","onTrue":"continue","onFalse":"stop_error"}` |
| `set_variables` | 设置流程变量 | `{"variables":{"key1":"value1","key2":123}}` ⚠️ 必须包在 `variables` 字段内 |

**condition_check 操作符：** `eq` `ne` `gt` `lt` `gte` `lte` `contains` `not_contains` `empty` `not_empty` `true` `false`
**condition_check 动作：** `continue`（继续） `skip_next`（跳过下一步） `jump_to_step`（跳转，需加 `jumpStepTrue`/`jumpStepFalse`） `stop_success` `stop_error`

### 变量传递

步骤之间通过变量传递数据：
- `http_request` 的 `outputVar` 指定变量名，响应存入该变量（同时生成 `变量名_status` 和 `变量名_raw`）
- 后续步骤用 `{{变量名}}` 或 `{{变量名.字段}}` 引用（整个字符串是变量引用时保持原类型）
- `set_variables` 设置的变量也可被后续步骤引用
- 内置变量：`{{deviceId}}` 当前设备ID

示例链路：
```
步骤1: http_request → outputVar="proxy" → 调用代理接口
步骤2: set_proxy_and_wait → s5Url="{{proxy.s5Url}}" → 设置代理
步骤3: set_location → lat={{proxy.lat}}, lng={{proxy.lng}}
步骤4: change_os_and_wait → 改机重启并等待上线
步骤5: network_check → 检测网络连通
步骤6: start_bot → serverUrl="ws://服务器:1002" → 启动脚本APK
步骤7: execute_repo_script → scriptId=1 → 执行仓库脚本
```

### 执行与监控

> **限制：同一设备同时只能运行一个 RPA 流程。** 对同一设备发起新的 rpa run 会自动取消（cancelled）正在运行的流程。

```bash
jpy-cloud rpa run <rpa_id> --device <设备ID> [--mode single|loop]  # 执行（single=单次, loop=循环）
jpy-cloud rpa stop --device <设备ID>                                # 停止
jpy-cloud rpa status --device <设备ID> --json                       # 实时状态
jpy-cloud rpa history [--device <设备ID>] [--limit <n>]             # 执行历史
```

---

## 三、文件管理

> 管理集控平台文件管理器中的文件，安装 APK 到设备

```bash
jpy-cloud file list [--json]                        # 列出文件管理器中的文件（含下载URL）
jpy-cloud file install <设备ID> <fileId> [--json]    # 安装APK到设备（自动轮询进度，120s超时）
jpy-cloud file status <设备ID> <taskId> [--json]     # 手动查询安装任务进度
```

> **注意：file install / download_url 等下载类操作底层都是设备端下载。** 设备必须能访问目标 URL（S3 地址），如果设备无外网需要先通过 `set_proxy_and_wait` 配置代理。

file status 状态码：

| status | 含义 |
|--------|------|
| 0 | 等待中 |
| 1 | 下载中（process 字段为进度百分比） |
| 2 | ��载完成 |
| 3 | 安装中 |
| 4 | 安装完成/失败（需结合 process 判断） |

示例：
```bash
# 查看已上传的文件
jpy-cloud file list --json
# → 返回: [{fileId: 10142, fileName: "pdd.apk", fileSize: 41943040, downloadUrl: "https://..."}, ...]

# 安装 APK 到设备
jpy-cloud file install 112230 10142 --json
```

---

## 四、脚本仓库

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

## 五、临时执行脚本（DebugExec）

> 不走 RPA，直接在设备上执行代码片段并拿结果。适合快速测试、调试脚本。
> 走本地服务（localhost:1001），无需 -s -k。

**注意：设备ID 使用 CRC32 ID（非云平台 deviceId）。** 传错 ID 时命令会列出当前在线设备的 CRC32 ID 供选择。

```bash
# 执行代码片段，等待返回结果
jpy-cloud debug <CRC32设备ID> --code "代码" [--timeout 30000] [--json]

# 从文件读取代码执行（推荐，避免引号冲突）
jpy-cloud debug <CRC32设备ID> --code-file ./test.js [--timeout 30000] [--json]
```

**推荐使用 `--code-file`**：复杂脚本含引号、换行时，`--code` 容易引号冲突，建议将代码写入 .js 文件后用 `--code-file` 执行。

示例：
```bash
# 查看在线设备的 CRC32 ID（传一个不存在的ID即可触发列表）
jpy-cloud debug 0 --code "return 1"
# 输出：当前在线设备:
#   CRC32 ID     序列号              型号
#   3858498935   06161JEC205494      ...

# 快速测试：获取设备信息
jpy-cloud debug 3858498935 --code "let info = await android.app.getDeviceInfo(); return info" --json

# 推荐方式：将代码写入文件
echo 'let r = await android.ocr.ocr(); return r' > /tmp/test_ocr.js
jpy-cloud debug 3858498935 --code-file /tmp/test_ocr.js --json

# 从文件执行完整脚本
jpy-cloud debug 3858498935 --code-file ./my_script.js --timeout 60000 --json
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

## 六、完整工作流示例

### 示例：改机 + 代理 + 网络检测 + 启动脚本

```bash
# 1. 查看可用设备
jpy-cloud devices -s https://114.67.244.162 -k YOUR_KEY --json

# 2. 创建 RPA 流程
jpy-cloud rpa create --name "海外养号流程"
# → 返回 ID: 29

# 3. 编排步骤（注意类型名必须完全匹配）
jpy-cloud rpa step add 29 --type http_request --name "获取代理" \
  --params '{"url":"https://api.proxy.com/get","method":"GET","outputVar":"proxy"}'

jpy-cloud rpa step add 29 --type set_proxy_and_wait --name "设置代理" \
  --params '{"s5Url":"{{proxy.s5Url}}","nOutSwID":1}'

jpy-cloud rpa step add 29 --type set_location --name "设置定位" \
  --params '{"lat":37.7749,"lng":-122.4194,"randomRange":3}'

jpy-cloud rpa step add 29 --type change_os_and_wait --name "改机重启" \
  --params '{"country":"us"}'

jpy-cloud rpa step add 29 --type network_check --name "网络检测" \
  --params '{}'

jpy-cloud rpa step add 29 --type start_bot --name "启动脚本APK" \
  --params '{"serverUrl":"ws://你的服务器IP:1002"}'

jpy-cloud rpa step add 29 --type execute_repo_script --name "执行仓库脚本" \
  --params '{"scriptId":1,"timeout":60000}'

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
# 1. 先查在线设备的 CRC32 ID
jpy-cloud debug 0 --code "return 1"
# → 得到 CRC32 ID: 3858498935

# 2. 用 debug 临时跑一段代码，验证逻辑（推荐 --code-file）
cat > /tmp/test_wechat.js << 'EOF'
await android.app.runApp('com.tencent.mm')
await sleep(3000)
let node = await android.acc.findViewEx({text: '发现'}, 5000)
return {found: !!node}
EOF
jpy-cloud debug 3858498935 --code-file /tmp/test_wechat.js --json

# 3. 验证通过后，存入脚本仓库（也推荐 --code-file）
cat > /tmp/open_wechat.js << 'EOF'
await log('启动微信')
await android.app.runApp('com.tencent.mm')
await sleep(3000)
let node = await android.acc.findViewEx({text: '发现'}, 5000)
if (!node) throw new Error('微信启动失败')
await log('微信已打开')
return {success: true}
EOF
jpy-cloud script create --name "打开微信" --code-file /tmp/open_wechat.js --desc "启动微信并验证"
# → 返回 scriptId: 5

# 4. 创建 RPA 流程，引用该脚本
jpy-cloud rpa create --name "微信养号"
# → 返回 ID: 30
jpy-cloud rpa step add 30 --type change_os_and_wait --name "改机"
jpy-cloud rpa step add 30 --type network_check --name "网络检测"
jpy-cloud rpa step add 30 --type start_bot --name "启动脚本APK" \
  --params '{"serverUrl":"ws://你的服务器IP:1002"}'
jpy-cloud rpa step add 30 --type execute_repo_script --name "打开微信" \
  --params '{"scriptId": 5, "timeout": 30000}'

# 5. 执行 RPA 并监控
jpy-cloud rpa run 30 --device 112230
jpy-cloud rpa status --device 112230 --json

# 6. 查看执行历史和日志
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
