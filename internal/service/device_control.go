package service

import (
	"context"
	"fmt"
	"time"

	"adminApi/rtcCtl"
	"adminApi/userDeviceCtl"
	"port-mapping-demo/coreClass"
	"port-mapping-demo/internal/manager"

	"github.com/ghp3000/logs"
	"github.com/ghp3000/public"
	"github.com/vmihailenco/msgpack/v5"
)

// DeviceCommandInfo 包含执行命令所需的设备信息
type DeviceCommandInfo struct {
	DeviceId            uint64 `json:"deviceId"`
	TbProxyId           uint64 `json:"tbProxyId"`
	TbYunJiUserDeviceId uint64 `json:"tbYunJiUserDeviceId"`
}

// SendGenericCommandToDevice 发送通用命令
// 支持自定义 F 码、Data、Req 和 同步/异步
func SendGenericCommandToDevice(key string, deviceIds []DeviceCommandInfo, f uint16, data interface{}, req bool, isSync bool, timeout time.Duration) (interface{}, error) {
	// 1. 确保已登录
	if err := EnsureLogin(key); err != nil {
		return nil, fmt.Errorf("ensure login failed: %v", err)
	}

	// 2. 获取 Core 实例
	core := manager.GetInstance().Core

	// 3. 按中间件分组设备
	proxyGroups := make(map[uint64][]uint64)
	for _, d := range deviceIds {
		proxyGroups[d.TbProxyId] = append(proxyGroups[d.TbProxyId], d.DeviceId)
	}

	var allResults []interface{}

	// 4. 遍历每个中间件，确保连接并发送命令
	for proxyId, targetDevIds := range proxyGroups {
		// 使用 ProxyId 作为连接 Key
		_, connected := core.MidGetConn(proxyId)
		if !connected {
			logs.Info("Proxy %d not connected, initiating connection...", proxyId)

			// 4.1 获取中间件 RTC Token
			pId := int64(proxyId)
			rtcRes, errApi := globalApi.RtcCtl.GetRtcToken(rtcCtl.GetRtcTokenReq{
				TbProxyId: &pId,
			})
			if errApi != nil {
				return nil, fmt.Errorf("proxy %d get rtc token failed: %s", proxyId, errApi.Msg)
			}

			// 4.2 构造 RtcToken
			// 注意：Host 字段被用作存储连接的 Key，这里设为 ProxyId
			rtcToken := coreClass.RtcToken{
				UserId:   0,
				DeviceId: 0, // 连接是中间件级别的，不绑定特定设备
				HostUrl:  rtcRes.Url,
				Token:    rtcRes.Token,
				GuestUrl: rtcRes.Url,
				Host:     fmt.Sprintf("%d", proxyId),
			}

			// 4.3 连接中间件
			core.MiddleRtcConnect(rtcToken)

			// 4.4 等待连接建立
			connected = false
			for i := 0; i < 20; i++ {
				time.Sleep(200 * time.Millisecond)
				if _, ok := core.MidGetConn(proxyId); ok {
					connected = true
					logs.Info("Proxy %d connected successfully", proxyId)
					break
				}
			}
			if !connected {
				return nil, fmt.Errorf("timeout waiting for proxy %d connection", proxyId)
			}
		} else {
			logs.Info("Proxy %d already connected, reusing connection", proxyId)
		}

		// 5. 构造并发送命令
		cmd := coreClass.GenericCommand{
			F:    f,
			Data: data,
			Req:  req,
		}

		// 调用更新后的 MiddleRtcSendGenericCommand，传入 proxyId
		res, err := core.MiddleRtcSendGenericCommand(proxyId, targetDevIds, cmd, isSync, timeout)
		if err != nil {
			logs.Error("Send command to proxy %d failed: %v", proxyId, err)
			// 如果是同步模式且发生错误，目前策略是返回错误 (或者可以收集错误)
			if isSync {
				return nil, err
			}
		}

		if isSync && res != nil {
			if resSlice, ok := res.([]interface{}); ok {
				allResults = append(allResults, resSlice...)
			} else {
				allResults = append(allResults, res)
			}
		}
	}

	if isSync {
		if len(allResults) == 1 {
			return allResults[0], nil
		}
		return allResults, nil
	}

	return nil, nil
}

