# Go 端口转发服务 (Go Port Transfer Service)

本项目是一个**高性能端口转发与设备管理服务**，支持统一的 HTTP/WebSocket API 接口。

## 🚀 概览

该服务提供了一个模块化的后端，用于管理设备连接和端口映射。它采用**统一 API 架构**，所有业务端点均可通过 HTTP (POST) 和 WebSocket 访问，并确保跨协议的数据结构一致性。

## ✨ 核心特性

*   **统一 API 架构**：所有业务逻辑通过相同的 Request/Response 结构暴露给 HTTP 和 WebSocket。
*   **交互式文档**：提供自动生成的交互式 API 文档，访问 `/doc` 即可查看。
*   **双协议支持**：
    *   **HTTP API**：监听端口 `1001` (仅限 POST)。
    *   **WebSocket**：监听端口 `1002`。
*   **严格类型检查**：使用 Go 泛型严格定义和验证 Request/Response 模型。
*   **Docker 就绪**：经过优化的多阶段 Docker 构建流程。

## 🛠️ 快速开始

### 前置条件

*   Go 1.25+
*   Docker (可选)

### 本地开发

1.  **编译二进制文件**
    ```bash
    go build -o server main.go
    ```

2.  **运行服务**
    ```bash
    ./server
    ```
    服务启动后地址如下：
    *   HTTP Server: `http://0.0.0.0:1001`
    *   WebSocket Server: `ws://0.0.0.0:1002`

3.  **查看文档**
    在浏览器中打开 `http://localhost:1001/doc`。

### 🐳 Docker 部署

**选项 1: 使用 Docker Compose (推荐)**

这是构建和启动服务最简单的方法。

```bash
docker compose up -d --build
```

**选项 2: 手动构建与运行**

1.  **构建镜像**
    ```bash
    docker build -t go-port-trans .
    ```

2.  **运行容器**
    ```bash
    docker run -d \
      -p 1001:1001 \
      -p 1002:1002 \
      --name port-trans \
      go-port-trans
    ```
    *注意：如果遇到 `image 'go-port-trans:latest' not found` 错误，请确保步骤 1 构建成功。*

## 📚 API 文档与用法

本项目内置了自托管的文档页面。
访问 `/doc` 端点 (例如 `http://localhost:1001/doc`) 可以：
*   查看所有可用端点。
*   检查 Request/Response JSON Schema。
*   **一键复制**：复制完整的 API 定义上下文供 AI 助手使用。

### WebSocket 协议

WebSocket 接口使用简单的信封协议 (Envelope Protocol) 将请求路由到与 HTTP API 相同的处理程序。

**连接地址**: `ws://<host>:1002`

**请求信封 (Request Envelope)**:
```json
{
  "path": "/api/connect",      // 对应 HTTP 路由路径
  "id": "unique-req-id",       // 可选的相关性 ID
  "data": {                    // 实际请求负载 (与 HTTP POST body 相同)
    "key": "...",
    "deviceId": 123
  }
}
```

**响应信封 (Response Envelope)**:
```json
{
  "id": "unique-req-id",       // 回显请求 ID
  "path": "/api/connect",
  "success": true,             // 执行状态
  "message": "",               // 失败时的错误信息
  "data": { ... }              // 响应负载
}
```

## 📂 项目结构

```
.
├── Dockerfile              # Docker 构建配置
├── main.go                 # 应用程序入口与路由注册
├── run.sh                  # 启动脚本
├── internal/
│   ├── manager/            # 状态管理 (单例)
│   ├── model/              # Request/Response 结构体定义
│   └── service/            # 业务逻辑实现
└── pkg/
    ├── framework/          # Web/WS 框架与自动文档引擎
    └── portmap/            # 核心端口转发逻辑
```

## 🔗 关联项目

*   **前端仓库地址**：[https://github.com/cih1996/jp-cloud-script/](https://github.com/cih1996/jp-cloud-script/)
