package service

import (
	"context"
	"fmt"
	"port-mapping-demo/internal/config"
	"port-mapping-demo/internal/devicews"
	"port-mapping-demo/internal/manager"
	"port-mapping-demo/internal/model"
	"port-mapping-demo/third_party/JpyApiAgent/table/coreClass/middleAgentRtc"
	"sync"
	"time"

	"adminApi"
	centCtl "port-mapping-demo/third_party/JpyApiAgent/table/coreClass/centControlPlatform"

	"github.com/ghp3000/logs"
)

var (
	// jpyCore 是 JpyApiAgent 的核心对象，封装了登录、RTC连接、端口映射等全部通讯逻辑
	jpyCore     *centCtl.Core
	currentKey  string
	currentHost string // 当前连接的服务器地址
	loginLock   sync.Mutex

	// deviceWSServer 设备 WebSocket 服务器
	deviceWSServer *devicews.Server
)

// SetDeviceWSServer 设置设备 WebSocket 服务器
func SetDeviceWSServer(server *devicews.Server) {
	deviceWSServer = server
}

// GetDeviceWSServer 获取设备 WebSocket 服务器
func GetDeviceWSServer() *devicews.Server {
	return deviceWSServer
}

// EnsureLogin 确保已登录集控平台
// 使用 JpyApiAgent 的 NewCore + Login + CenterGetDeviceList 替代旧的手动 WS 连接
// host 参数可选，如果传入则更新配置并使用新地址连接
func EnsureLogin(key string, host string) error {
	loginLock.Lock()
	defer loginLock.Unlock()

	if key == "" {
		return fmt.Errorf("secret key is required")
	}

	logs.Info("[EnsureLogin] 开始登录, host=%s, currentHost=%s, currentKey=%s", host, currentHost, currentKey)

	// 如果传入了新的 host，先更新配置
	if host != "" {
		// 清理 host：去掉协议前缀和路径后缀
		cleanHost := host
		if len(cleanHost) > 0 {
			// 去掉 https:// 前缀
			if len(cleanHost) > 8 && cleanHost[:8] == "https://" {
				cleanHost = cleanHost[8:]
			}
			// 去掉 http:// 前缀
			if len(cleanHost) > 7 && cleanHost[:7] == "http://" {
				cleanHost = cleanHost[7:]
			}
			// 去掉 wss:// 或 ws:// 前缀
			if len(cleanHost) > 6 && cleanHost[:6] == "wss://" {
				cleanHost = cleanHost[6:]
			}
			if len(cleanHost) > 5 && cleanHost[:5] == "ws://" {
				cleanHost = cleanHost[5:]
			}
			// 去掉 /ws 后缀
			if len(cleanHost) > 3 && cleanHost[len(cleanHost)-3:] == "/ws" {
				cleanHost = cleanHost[:len(cleanHost)-3]
			}
			// 去掉末尾的 /
			if len(cleanHost) > 0 && cleanHost[len(cleanHost)-1] == '/' {
				cleanHost = cleanHost[:len(cleanHost)-1]
			}
		}

		newWsUrl := "wss://" + cleanHost + "/ws"
		logs.Info("[EnsureLogin] cleanHost=%s, newWsUrl=%s, configWsUrl=%s", cleanHost, newWsUrl, config.GetWsUrl())

		// 检查地址是否变化
		if newWsUrl != config.GetWsUrl() || cleanHost != currentHost {
			logs.Info("[EnsureLogin] 服务器地址变更: %s -> %s", config.GetWsUrl(), newWsUrl)
			if err := config.SetWsUrl(newWsUrl); err != nil {
				logs.Error("保存配置失败: %v", err)
			}
			// 地址变更，清理旧的中间件连接，强制重新连接
			middleAgentRtc.ClearAll()
			if jpyCore != nil {
				jpyCore.Reset()
			}
			currentKey = ""
			currentHost = ""
			jpyCore = nil
		}
	}

	// 已登录且 key 和 host 都未变，直接返回
	if key == currentKey && currentHost != "" && jpyCore != nil && jpyCore.GetApi() != nil {
		return nil
	}

	// 切换用户或重新登录，清理旧状态
	if jpyCore != nil {
		logs.Info("切换用户，清理旧连接并重新初始化 JpyApiAgent Core...")
		middleAgentRtc.ClearAll()
		jpyCore.Reset()
		jpyCore = nil
	}

	// 获取集控平台地址（去掉 wss:// 前缀和 /ws 后缀，JpyApiAgent 内部会自行拼接）
	tableIP := config.GetTableIP()
	if tableIP == "" {
		return fmt.Errorf("未登录，请先通过前端或 CLI 登录集控平台")
	}

	logs.Info("正在通过 JpyApiAgent 连接集控平台: %s", tableIP)

	// 创建 JpyApiAgent Core 并登录
	jpyCore = centCtl.NewCore(tableIP, key)
	if success := jpyCore.Login(); !success {
		jpyCore = nil
		return fmt.Errorf("集控平台登录失败")
	}

	logs.Info("集控平台登录成功")
	currentKey = key
	currentHost = tableIP

	// 将 Core 设置到 Manager 中供其他模块使用
	manager.GetInstance().SetCore(jpyCore)

	// 获取设备列表（内部会自动创建所有中间件的 RTC 连接）
	_, err := jpyCore.CenterGetDeviceList()
	if err != nil {
		logs.Error("获取设备列表失败: %v", err)
		// 登录成功但获取设备列表失败，不影响登录状态
	} else {
		logs.Info("设备列表获取成功，中间件 RTC 连接已自动建立")
	}

	return nil
}

