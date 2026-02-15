package centControlPlatform

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/ghp3000/logs"
	"github.com/ghp3000/netclient/NetClient"
	"github.com/ghp3000/netclient/bufferPool"
	"github.com/ghp3000/public"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var HttpServer *gin.Engine
var ProxyTarget = "https://minio.accjs.cn" // 这里设置目标转发地址
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有跨域请求
	},
}

type bodyInfo struct {
	App  string          `json:"app"`
	Fun  string          `json:"fun"`
	Data json.RawMessage `json:"data"` // 使用 RawMessage 避免解析具体数据，提高性能
}

func (c *Core) InitHttpServer() {
	HttpServer = gin.Default()

	// 1. 初始化反向代理 (只执行一次，复用连接池)
	target, err := url.Parse(ProxyTarget)
	if err != nil {
		logs.Error("解析目标 URL 失败: %v", err)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	// 配置 Transport：忽略证书验证，并优化连接池配置
	proxy.Transport = &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	// CORS 中间件 — 允许 api-docs.html 等跨域前端调用
	HttpServer.Use(func(ss *gin.Context) {
		ss.Header("Access-Control-Allow-Origin", "*")
		ss.Header("Access-Control-Allow-Methods", "POST, OPTIONS")
		ss.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if ss.Request.Method == http.MethodOptions {
			ss.AbortWithStatus(204)
			return
		}
		ss.Next()
	})

	HttpServer.Use(func(ss *gin.Context) {
		// 判断是否是 WebSocket 请求
		if ss.IsWebsocket() {
			c.handleWebSocket(ss)
			ss.Abort()
			return
		}

		// 限制只允许 POST 请求
		if ss.Request.Method != http.MethodPost {
			ss.JSON(http.StatusMethodNotAllowed, gin.H{"code": -1, "msg": "method not allowed"})
			ss.Abort()
			return
		}

		// 2. 读取并解析请求体 (业务逻辑需要)
		var bodyBytes []byte
		if ss.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(ss.Request.Body)
			ss.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		var bi bodyInfo
		if len(bodyBytes) > 0 {
			// 只解析需要的字段，忽略 data 字段的详细解析
			_ = json.Unmarshal(bodyBytes, &bi)
		}

		// 调用自定义业务逻辑
		if handled := c.handleCustomLogic(ss, &bi); handled {
			ss.Abort()
			return
		}

		// 4. 执行转发
		// 修改请求头以符合代理要求
		ss.Request.Host = target.Host
		ss.Request.URL.Host = target.Host
		ss.Request.URL.Scheme = target.Scheme

		// 设置 Token
		ss.Request.Header.Set("Cookie", fmt.Sprintf("goAdminToken=%s", c.token))

		// 直接复用 proxy 实例进行转发
		proxy.ServeHTTP(ss.Writer, ss.Request)

		// 代理执行完后中止 Gin 的后续处理
		ss.Abort()
	})

	go HttpServer.Run("0.0.0.0:8888")
}

// handleCustomLogic 处理自定义业务逻辑
// 返回 true 表示已处理（不需转发），false 表示继续转发
func (c *Core) handleCustomLogic(ss *gin.Context, bi *bodyInfo) bool {
	logs.Info("检测解析==============", bi.App, bi.Fun)

	// 1. 登录控制拦截
	if bi.App == "loginCtl" {
		if bi.Fun == "secretKeyLogin" {
			return c.httpLogin(ss, bi)
		}
		ss.JSON(http.StatusOK, gin.H{"code": 0, "message": "not forwarded"})
		return true
	}
	// 2. 设备列表拦截
	if bi.App == "userDeviceCtl" && bi.Fun == "getUserDeviceList" {
		c.httpGetDeviceList(ss, bi)
		return true
	}
	// 新增：自定义登录接口
	if bi.App == "userLogin" && bi.Fun == "apiLogin" {
		return c.httpLogin(ss, bi)
	}
	// 3. 中间件同步命令拦截
	if bi.App == "middleAgent" {
		return c.httpMiddleAgentSync(ss, bi)
	}
	// 4. 端口映射拦截
	if bi.App == "portMap" {
		return c.httpPortMap(ss, bi)
	}
	return false
}

// handleWebSocket 处理 WebSocket 请求，根据 URL 参数分发
func (c *Core) handleWebSocket(ss *gin.Context) {
	middleWareIdStr := ss.Query("middleWareId")
	deviceIdStr := ss.Query("deviceId")

	hasMiddle := middleWareIdStr != ""
	hasDevice := deviceIdStr != ""

	// 两个都有或都没有 → 错误
	if hasMiddle == hasDevice {
		ss.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "请指定 middleWareId 或 deviceId 其中之一"})
		return
	}

	if hasMiddle {
		var middleWareId uint64
		_, _ = fmt.Sscanf(middleWareIdStr, "%d", &middleWareId)
		if middleWareId == 0 {
			ss.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "无效的 middleWareId"})
			return
		}
		c.handleMiddleWareWS(ss, middleWareId)
	} else {
		var deviceId uint64
		_, _ = fmt.Sscanf(deviceIdStr, "%d", &deviceId)
		if deviceId == 0 {
			ss.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "无效的 deviceId"})
			return
		}
		c.handleDeviceWS(ss, deviceId)
	}
}

