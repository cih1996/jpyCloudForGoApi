package websocketClient

import (
	"crypto/tls"
	"fmt"
	"github.com/ghp3000/logs"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"net/url"
	"time"
)

// 定义消息类型常量
const (
	cmdConnect        = iota // 连接命令
	cmdDisconnect            // 断开连接命令
	cmdSend                  // 发送消息命令
	cmdClose                 // 关闭命令
	cmdUpdateToken           // 更新token命令
	cmdGetStatus             // 获取状态命令
	cmdStartHeartbeat        // 启动心跳命令
)

// 命令通道消息结构
type command struct {
	typ     int              // 命令类型
	payload interface{}      // 命令数据
	result  chan interface{} // 结果通道
}

// 发送消息结构
type sendMessage struct {
	messageType int    // 消息类型
	data        []byte // 消息数据
}

// WSClient WebSocket客户端结构体
type WSClient struct {
	url               string          // WebSocket服务器地址
	Conn              *websocket.Conn // WebSocket连接
	isConnected       bool            // 连接状态
	reconnectTime     time.Duration   // 重连时间间隔
	maxReconnectTime  time.Duration   // 最大重连时间间隔
	done              chan struct{}   // 关闭信号
	cmdChan           chan command    // 命令通道
	heartbeatStopped  chan struct{}   // 心跳停止信号
	heartbeatInterval time.Duration   // 心跳间隔

	// 回调函数
	onConnect    func()                                // 连接成功回调
	onDisconnect func(err error)                       // 连接断开回调
	onMessage    func(messageType int, message []byte) // 收到消息回调
	onError      func(err error)                       // 错误回调
}

// 辅助函数，安全关闭WebSocket连接
func SafeCloseConn(conn *websocket.Conn) {
	if conn != nil {
		if err := conn.Close(); err != nil {
			log.Printf("关闭WebSocket连接出错: %v\n", err)
		}
	}
}

// 辅助函数，设置读取超时
func setReadDeadline(conn *websocket.Conn, timeout time.Duration) error {
	if conn == nil {
		return fmt.Errorf("连接为空")
	}
	return conn.SetReadDeadline(time.Now().Add(timeout))
}

// 辅助函数，处理关闭消息发送错误
func sendCloseMessage(conn *websocket.Conn) {
	if conn != nil {
		err := conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Printf("发送关闭消息失败: %v\n", err)
		}
	}
}

// NewWSClient 创建新的WebSocket客户端
func NewWSClient(fullUrl string) *WSClient {
	// 使用httpClient.go中定义的WsUrl加上用户token

	logs.Info("WebSocket连接地址: %s\n", fullUrl)

	client := &WSClient{
		url:               fullUrl,
		reconnectTime:     time.Second * 2,  // 初始重连时间为2秒
		maxReconnectTime:  time.Second * 10, // 最大重连时间为60秒
		done:              make(chan struct{}),
		cmdChan:           make(chan command),
		heartbeatStopped:  make(chan struct{}),
		heartbeatInterval: time.Second * 5, // 心跳间隔为10秒
	}

	// 启动命令处理goroutine
	go client.commandLoop()

	return client
}