// forceRelogin 强制重新登录
func forceRelogin(key string) error {
	if key == "" {
		return fmt.Errorf("secret key is required")
	}
	logs.Info("[forceRelogin] 开始强制重新登录...")

	// 清理所有中间件对象（它们持有旧 session 的引用）
	middleAgentRtc.ClearAll()

	// 重置 Core 状态（清理 token、设备列表等）
	if jpyCore != nil {
		jpyCore.Reset()
	}

	loginLock.Lock()
	currentKey = ""
	currentHost = ""
	jpyCore = nil
	loginLock.Unlock()

	logs.Info("[forceRelogin] 已清理旧状态，开始重新登录...")
	return EnsureLogin(key, "")
}

// GetGlobalApi 返回 JpyApiAgent 内部的 AdminApi 实例
// 供需要直接调用集控平台 API 的处理器使用（如 ChangeOs、HideApp、SetS5 等）
func GetGlobalApi() *adminApi.AdminApi {
	if jpyCore == nil {
		return nil
	}
	return jpyCore.GetApi()
}

// GetJpyCore 返回 JpyApiAgent Core 实例
func GetJpyCore() *centCtl.Core {
	return jpyCore
}

// GetCurrentKey 返回当前登录的 key（供 RPA 步骤重连使用）
func GetCurrentKey() string {
	loginLock.Lock()
	defer loginLock.Unlock()
	return currentKey
}

// GetCurrentHost 返回当前登录的 host（供调试使用）
func GetCurrentHost() string {
	loginLock.Lock()
	defer loginLock.Unlock()
	return currentHost
}

// GetDevices 获取设备列表
func GetDevices(ctx context.Context, req *model.GetDevicesRequest) (*ginHWrapper, error) {
	if err := EnsureLogin(req.Key, ""); err != nil {
		return nil, err
	}

	// 使用 JpyApiAgent Core 获取设备列表
	list, err := jpyCore.CenterGetDeviceList()
	if err != nil {
		return nil, fmt.Errorf("获取设备列表失败: %v", err)
	}

	mgr := manager.GetInstance()
	return &ginHWrapper{
		"devices":  list,
		"mappings": mgr.GetAllMappings(),
	}, nil
}

// GetMappings 获取端口映射列表
func GetMappings(ctx context.Context, req *model.GetMappingsRequest) (*ginHWrapper, error) {
	mgr := manager.GetInstance()
	return &ginHWrapper{
		"mappings": mgr.GetAllMappings(),
	}, nil
}