// handleMiddleWareWS 中间件 WS 桥接
func (c *Core) handleMiddleWareWS(ss *gin.Context, middleWareId uint64) {
	// 升级 WS
	wsConn, err := upgrader.Upgrade(ss.Writer, ss.Request, nil)
	if err != nil {
		logs.Error("[MiddleWareWS] 升级连接失败: %v", err)
		return
	}
	logs.Info("[MiddleWareWS] 连接建立, middleWareId: %d", middleWareId)

	// 连接复用：先查已有 RTC，没有才创建
	middleAgent, ok := c.GetMiddleAgent(middleWareId)
	if !ok || middleAgent == nil {
		// 需要创建新的 RTC 连接
		middleAgent, err = c.CreatMiddlewareRtc(middleWareId, nil, nil, nil)
		if err != nil {
			logs.Error("[MiddleWareWS] 创建 RTC 失败: %v", err)
			_ = wsConn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"code":-1,"msg":"创建RTC失败: %v"}`, err)))
			_ = wsConn.Close()
			return
		}
	}

	// 生成唯一 ID 并注册 WS 客户端
	clientId := fmt.Sprintf("ws-%d-%p", middleWareId, wsConn)
	middleAgent.AddWsClient(clientId, wsConn)

	// WS 读取循环 (WS → RTC)
	go func() {
		defer func() {
			middleAgent.RemoveWsClient(clientId)
			_ = wsConn.Close()
			logs.Info("[MiddleWareWS] 连接关闭, middleWareId: %d, client: %s", middleWareId, clientId)
		}()

		for {
			_, message, err := wsConn.ReadMessage()
			if err != nil {
				logs.Info("[MiddleWareWS] 读取消息结束: %v", err)
				break
			}

			// 解析 WS JSON 消息
			var wsMsg struct {
				F        uint16      `json:"f"`
				DeviceId uint64      `json:"deviceId"`
				Data     interface{} `json:"data"`
				Req      bool        `json:"req"`
			}
			if err := json.Unmarshal(message, &wsMsg); err != nil {
				logs.Error("[MiddleWareWS] JSON解析失败: %v, raw: %s", err, string(message))
				continue
			}

			if middleAgent.Rtc == nil {
				logs.Error("[MiddleWareWS] RTC 未就绪, middleWareId: %d", middleWareId)
				continue
			}

			// 构造 public.Message
			msg := public.NewMessage(bufferPool.TypeMsgpack, wsMsg.F, atomic.AddUint32(&middleAgent.Seq, 1))
			msg.Req = wsMsg.Req
			if wsMsg.Data != nil {
				if err := msg.Marshal(wsMsg.Data); err != nil {
					logs.Error("[MiddleWareWS] 消息Marshal失败: %v", err)
					continue
				}
			}

			// 通过 RTC 发送
			if err := middleAgent.Rtc.SendMsgpack(msg, wsMsg.DeviceId); err != nil {
				logs.Error("[MiddleWareWS] RTC发送失败: %v", err)
			}
		}
	}()
}

// handleDeviceWS 设备 WS 桥接
func (c *Core) handleDeviceWS(ss *gin.Context, deviceId uint64) {
	// 升级 WS
	wsConn, err := upgrader.Upgrade(ss.Writer, ss.Request, nil)
	if err != nil {
		logs.Error("[DeviceWS] 升级连接失败: %v", err)
		return
	}
	logs.Info("[DeviceWS] 连接建立, deviceId: %d", deviceId)

	// 创建设备 RTC（一对一）
	onOpen := func(conn NetClient.NetClient) {
		logs.Info("[DeviceWS] 设备RTC连接已打开: %d", deviceId)
	}
	onClose := func(conn NetClient.NetClient) {
		logs.Info("[DeviceWS] 设备RTC连接已关闭: %d", deviceId)
		_ = wsConn.Close()
	}

	device, err := c.CreatDeviceH264AudioRtc(deviceId, onOpen, onClose, nil)
	if err != nil {
		logs.Error("[DeviceWS] 创建设备RTC失败: %v", err)
		_ = wsConn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"code":-1,"msg":"创建设备RTC失败: %v"}`, err)))
		_ = wsConn.Close()
		return
	}

	// 将 WS 连接存到设备对象
	device.WsConn = wsConn

	// WS 读取循环 (WS → RTC)
	go func() {
		defer func() {
			// 停止视频/音频流
			device.AsyncStopVideo()
			device.AsyncStopAudio()
			device.WsConn = nil
			_ = wsConn.Close()
			device.Reconnect = 1
			// 关闭设备 RTC
			if device.Rtc != nil {
				_ = device.Rtc.Close()
			}
			logs.Info("[DeviceWS] 连接关闭, deviceId: %d", deviceId)
		}()

		for {
			_, message, err := wsConn.ReadMessage()
			if err != nil {
				logs.Info("[DeviceWS] 读取消息结束: %v", err)
				break
			}

			// 解析 WS JSON 消息
			var wsMsg struct {
				F    uint16      `json:"f"`
				Data interface{} `json:"data"`
				Req  bool        `json:"req"`
			}
			if err := json.Unmarshal(message, &wsMsg); err != nil {
				logs.Error("[DeviceWS] JSON解析失败: %v, raw: %s", err, string(message))
				continue
			}

			if device.Rtc == nil {
				logs.Error("[DeviceWS] RTC 未就绪, deviceId: %d", deviceId)
				continue
			}

			// 构造 public.Message
			msg := public.NewMessage(bufferPool.TypeMsgpack, wsMsg.F, atomic.AddUint32(&device.Seq, 1))
			msg.Req = wsMsg.Req
			if wsMsg.Data != nil {
				if err := msg.Marshal(wsMsg.Data); err != nil {
					logs.Error("[DeviceWS] 消息Marshal失败: %v", err)
					continue
				}
			}

			// 设备直连，deviceId=0
			if err := device.Rtc.SendMsgpack(msg, 0); err != nil {
				logs.Error("[DeviceWS] RTC发送失败: %v", err)
			}
		}
	}()
}
