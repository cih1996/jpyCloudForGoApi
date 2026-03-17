# AccBot WebSocket 通信协议规范 v1.0

## 概述

本协议定义了 AccBot 脚本执行器与服务端之间的 WebSocket 二进制通信规范。

**设计原则：**
- 每个数据包都携带设备 ID（支持 TCP 连接复用/网关代理场景）
- 二进制协议，高效紧凑
- 支持心跳保活、断线重连
- 客户端连接后立即发送初始化包（避免被服务端当作垃圾连接）

---

## 协议结构

### 数据包格式

```
┌──────────────────────────────────────────────────────────────────┐
│                        协议头 (16 字节)                           │
├─────────┬─────────┬──────────────┬──────────────┬────────────────┤
│ 字节0   │ 字节1   │  字节2-5     │  字节6-9     │   字节10-13    │
│ 消息类型 │ 数据格式 │  设备ID      │  消息序号     │   数据长度     │
│ (1byte) │ (1byte) │  (4bytes)    │  (4bytes)    │   (4bytes)     │
├─────────┴─────────┴──────────────┴──────────────┴────────────────┤
│ 字节14-15: 保留字段 (2bytes，用于未来扩展，当前填 0x0000)           │
├──────────────────────────────────────────────────────────────────┤
│                        业务数据 (变长)                            │
│                    长度由协议头中的"数据长度"指定                   │
└──────────────────────────────────────────────────────────────────┘
```

### 字段说明

| 字段 | 偏移 | 长度 | 类型 | 说明 |
|------|------|------|------|------|
| msgType | 0 | 1 | uint8 | 消息类型 |
| dataFormat | 1 | 1 | uint8 | 数据格式 |
| deviceId | 2 | 4 | uint32 | 设备唯一ID（大端序） |
| seqNo | 6 | 4 | uint32 | 消息序号（大端序，递增） |
| dataLen | 10 | 4 | uint32 | 业务数据长度（大端序） |
| reserved | 14 | 2 | uint16 | 保留字段，填 0 |
| payload | 16 | N | bytes | 业务数据 |

**注意：所有多字节整数使用大端序（Big-Endian）**

---

## 消息类型 (msgType)

### 客户端 → 服务端

| 值 | 名称 | 说明 |
|----|------|------|
| 0x01 | HEARTBEAT | 心跳包 |
| 0x02 | INIT | 初始化/注册（连接后第一个包） |
| 0x10 | STATUS_REPORT | 状态上报 |
| 0x11 | LOG_REPORT | 日志上报 |
| 0x12 | TASK_RESULT | 任务执行结果上报 |
| 0x13 | PROGRESS_REPORT | 进度上报 |
| 0x14 | SCRIPT_PULL | 请求拉取脚本（缓存未命中时） |
| 0x15 | RESOURCE_PULL | 请求拉取资源（图片等） |
| 0x16 | SCREENSHOT_DATA | 截图数据上传 |
| 0x17 | DEBUG_RESULT | 调试执行结果上报 |
| 0x18 | NODES_DATA | 节点信息上报 |

### 服务端 → 客户端

| 值 | 名称 | 说明 |
|----|------|------|
| 0x01 | HEARTBEAT_ACK | 心跳响应 |
| 0x02 | INIT_ACK | 初始化响应 |
| 0x20 | TASK_PUSH | 下发脚本任务（支持缓存模式） |
| 0x21 | TASK_CANCEL | 取消任务 |
| 0x22 | CONFIG_UPDATE | 配置更新 |
| 0x23 | COMMAND | 执行命令（如重启、清理、截图、获取节点等） |
| 0x24 | SCRIPT_DATA | 脚本内容响应 |
| 0x25 | RESOURCE_DATA | 资源内容响应（图片等） |
| 0x26 | RESOURCE_PUSH | 主动推送资源（图片等） |
| 0x27 | DEBUG_EXEC | 调试模式直接执行（不缓存） |

---

## 数据格式 (dataFormat)

| 值 | 名称 | 说明 |
|----|------|------|
| 0x00 | NONE | 无数据（payload 为空） |
| 0x01 | JSON | JSON 格式 |
| 0x02 | TEXT | 纯文本（UTF-8） |
| 0x03 | BINARY | 二进制数据 |

---

## 设备 ID 生成规则

设备 ID 由设备序列号（serialno）生成，确保唯一且固定：

