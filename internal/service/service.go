package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"port-mapping-demo/coreClass"
	"port-mapping-demo/internal/config"
	"port-mapping-demo/internal/manager"
	"port-mapping-demo/internal/model"
	"sync"
	"time"

	"adminApi"
	"adminApi/loginCtl"
	"adminApi/rtcCtl"
	"adminApi/userDeviceCtl"

	"cnb.cool/accbot/goTool/sessionPkg"
	"cnb.cool/accbot/goTool/wsPkg"
	"github.com/ghp3000/logs"
	"github.com/gorilla/websocket"
)

var (
	s          *sessionPkg.Session
	globalApi  *adminApi.AdminApi
	currentKey string
	loginLock  sync.Mutex
)

// ensureLogin logic moved from main.go
func EnsureLogin(key string) error {
	loginLock.Lock()
	defer loginLock.Unlock()

	if key == "" {
		return fmt.Errorf("secret key is required")
	}

	if key == currentKey && s != nil && globalApi != nil {
		return nil
	}

	if s != nil {
		logs.Info("Switching user, closing old session...")
	}

	dialer := &websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	wsUrl := config.GetWsUrl()

	fmt.Println("Connecting to:", wsUrl)
	if wsUrl == "" {
		return fmt.Errorf("WebSocket URL is not configured. Please configure it via /api/config/update")
	}

	c, _, err := dialer.Dial(wsUrl, nil)
	if err != nil {
		logs.Error("连接底层服务失败", err)
		return fmt.Errorf("failed to connect to websocket server")
	}
	conn := wsPkg.NewWSConnByConn(c)
	s = sessionPkg.CreateSession(sessionPkg.SessionType_ws, conn)

	s.RegExpiryCallback(func(msg string) {
		logs.Debug("和服务端连接断开了", msg)
		loginLock.Lock()
		if currentKey == key {
			currentKey = ""
			s = nil
			globalApi = nil
		}
		loginLock.Unlock()
	})

	globalApi = adminApi.NewAdminApi(s)

	res, err1 := globalApi.LoginCtl.SecretKeyLogin(&loginCtl.SecretKeyLoginReq{SecretKey: &key})
	if err1 != nil {
		logs.Error("Login failed", err1.Msg)
		s = nil
		globalApi = nil
		return fmt.Errorf("login failed: %s", err1.Msg)
	}

	logs.Info("Login success", res.Token, res.UserInfo.UserName)
	currentKey = key

	// Set reconnection callback for auto-reconnection
	manager.GetInstance().Core.ReconnectCallback = func(proxyId uint64) {
		logs.Info("Attempting to reconnect proxy %d", proxyId)
		// Ensure globalApi is available
		if globalApi == nil {
			logs.Error("Cannot reconnect proxy %d: globalApi is nil", proxyId)
			return
		}

		pId := int64(proxyId)
		rtcRes, errApi := globalApi.RtcCtl.GetRtcToken(rtcCtl.GetRtcTokenReq{
			TbProxyId: &pId,
		})
		if errApi != nil {
			logs.Error("Reconnect proxy %d failed: get rtc token failed: %s", proxyId, errApi.Msg)
			return
		}

		rtcToken := coreClass.RtcToken{
			UserId:   0,
			DeviceId: 0,
			HostUrl:  rtcRes.Url,
			Token:    rtcRes.Token,
			GuestUrl: rtcRes.Url,
			Host:     fmt.Sprintf("%d", proxyId),
		}

		manager.GetInstance().Core.MiddleRtcConnect(rtcToken)
	}

	// Set unified middleware message callback
	manager.GetInstance().Core.CallbackUnifiedMiddlewareMessage = func(proxyId uint64, deviceId uint64, msgType int, data interface{}) {
		// logs.Info("Broadcasting upstream message from proxy %d, device %d", proxyId, deviceId)
		BroadcastToUnifiedClients(&UnifiedResponse{
			Type: "UpstreamMessage",
			Code: 200,
			Msg:  "Received upstream message",
			Data: map[string]interface{}{
				"proxyId":  proxyId,
				"deviceId": deviceId,
				"msgType":  msgType,
				"data":     data,
			},
		})
	}

	return nil
}

// GetGlobalApi returns the global AdminApi instance
func GetGlobalApi() *adminApi.AdminApi {
	return globalApi
}