// CloseMiddleConnection explicitly closes the middleware connection for a device
func CloseMiddleConnection(deviceId uint64) {
	core := manager.GetInstance().Core
	if midRtc, ok := core.MidGetConn(deviceId); ok {
		if midRtc.Conn != nil {
			logs.Info("Closing connection for device %d", deviceId)
			midRtc.Conn.Close()
		}
		core.MidDelConn(deviceId)
	}
}

// MiddleCommandRequest 请求结构
type MiddleCommandRequest struct {
	Key      string `json:"key"` // 如果不传则尝试使用 currentKey
	DeviceId uint64 `json:"deviceId"`
	Data     struct {
		F    uint16      `json:"f"`
		Data interface{} `json:"data"`
		Req  bool        `json:"req"`
		Seq  uint32      `json:"seq"`
	} `json:"data"`
}

// MiddleExecuteCommand 执行中间件命令
func MiddleExecuteCommand(ctx context.Context, req *MiddleCommandRequest) (*interface{}, error) {
	// Determine key
	key := req.Key
	if key == "" {
		loginLock.Lock()
		key = currentKey
		loginLock.Unlock()
	}
	if key == "" {
		return nil, fmt.Errorf("authentication key is required")
	}

	// 1. Find Device Info (TbProxyId)
	deviceInfo, err := findDeviceInfo(key, req.DeviceId)
	if err != nil {
		return nil, err
	}

	// 2. Send Command
	// isSync=true for API/WS to return result in response
	res, err := SendGenericCommandToDevice(key, []DeviceCommandInfo{*deviceInfo}, req.Data.F, req.Data.Data, req.Data.Req, true, 20*time.Second)
	if err != nil {
		return nil, err
	}

	// Process response to decode Msgpack data for JSON output
	processedRes := processResponse(res)
	return &processedRes, nil
}

// processResponse processes the result to ensure Data is correctly decoded for JSON
func processResponse(res interface{}) interface{} {
	if res == nil {
		return nil
	}

	// If it's a slice, process each element
	if resSlice, ok := res.([]interface{}); ok {
		newSlice := make([]interface{}, len(resSlice))
		for i, v := range resSlice {
			newSlice[i] = processResponse(v)
		}
		return newSlice
	}

	// If it's a *public.Message, unmarshal DataMsgpack
	if msg, ok := res.(*public.Message); ok {
		// Create a map to hold the full JSON response
		resMap := make(map[string]interface{})

		resMap["f"] = msg.F
		resMap["req"] = msg.Req
		resMap["seq"] = msg.Seq
		if msg.Code != 0 {
			resMap["code"] = msg.Code
		}
		if msg.Msg != "" {
			resMap["msg"] = msg.Msg
		}
		if msg.T != 0 {
			resMap["t"] = msg.T
		}

		// Decode DataMsgpack if present
		if len(msg.DataMsgpack) > 0 {
			var data interface{}
			if err := msgpack.Unmarshal(msg.DataMsgpack, &data); err == nil {
				resMap["data"] = data
			} else {
				logs.Warn("Failed to unmarshal DataMsgpack: %v", err)
			}
		}
		return resMap
	}

	return res
}

// 寻找对应key的设备ID
func findDeviceInfo(key string, deviceId uint64) (*DeviceCommandInfo, error) {
	if err := EnsureLogin(key); err != nil {
		return nil, err
	}

	// Fetch all devices (inefficient but works for now)
	res, err := globalApi.UserDeviceCtl.GetUserDeviceList(&userDeviceCtl.GetUserDeviceListReq{
		PageNum:  1,
		PageSize: 999999, // Fetch all to find specific one
	})
	if err != nil {
		return nil, fmt.Errorf("get device list failed: %v", err)
	}

	for _, d := range res.Records {
		if uint64(d.DeviceInfo.DeviceId) == deviceId {
			return &DeviceCommandInfo{
				DeviceId:            uint64(d.DeviceInfo.DeviceId),
				TbProxyId:           uint64(d.DeviceInfo.TbProxyId),
				TbYunJiUserDeviceId: uint64(d.TbYunJiUserDeviceId),
			}, nil
		}
	}
	return nil, fmt.Errorf("device %d not found", deviceId)
}