```
1. 读取设备序列号: cat /metadata/property/serialno
2. 计算 hash: CRC32(serialno) 或 FNV-1a(serialno)
3. 取低 32 位作为 deviceId
```

**示例：**
```
serialno = "ABC123XYZ"
deviceId = CRC32("ABC123XYZ") = 0x1A2B3C4D
```

---

## 消息详细定义

### 1. 心跳 (HEARTBEAT / HEARTBEAT_ACK)

**客户端发送：**
```
msgType: 0x01
dataFormat: 0x00
payload: 空
```

**服务端响应：**
```
msgType: 0x01
dataFormat: 0x00
payload: 空
```

**心跳策略：**
- 客户端每 30 秒发送一次心跳
- 服务端收到后立即响应
- 客户端 90 秒未收到响应则判定断线，触发重连

---

### 2. 初始化 (INIT / INIT_ACK)

**客户端发送（连接成功后立即发送）：**
```
msgType: 0x02
dataFormat: 0x01 (JSON)
payload: {
    "serialno": "ABC123XYZ",        // 原始序列号
    "version": "1.0.0",             // 执行器版本
    "sdkVersion": "1.0.36",         // AccBot SDK 版本
    "screenWidth": 1080,            // 屏幕宽度
    "screenHeight": 2400,           // 屏幕高度
    "brand": "Xiaomi",              // 设备品牌
    "model": "Redmi K50",           // 设备型号
    "androidVersion": "13",         // Android 版本
    "state": "idle"                 // 当前状态
}
```

**服务端响应：**
```
msgType: 0x02
dataFormat: 0x01 (JSON)
payload: {
    "success": true,
    "serverTime": 1710582400000,    // 服务器时间戳
    "config": {                     // 可选配置下发
        "heartbeatInterval": 30000,
        "taskTimeout": 300000
    }
}
```

---

### 3. 状态上报 (STATUS_REPORT)

**客户端发送：**
```
msgType: 0x10
dataFormat: 0x01 (JSON)
payload: {
    "state": "running",             // idle | running | error | done
    "currentTask": "001_登录.js",
    "progress": "正在输入密码",
    "timestamp": 1710582400000
}
```

---

### 4. 日志上报 (LOG_REPORT)

**客户端发送：**
```
msgType: 0x11
dataFormat: 0x01 (JSON)
payload: {
    "level": "info",                // debug | info | warn | error
    "tag": "001_登录.js",
    "message": "点击登录按钮",
    "timestamp": 1710582400000
}
```

---

### 5. 任务结果上报 (TASK_RESULT)

**客户端发送：**
```
msgType: 0x12
dataFormat: 0x01 (JSON)
payload: {
    "taskId": "task_001",           // 服务端下发的任务ID
    "taskName": "001_登录.js",
    "success": true,
    "result": { "userId": 123 },    // 脚本 return 的数据
    "error": null,                  // 失败时的错误信息
    "duration": 5000,               // 执行耗时(ms)
    "timestamp": 1710582400000
}
```

---

### 6. 进度上报 (PROGRESS_REPORT)

**客户端发送：**
```
msgType: 0x13
dataFormat: 0x01 (JSON)
payload: {
    "taskId": "task_001",
    "progress": "正在输入密码",
    "percent": 50,                  // 可选，进度百分比
    "timestamp": 1710582400000
}
```

---

### 7. 任务下发 (TASK_PUSH)

支持两种模式：直接推送代码 或 仅推送 hash（客户端从缓存加载）

**服务端发送：**
```
msgType: 0x20
dataFormat: 0x01 (JSON)
payload: {
    "taskId": "task_001",           // 任务唯一ID
    "taskName": "登录任务",          // 任务名称（用于日志显示）
    "scriptHash": "a1b2c3d4",       // 脚本内容的 hash（用于缓存校验）
    "code": "await log('开始')...", // 脚本代码（可选，为 null 时客户端从缓存加载）
    "timeout": 120000,              // 超时时间(ms)
    "priority": 0,                  // 优先级（0=普通，数字越大越优先）
    "resources": [                  // 可选，依赖的资源列表
        { "name": "button.png", "hash": "e5f6g7h8" }
    ]
}
```

**客户端处理逻辑：**
```
收到 TASK_PUSH
    │
    ├── code 不为空 → 直接执行，同时缓存脚本（以 scriptHash 为 key）
    │
    └── code 为空 → 检查本地缓存
        ├── 缓存命中（hash 匹配）→ 从缓存加载执行
        └── 缓存未命中 → 发送 SCRIPT_PULL 请求拉取
```