// GetDevices Handler
func GetDevices(ctx context.Context, req *model.GetDevicesRequest) (*ginHWrapper, error) {
	if err := EnsureLogin(req.Key); err != nil {
		return nil, err
	}

	ret, err := globalApi.UserDeviceCtl.GetUserDeviceList(&userDeviceCtl.GetUserDeviceListReq{
		PageNum:                               0,
		PageSize:                              999999,
		GetUserDeviceListgetUserDeviceListReq: userDeviceCtl.GetUserDeviceListgetUserDeviceListReq{},
	})

	if err != nil {
		return nil, fmt.Errorf(err.Msg)
	}

	mgr := manager.GetInstance()
	return &ginHWrapper{
		"devices":  ret,
		"mappings": mgr.GetAllMappings(),
	}, nil
}

// GetMappings Handler
func GetMappings(ctx context.Context, req *model.GetMappingsRequest) (*ginHWrapper, error) {
	mgr := manager.GetInstance()
	return &ginHWrapper{
		"mappings": mgr.GetAllMappings(),
	}, nil
}

// ConnectDevice Handler
func ConnectDevice(ctx context.Context, req *model.ConnectRequest) (*model.Response, error) {
	if err := EnsureLogin(req.Key); err != nil {
		return nil, err
	}

	mgr := manager.GetInstance()

	// Check existing mapping
	if mgr.HasMapping(req.LocalPort) {
		logs.Info("端口已被占用，正在强制断开旧连接", fmt.Sprintf("LocalPort: %d", req.LocalPort))
		mgr.RemoveMapping(req.LocalPort)
	}

	// Clean up core
	if info, ok := mgr.Core.LocalPortGetPortMapInfo(req.LocalPort); ok {
		if info.Portforwarder != nil {
			info.Portforwarder.Close()
		}
		mgr.Core.LocalPortDelPortMapInfo(req.LocalPort)
		time.Sleep(100 * time.Millisecond)
	}

	logs.Info("收到连接请求", fmt.Sprintf("Device: %d, Local: %d, Phone: %d", req.DeviceId, req.LocalPort, req.PhonePort))

	token, err := globalApi.UserDeviceCtl.GetDeviceCtlToken(userDeviceCtl.GetDeviceCtlTokenReq{TbYunJiUserDeviceId: &req.TbYunJiUserDeviceId})
	if err != nil {
		return nil, fmt.Errorf("Failed to get device token: " + err.Msg)
	}

	portToken, err := globalApi.RtcCtl.GetDeviceRtcPortToken(rtcCtl.GetDeviceRtcPortTokenReq{Token: &token})
	if err != nil {
		return nil, fmt.Errorf("Failed to get RTC token: " + err.Msg)
	}

	portMapInfo := &coreClass.PortMapInfo{
		DeviceId:      req.DeviceId,
		PhonePort:     req.PhonePort,
		LocalPort:     req.LocalPort,
		PortMapRtc:    nil,
		Portforwarder: nil,
	}

	mgr.Core.PortMapConnect(portToken.Url, portToken.Token, portMapInfo)

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

// DisconnectDevice Handler
func DisconnectDevice(ctx context.Context, req *model.DisconnectRequest) (*model.Response, error) {
	mgr := manager.GetInstance()

	if info, ok := mgr.Core.LocalPortGetPortMapInfo(req.LocalPort); ok {
		if info.Portforwarder != nil {
			info.Portforwarder.Close()
		}
		mgr.Core.LocalPortDelPortMapInfo(req.LocalPort)
	}

	if mgr.HasMapping(req.LocalPort) {
		mgr.RemoveMapping(req.LocalPort)
	}

	logs.Info("断开连接", fmt.Sprintf("LocalPort: %d", req.LocalPort))

	return &model.Response{
		Success: true,
		Message: fmt.Sprintf("Disconnected port %d", req.LocalPort),
	}, nil
}

// Helper type for arbitrary JSON response
type ginHWrapper map[string]interface{}

// UpdateConfig Request
type UpdateConfigRequest struct {
	WsUrl string `json:"wsUrl" binding:"required"`
}

// UpdateConfig Handler
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

	// Force reconnection if URL changed
	loginLock.Lock()
	if s != nil {
		logs.Info("Configuration changed, closing existing session to force reconnect")
		s = nil
		globalApi = nil
		currentKey = ""
	}
	loginLock.Unlock()

	return &model.Response{
		Success: true,
		Message: "Configuration updated. Reconnection will occur on next request.",
	}, nil
}

type GetConfigRequest struct{}

// GetConfig Handler
func GetConfig(ctx context.Context, req *GetConfigRequest) (*ginHWrapper, error) {
	return &ginHWrapper{
		"wsUrl": config.GetWsUrl(),
	}, nil
}