// ConnectDevice 创建端口映射连接
// 使用 JpyApiAgent 的 CreatPortMapSocket5Rtc 替代旧的手动 RTC Token 获取和连接
func ConnectDevice(ctx context.Context, req *model.ConnectRequest) (*model.Response, error) {
	if err := EnsureLogin(req.Key, ""); err != nil {
		return nil, err
	}

	mgr := manager.GetInstance()

	// 检查端口是否已被占用
	if mgr.HasMapping(req.LocalPort) {
		logs.Info("端口已被占用，正在强制断开旧连接 LocalPort: %d", req.LocalPort)
		// 清理旧的端口映射
		if portMap, ok := jpyCore.GetPortMap(req.DeviceId); ok {
			if portMap.Rtc != nil {
				portMap.Reconnect = 1
				_ = portMap.Rtc.Close()
			}
		}
		mgr.RemoveMapping(req.LocalPort)
		time.Sleep(100 * time.Millisecond)
	}

	logs.Info("收到连接请求 Device: %d, Local: %d, Phone: %d", req.DeviceId, req.LocalPort, req.PhonePort)

	// 使用 JpyApiAgent 创建端口映射（内部自动获取 Token、建立 RTC 连接、启动本地转发）
	_, err := jpyCore.CreatPortMapSocket5Rtc(req.DeviceId, req.PhonePort, req.LocalPort, 0)
	if err != nil {
		return nil, fmt.Errorf("创建端口映射失败: %v", err)
	}

	session := model.MappingSession{
		DeviceId:            req.DeviceId,
		TbYunJiUserDeviceId: req.TbYunJiUserDeviceId,
		LocalPort:           req.LocalPort,
		PhonePort:           req.PhonePort,
		CreateTime:          time.Now(),
		Status:              "active",
		Key:                 req.Key,
	}

	mgr.AddMapping(req.LocalPort, session)

	return &model.Response{
		Success: true,
		Message: "Connection initiated",
		Data:    fmt.Sprintf("127.0.0.1:%d -> Device:%d", req.LocalPort, req.PhonePort),
	}, nil
}

// DisconnectDevice 断开端口映射连接
func DisconnectDevice(ctx context.Context, req *model.DisconnectRequest) (*model.Response, error) {
	mgr := manager.GetInstance()

	// 通过 JpyApiAgent 清理端口映射
	if jpyCore != nil {
		// 查找对应的设备ID（从 mapping session 中获取）
		if session, ok := mgr.GetMapping(req.LocalPort); ok {
			if portMap, ok := jpyCore.GetPortMap(session.DeviceId); ok {
				if portMap.Rtc != nil {
					portMap.Reconnect = 1 // 标记为手动断开，不自动重连
					_ = portMap.Rtc.Close()
				}
			}
		}
	}

	if mgr.HasMapping(req.LocalPort) {
		mgr.RemoveMapping(req.LocalPort)
	}

	logs.Info("断开连接 LocalPort: %d", req.LocalPort)

	return &model.Response{
		Success: true,
		Message: fmt.Sprintf("Disconnected port %d", req.LocalPort),
	}, nil
}

// ginHWrapper 任意 JSON 响应的辅助类型
type ginHWrapper map[string]interface{}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	WsUrl string `json:"wsUrl" binding:"required"`
}

// UpdateConfig 更新集控平台地址配置
func UpdateConfig(ctx context.Context, req *UpdateConfigRequest) (*model.Response, error) {
	oldUrl := config.GetWsUrl()
	if req.WsUrl == oldUrl {
		return &model.Response{
			Success: true,
			Message: "Configuration unchanged",
		}, nil
	}

	if err := config.SetWsUrl(req.WsUrl); err != nil {
		return nil, fmt.Errorf("failed to save config: %v", err)
	}

	// 地址变更，强制重新连接
	loginLock.Lock()
	if jpyCore != nil {
		logs.Info("配置变更，关闭现有连接以强制重连")
		jpyCore = nil
		currentKey = ""
	}
	loginLock.Unlock()

	return &model.Response{
		Success: true,
		Message: "Configuration updated. Reconnection will occur on next request.",
	}, nil
}

// GetConfigRequest 获取配置请求
type GetConfigRequest struct{}

// GetConfig 获取当前配置
func GetConfig(ctx context.Context, req *GetConfigRequest) (*ginHWrapper, error) {
	return &ginHWrapper{
		"wsUrl": config.GetWsUrl(),
	}, nil
}