---

### 8. 脚本拉取 (SCRIPT_PULL / SCRIPT_DATA)

**客户端请求（缓存未命中时）：**
```
msgType: 0x14
dataFormat: 0x01 (JSON)
payload: {
    "taskId": "task_001",           // 关联的任务ID
    "scriptHash": "a1b2c3d4"        // 需要拉取的脚本 hash
}
```

**服务端响应：**
```
msgType: 0x24
dataFormat: 0x01 (JSON)
payload: {
    "taskId": "task_001",
    "scriptHash": "a1b2c3d4",
    "code": "await log('开始')..."  // 完整脚本代码
}
```

---

### 9. 资源拉取 (RESOURCE_PULL / RESOURCE_DATA)

用于拉取找图所需的图片等资源。

**客户端请求：**
```
msgType: 0x15
dataFormat: 0x01 (JSON)
payload: {
    "name": "button.png",           // 资源名称
    "hash": "e5f6g7h8"              // 资源 hash
}
```

**服务端响应：**
```
msgType: 0x25
dataFormat: 0x03 (BINARY)
payload: {
    前 4 字节: hash 长度 (uint32, 大端序)
    接下来 N 字节: hash 字符串 (UTF-8)
    前 4 字节: name 长度 (uint32, 大端序)
    接下来 M 字节: name 字符串 (UTF-8)
    剩余字节: 资源二进制数据
}
```

或使用 JSON + Base64（简单但效率略低）：
```
msgType: 0x25
dataFormat: 0x01 (JSON)
payload: {
    "name": "button.png",
    "hash": "e5f6g7h8",
    "data": "base64编码的图片数据..."
}
```

---

### 10. 资源推送 (RESOURCE_PUSH)

服务端主动推送资源（预加载或更新）。

**服务端发送：**
```
msgType: 0x26
dataFormat: 0x01 (JSON)
payload: {
    "name": "button.png",
    "hash": "e5f6g7h8",
    "data": "base64编码的图片数据..."
}
```

---

### 11. 调试执行 (DEBUG_EXEC)

开发调试用，直接执行代码片段，不走缓存，不入队列。

**服务端发送：**
```
msgType: 0x27
dataFormat: 0x01 (JSON)
payload: {
    "debugId": "debug_001",         // 调试会话ID
    "code": "await log('测试')...", // 要执行的代码
    "timeout": 30000                // 超时时间
}
```

**客户端处理：**
- 立即执行（如果当前有任务在执行，可选择排队或中断）
- 执行结果通过 TASK_RESULT 上报（taskId 使用 debugId）
- 不缓存脚本

---

### 12. 截图请求 (COMMAND: screenshot)

**服务端发送：**
```
msgType: 0x23
dataFormat: 0x01 (JSON)
payload: {
    "cmd": "screenshot",
    "params": {
        "quality": 80,              // 图片质量 1-100
        "scale": 0.5                // 缩放比例（可选，默认 1.0）
    }
}
```

**客户端响应（截图数据上传）：**
```
msgType: 0x16
dataFormat: 0x01 (JSON)
payload: {
    "width": 1080,
    "height": 2400,
    "data": "base64编码的图片..."
}
```

---

### 13. 节点信息请求 (COMMAND: getNodes)

**服务端发送：**
```
msgType: 0x23
dataFormat: 0x01 (JSON)
payload: {
    "cmd": "getNodes",
    "params": {
        "requestId": "xxx",      // 可选，用于关联请求和响应
        "windowId": -1,          // 可选，-1 表示当前 app 窗口
        "visibleOnly": true      // 可选，是否只获取可见节点
    }
}
```

**客户端响应（节点信息上报）：**
```
msgType: 0x18
dataFormat: 0x01 (JSON)
payload: {
    "requestId": "xxx",
    "nodes": {
        "boundsInScreen": [0, 0, 1080, 2400],
        "childCount": 3,
        "children": [...],
        "className": "android.widget.FrameLayout",
        "contentDescription": "",
        "depth": 0,
        "isCheckable": false,
        "isChecked": false,
        "isClickable": false,
        "isEditable": false,
        "isEnabled": true,
        "isFocusable": false,
        "isFocused": false,
        "isLongClickable": false,
        "isPassword": false,
        "isScrollable": false,
        "isVisibleToUser": true,
        "resourceId": "",
        "text": ""
    },
    "timestamp": 1234567890
}
```