// commandLoop 处理命令的主循环SetCallbacks
func (c *WSClient) commandLoop() {
	for {
		select {
		case <-c.done:
			// 客户端被关闭
			if c.Conn != nil {
				sendCloseMessage(c.Conn)
				SafeCloseConn(c.Conn)
			}
			return

		case cmd := <-c.cmdChan:
			switch cmd.typ {
			case cmdConnect:
				// 执行连接
				c.handleConnect()
				if cmd.result != nil {
					cmd.result <- nil
				}

			case cmdDisconnect:
				// 断开连接
				if c.Conn != nil {
					SafeCloseConn(c.Conn)
					c.Conn = nil
				}
				c.isConnected = false
				if cmd.result != nil {
					cmd.result <- nil
				}

			case cmdSend:
				// 发送消息
				var err error
				if msg, ok := cmd.payload.(sendMessage); ok && c.isConnected && c.Conn != nil {
					err = c.Conn.WriteMessage(msg.messageType, msg.data)
					if err != nil {
						log.Printf("发送消息失败: %v\n", err)
						if c.onError != nil {
							c.onError(err)
						}
					}
				}
				if cmd.result != nil {
					cmd.result <- err
				}

			case cmdClose:
				// 关闭客户端
				close(c.done)
				if c.Conn != nil {
					sendCloseMessage(c.Conn)
					SafeCloseConn(c.Conn)
					c.Conn = nil
				}
				c.isConnected = false
				if cmd.result != nil {
					cmd.result <- nil
				}
				return

			case cmdUpdateToken:
				// 更新token
				if token, ok := cmd.payload.(string); ok {
					c.url = c.modUrl(c.url, token)
					needReconnect := c.isConnected

					if needReconnect && c.Conn != nil {
						SafeCloseConn(c.Conn)
						c.Conn = nil
						c.isConnected = false
					}

					if needReconnect {
						c.handleConnect()
					}
				}
				if cmd.result != nil {
					cmd.result <- nil
				}

			case cmdGetStatus:
				// 获取状态
				if cmd.result != nil {
					cmd.result <- c.isConnected
				}
			case cmdStartHeartbeat:
				// 启动心跳
				c.startHeartbeat()
				if cmd.result != nil {
					cmd.result <- nil
				}
			}
		}
	}
}

func (c *WSClient) Url变更(url string) {
	// 先通过命令通道关闭现有连接
	c.Close()

	// 更新URL
	c.url = url

	// 不要自动重连，让上层控制重连
}

// handleConnect 处理连接逻辑
func (c *WSClient) handleConnect() {
	// 配置跳过证书验证的dialer
	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	// 设置请求头
	header := http.Header{}
	header.Add("Content-Type", "application/json")

	// 尝试建立连接
	conn, _, err := dialer.Dial(c.url, header)

	if err != nil {
		log.Printf("WebSocket连接失败: %v\n", err)
		if c.onError != nil {
			c.onError(err)
		}
		// 启动重连
		//go c.reconnect()
		return
	}

	c.Conn = conn
	c.isConnected = true

	if c.onConnect != nil {
		c.onConnect()
	}

	// 启动心跳机制
	c.startHeartbeat()

	// 启动消息监听
	go c.readPump()
}

// SetCallbacks 设置回调函数
func (c *WSClient) SetCallbacks(
	onConnect func(),
	onDisconnect func(err error),
	onMessage func(messageType int, message []byte),
	onError func(err error)) {

	c.onConnect = onConnect
	c.onDisconnect = onDisconnect
	c.onMessage = onMessage
	c.onError = onError
}

// Connect 连接到WebSocket服务器
func (c *WSClient) Connect() {
	cmd := command{
		typ:    cmdConnect,
		result: make(chan interface{}),
	}
	c.cmdChan <- cmd
	<-cmd.result // 等待连接完成
}

// reconnect 自动重连功能
func (c *WSClient) reconnect() {
	currentReconnectTime := c.reconnectTime
	for {
		select {
		case <-c.done:
			return
		case <-time.After(currentReconnectTime):
			log.Printf("尝试重新连接WebSocket服务器...\n")

			cmd := command{
				typ:    cmdConnect,
				result: make(chan interface{}),
			}
			c.cmdChan <- cmd
			<-cmd.result

			// 检查连接结果
			statusCmd := command{
				typ:    cmdGetStatus,
				result: make(chan interface{}),
			}
			c.cmdChan <- statusCmd
			isConnected := (<-statusCmd.result).(bool)

			if !isConnected {
				// 连接失败，增加重连时间（指数退避）
				currentReconnectTime = time.Duration(float64(currentReconnectTime) * 1.5)
				if currentReconnectTime > c.maxReconnectTime {
					currentReconnectTime = c.maxReconnectTime
				}
				log.Printf("重连失败, 下次重连时间: %v\n", currentReconnectTime)
				continue
			}

			// 重连成功，退出循环
			return
		}
	}
}