---

### 14. 调试执行结果 (DEBUG_RESULT)

**客户端发送：**
```
msgType: 0x17
dataFormat: 0x01 (JSON)
payload: {
    "debugId": "debug_001",     // 对应 DEBUG_EXEC 的 debugId
    "success": true,
    "result": { ... },          // 执行结果
    "error": null,              // 失败时的错误信息
    "logs": ["log1", "log2"],   // 执行过程中的日志
    "duration": 1500,           // 执行耗时(ms)
    "timestamp": 1710582400000
}
```

---

### 15. 取消任务 (TASK_CANCEL)

**服务端发送：**
```
msgType: 0x21
dataFormat: 0x01 (JSON)
payload: {
    "taskId": "task_001",           // 要取消的任务ID
    "reason": "用户手动取消"
}
```

---

### 14. 配置更新 (CONFIG_UPDATE)

**服务端发送：**
```
msgType: 0x22
dataFormat: 0x01 (JSON)
payload: {
    "heartbeatInterval": 30000,
    "taskTimeout": 300000,
    "logLevel": "info"
}
```

---

### 15. 执行命令 (COMMAND)

**服务端发送：**
```
msgType: 0x23
dataFormat: 0x01 (JSON)
payload: {
    "cmd": "restart",               // restart | clearTasks | clearCache | screenshot | getNodes
    "params": {}
}
```

---

## 连接生命周期

```
┌─────────────────────────────────────────────────────────────────┐
│                        客户端                                    │
└─────────────────────────────────────────────────────────────────┘
        │
        │ 1. WebSocket 连接
        ▼
┌───────────────────┐
│   连接成功         │
└────────┬──────────┘
         │
         │ 2. 立即发送 INIT 包（3秒内必须发送，否则服务端断开）
         ▼
┌───────────────────┐         ┌───────────────────┐
│   等待 INIT_ACK   │────────▶│   初始化完成       │
└───────────────────┘         └────────┬──────────┘
                                       │
         ┌─────────────────────────────┴─────────────────────────┐
         │                                                       │
         ▼                                                       ▼
┌───────────────────┐                               ┌───────────────────┐
│   定时心跳         │                               │   接收/处理任务    │
│   (每30秒)        │                               │                   │
└───────────────────┘                               └───────────────────┘
         │
         │ 90秒无响应
         ▼
┌───────────────────┐
│   判定断线         │
│   触发重连         │
└───────────────────┘
```

---

## 重连策略

```
重连间隔 = min(30秒, 2^重试次数 秒)

第1次重连: 等待 2 秒
第2次重连: 等待 4 秒
第3次重连: 等待 8 秒
第4次重连: 等待 16 秒
第5次及以后: 等待 30 秒
```

---

## Go 服务端实现参考

### 协议解析

```go
package protocol

import (
    "encoding/binary"
    "errors"
)

const HeaderSize = 16

type MsgType uint8

const (
    MsgHeartbeat      MsgType = 0x01
    MsgInit           MsgType = 0x02
    MsgStatusReport   MsgType = 0x10
    MsgLogReport      MsgType = 0x11
    MsgTaskResult     MsgType = 0x12
    MsgProgressReport MsgType = 0x13
    MsgTaskPush       MsgType = 0x20
    MsgTaskCancel     MsgType = 0x21
    MsgConfigUpdate   MsgType = 0x22
    MsgCommand        MsgType = 0x23
)

type DataFormat uint8

const (
    FormatNone   DataFormat = 0x00
    FormatJSON   DataFormat = 0x01
    FormatText   DataFormat = 0x02
    FormatBinary DataFormat = 0x03
)

type Header struct {
    MsgType    MsgType
    DataFormat DataFormat
    DeviceID   uint32
    SeqNo      uint32
    DataLen    uint32
    Reserved   uint16
}

type Packet struct {
    Header  Header
    Payload []byte
}

// ParseHeader 解析协议头
func ParseHeader(data []byte) (*Header, error) {
    if len(data) < HeaderSize {
        return nil, errors.New("data too short")
    }
    return &Header{
        MsgType:    MsgType(data[0]),
        DataFormat: DataFormat(data[1]),
        DeviceID:   binary.BigEndian.Uint32(data[2:6]),
        SeqNo:      binary.BigEndian.Uint32(data[6:10]),
        DataLen:    binary.BigEndian.Uint32(data[10:14]),
        Reserved:   binary.BigEndian.Uint16(data[14:16]),
    }, nil
}

// ParsePacket 解析完整数据包
func ParsePacket(data []byte) (*Packet, error) {
    header, err := ParseHeader(data)
    if err != nil {
        return nil, err
    }
    if len(data) < HeaderSize+int(header.DataLen) {
        return nil, errors.New("incomplete packet")
    }
    return &Packet{
        Header:  *header,
        Payload: data[HeaderSize : HeaderSize+header.DataLen],
    }, nil
}

// BuildPacket 构建数据包
func BuildPacket(msgType MsgType, dataFormat DataFormat, deviceID, seqNo uint32, payload []byte) []byte {
    dataLen := uint32(len(payload))
    buf := make([]byte, HeaderSize+dataLen)

    buf[0] = byte(msgType)
    buf[1] = byte(dataFormat)
    binary.BigEndian.PutUint32(buf[2:6], deviceID)
    binary.BigEndian.PutUint32(buf[6:10], seqNo)
    binary.BigEndian.PutUint32(buf[10:14], dataLen)
    binary.BigEndian.PutUint16(buf[14:16], 0) // reserved

    if dataLen > 0 {
        copy(buf[HeaderSize:], payload)
    }
    return buf
}
```