// readPump 持续监听并接收消息
func (c *WSClient) readPump() {
	defer func() {
		// 断开连接，发送通知
		cmd := command{
			typ:    cmdDisconnect,
			result: make(chan interface{}),
		}
		c.cmdChan <- cmd
		<-cmd.result

		// 通知断开连接
		if c.onDisconnect != nil {
			c.onDisconnect(nil)
		}

		// 不再自动重连，改为由上层处理
		// go c.reconnect()
	}()

	// 设置读取超时
	if err := setReadDeadline(c.Conn, time.Minute*5); err != nil {
		log.Printf("设置读取超时失败: %v\n", err)
		return
	}

	// 设置Pong处理函数
	c.Conn.SetPongHandler(func(string) error {
		err := setReadDeadline(c.Conn, time.Minute*5)
		if err != nil {
			log.Printf("Pong处理: 设置读取超时失败: %v\n", err)
		}
		return err
	})

	for {
		messageType, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket读取错误: %v\n", err)
			}
			// 处理错误回调
			if c.onError != nil {
				c.onError(err)
			}
			return
		}

		// 收到消息回调
		if c.onMessage != nil {
			c.onMessage(messageType, message)
		}
	}
}

// SendMessage 发送消息
func (c *WSClient) SendMessage(messageType int, data []byte) error {
	cmd := command{
		typ: cmdSend,
		payload: sendMessage{
			messageType: messageType,
			data:        data,
		},
		result: make(chan interface{}),
	}
	c.cmdChan <- cmd
	result := <-cmd.result

	if result == nil {
		return nil
	}
	return result.(error)
}

// SendTextMessage 发送文本消息
func (c *WSClient) SendTextMessage(message string) error {
	return c.SendMessage(websocket.TextMessage, []byte(message))
}

// SendBinaryMessage 发送二进制消息
func (c *WSClient) SendBinaryMessage(data []byte) error {
	return c.SendMessage(websocket.BinaryMessage, data)
}

// Close 关闭WebSocket连接
func (c *WSClient) Close() {
	cmd := command{
		typ:    cmdClose,
		result: make(chan interface{}),
	}
	c.cmdChan <- cmd
	<-cmd.result
}

// IsConnected 返回连接状态
func (c *WSClient) IsConnected() bool {
	cmd := command{
		typ:    cmdGetStatus,
		result: make(chan interface{}),
	}
	c.cmdChan <- cmd
	return (<-cmd.result).(bool)
}

// UpdateToken 更新token
func (c *WSClient) UpdateToken(token string) {
	cmd := command{
		typ:     cmdUpdateToken,
		payload: token,
		result:  make(chan interface{}),
	}
	c.cmdChan <- cmd
	<-cmd.result
}

// modWsUrl 替换token
func (c *WSClient) modUrl(fullUrl string, token string) string {
	parsedURL, err := url.Parse(fullUrl)
	if err != nil {
		fmt.Println("Error parsing URL:", err)
		return "nil"
	}
	// 解析查询参数
	query := parsedURL.Query()

	// 替换token的值
	query.Set("token", token)

	// 将修改后的查询参数设置回URL
	parsedURL.RawQuery = query.Encode()

	// 输出修改后的URL
	return parsedURL.String()
}

// 添加心跳机制
func (c *WSClient) startHeartbeat() {
	// 先关闭之前的心跳（如果有）
	select {
	case <-c.heartbeatStopped:
	default:
		close(c.heartbeatStopped)
		c.heartbeatStopped = make(chan struct{})
	}

	// 启动新的心跳
	go func() {
		ticker := time.NewTicker(c.heartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-c.heartbeatStopped:
				log.Println("心跳停止")
				return
			case <-ticker.C:
				if !c.IsConnected() {
					return
				}

				// 发送ping消息
				err := c.Conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(time.Second*3))
				if err != nil {
					log.Printf("发送心跳失败: %v\n", err)

					// 如果心跳发送失败，尝试重新连接
					cmd := command{
						typ:    cmdDisconnect,
						result: make(chan interface{}),
					}
					c.cmdChan <- cmd
					<-cmd.result
					return
				}

				//log.Println("发送心跳")
			}
		}
	}()
}

// StartHeartbeat 启动心跳
func (c *WSClient) StartHeartbeat() {
	cmd := command{
		typ:    cmdStartHeartbeat,
		result: make(chan interface{}),
	}
	c.cmdChan <- cmd
	<-cmd.result

	c.startHeartbeat()
}