### WebSocket 处理示例

```go
package main

import (
    "encoding/json"
    "log"
    "net/http"
    "sync"
    "time"

    "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool { return true },
}

// DeviceConn 设备连接
type DeviceConn struct {
    DeviceID uint32
    Conn     *websocket.Conn
    LastSeen time.Time
}

// DeviceManager 设备管理器
type DeviceManager struct {
    devices map[uint32]*DeviceConn
    mu      sync.RWMutex
}

func (dm *DeviceManager) HandleConnection(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Println("upgrade error:", err)
        return
    }
    defer conn.Close()

    // 等待 INIT 包（3秒超时）
    conn.SetReadDeadline(time.Now().Add(3 * time.Second))
    _, data, err := conn.ReadMessage()
    if err != nil {
        log.Println("read init error:", err)
        return
    }

    packet, err := ParsePacket(data)
    if err != nil || packet.Header.MsgType != MsgInit {
        log.Println("invalid init packet")
        return
    }

    deviceID := packet.Header.DeviceID
    log.Printf("device %08X connected", deviceID)

    // 注册设备
    dm.mu.Lock()
    dm.devices[deviceID] = &DeviceConn{
        DeviceID: deviceID,
        Conn:     conn,
        LastSeen: time.Now(),
    }
    dm.mu.Unlock()

    defer func() {
        dm.mu.Lock()
        delete(dm.devices, deviceID)
        dm.mu.Unlock()
        log.Printf("device %08X disconnected", deviceID)
    }()

    // 发送 INIT_ACK
    ack := map[string]interface{}{
        "success":    true,
        "serverTime": time.Now().UnixMilli(),
    }
    ackData, _ := json.Marshal(ack)
    ackPacket := BuildPacket(MsgInit, FormatJSON, deviceID, 0, ackData)
    conn.WriteMessage(websocket.BinaryMessage, ackPacket)

    // 主循环
    conn.SetReadDeadline(time.Time{}) // 取消超时
    for {
        _, data, err := conn.ReadMessage()
        if err != nil {
            break
        }

        packet, err := ParsePacket(data)
        if err != nil {
            continue
        }

        // 更新最后活跃时间
        dm.mu.Lock()
        if dc, ok := dm.devices[packet.Header.DeviceID]; ok {
            dc.LastSeen = time.Now()
        }
        dm.mu.Unlock()

        // 处理消息
        switch packet.Header.MsgType {
        case MsgHeartbeat:
            // 响应心跳
            ackPacket := BuildPacket(MsgHeartbeat, FormatNone, packet.Header.DeviceID, packet.Header.SeqNo, nil)
            conn.WriteMessage(websocket.BinaryMessage, ackPacket)

        case MsgStatusReport:
            log.Printf("device %08X status: %s", packet.Header.DeviceID, string(packet.Payload))

        case MsgLogReport:
            log.Printf("device %08X log: %s", packet.Header.DeviceID, string(packet.Payload))

        case MsgTaskResult:
            log.Printf("device %08X result: %s", packet.Header.DeviceID, string(packet.Payload))
        }
    }
}

// SendTask 向设备下发任务
func (dm *DeviceManager) SendTask(deviceID uint32, taskID, taskName, code string, timeout int) error {
    dm.mu.RLock()
    dc, ok := dm.devices[deviceID]
    dm.mu.RUnlock()

    if !ok {
        return errors.New("device not found")
    }

    task := map[string]interface{}{
        "taskId":   taskID,
        "taskName": taskName,
        "code":     code,
        "timeout":  timeout,
    }
    data, _ := json.Marshal(task)
    packet := BuildPacket(MsgTaskPush, FormatJSON, deviceID, 0, data)
    return dc.Conn.WriteMessage(websocket.BinaryMessage, packet)
}
```

---

## 版本历史

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-03-16 | 初始版本 |
| 1.1 | 2026-03-16 | 新增脚本缓存、资源管理、调试模式 |

---

## 附录：客户端缓存机制

### 缓存目录结构

```
/sdcard/accbot/
├── cache/
│   ├── scripts/           # 脚本缓存
│   │   ├── a1b2c3d4.js    # 以 hash 命名
│   │   └── e5f6g7h8.js
│   └── resources/         # 资源缓存（图片等）
│       ├── button.png     # 以原始名称存储
│       └── icon.png
└── cache_index.json       # 缓存索引
```

### 缓存索引文件 (cache_index.json)

```json
{
  "scripts": {
    "a1b2c3d4": {
      "size": 10240,
      "cachedAt": 1710582400000,
      "lastUsed": 1710582500000,
      "hitCount": 5
    }
  },
  "resources": {
    "button.png": {
      "hash": "e5f6g7h8",
      "size": 2048,
      "cachedAt": 1710582400000
    }
  }
}
```

### 缓存策略

1. **脚本缓存**
   - 以 scriptHash 为 key 存储
   - 收到带 code 的 TASK_PUSH 时自动缓存
   - 缓存命中时直接使用，无需网络请求
   - 可设置最大缓存数量/大小，LRU 淘汰

2. **资源缓存**
   - 以资源名称存储，hash 用于校验
   - 任务依赖的资源在执行前检查
   - 未命中时通过 RESOURCE_PULL 拉取

3. **缓存清理命令**
   ```json
   {
     "cmd": "clearCache",
     "params": {
       "type": "all"    // all | scripts | resources
     }
   }
   ```

### 任务执行流程（带缓存）

```
收到 TASK_PUSH
    │
    ▼
检查 resources 依赖
    │
    ├── 有缺失资源 → 发送 RESOURCE_PULL 拉取
    │                   │
    │                   ▼
    │               收到 RESOURCE_DATA → 保存到缓存
    │
    ▼
检查 code 字段
    │
    ├── code 不为空 → 缓存脚本 → 执行
    │
    └── code 为空 → 检查脚本缓存
        │
        ├── 命中 → 从缓存加载 → 执行
        │
        └── 未命中 → 发送 SCRIPT_PULL
                        │
                        ▼
                    收到 SCRIPT_DATA → 缓存 → 执行
```

---

## 附录：调试模式使用场景

### 1. 快速测试代码片段

开发时快速验证某段代码是否正确：

```json
// 服务端发送
{
  "msgType": "0x27",
  "payload": {
    "debugId": "test_001",
    "code": "let result = await android.ocr.ocr(); await log(JSON.stringify(result)); return result;",
    "timeout": 10000
  }
}
```

### 2. 实时截图调试

获取当前屏幕截图，用于确定坐标：

```json
// 服务端发送
{
  "msgType": "0x23",
  "payload": {
    "cmd": "screenshot",
    "params": { "quality": 80 }
  }
}

// 客户端响应
{
  "msgType": "0x16",
  "payload": {
    "width": 1080,
    "height": 2400,
    "data": "base64..."
  }
}
```

### 3. 推送测试图片

开发找图功能时，推送测试图片：

```json
// 服务端发送
{
  "msgType": "0x26",
  "payload": {
    "name": "test_button.png",
    "hash": "abc123",
    "data": "base64..."
  }
}
```

然后执行调试代码测试找图：

```json
{
  "msgType": "0x27",
  "payload": {
    "debugId": "test_find",
    "code": "let r = await android.openCvUtil.findImgByName('test_button.png'); await log(JSON.stringify(r)); return r;"
  }
}
```
