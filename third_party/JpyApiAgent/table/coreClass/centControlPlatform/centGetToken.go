package centControlPlatform

import (
	"adminApi/rtcCtl"
	"adminApi/userDeviceCtl"
	"encoding/json"

	"cnb.cool/accbot/goTool/ErrPkg"
	"cnb.cool/accbot/goTool/TcpTypePkg"
	"github.com/ghp3000/logs"
)

// getRtcTokenRawResponse 原始响应结构（包含 code 和 data）
type getRtcTokenRawResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Url   string `json:"url"`
		Token string `json:"token"`
	} `json:"data"`
}

// CenterGetMiddlewareRtcToken 获取中间件控制打洞Token
func (c *Core) CenterGetMiddlewareRtcToken(middlewareId uint64) (*rtcCtl.GetRtcTokenRes, *ErrPkg.Err) {
	midId := int64(middlewareId)
	req := rtcCtl.GetRtcTokenReq{TbProxyId: &midId}

	// 直接调用底层 session 发送请求，获取原始响应
	// 这样即使 code != 200，我们也能拿到 data
	payload := map[string]any{
		"app":  "rtcCtl",
		"fun":  "getRtcToken",
		"data": req,
	}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, ErrPkg.NewErrE("序列化失败", err)
	}

	// 防御：session 断开时不能调用，否则空指针 panic
	if c.server.session == nil {
		return nil, ErrPkg.NewErrE("服务器连接已断开，无法获取RTC Token", nil)
	}

	// 发送请求并获取原始响应
	response, sendErr := c.server.session.SendCallBytes(TcpTypePkg.TcpType_Text_AutoGzip, jsonBytes, 30*1000)
	if sendErr != nil {
		logs.Error("中间件RTC Token请求失败: %s", sendErr.Msg)
		return nil, sendErr
	}

	// 解析原始响应
	var rawResp getRtcTokenRawResponse
	if jsonErr := json.Unmarshal(response, &rawResp); jsonErr != nil {
		return nil, ErrPkg.NewErrE("响应解析失败", jsonErr)
	}

	// 检查是否有有效的 url 和 token（即使 code != 200）
	if rawResp.Data.Url != "" && rawResp.Data.Token != "" {
		if rawResp.Code != 200 {
			logs.Info("中间件[%d]获取Token成功（忽略code=%d, msg=%s）：url=%s", middlewareId, rawResp.Code, rawResp.Msg, rawResp.Data.Url)
		}
		return &rtcCtl.GetRtcTokenRes{
			Url:   rawResp.Data.Url,
			Token: rawResp.Data.Token,
		}, nil
	}

	// 没有有效数据，返回错误
	if rawResp.Code != 200 {
		logs.Error("中间件RTC Token获取错误: code=%d, msg=%s", rawResp.Code, rawResp.Msg)
		return nil, ErrPkg.NewErr(rawResp.Code, rawResp.Msg)
	}

	return nil, ErrPkg.NewErrE("响应数据为空", nil)
}

// CenterGetPortMapSocket5RtcToken 获取设备端口映射打洞Token
func (c *Core) CenterGetPortMapSocket5RtcToken(deviceId uint64) (*rtcCtl.GetDeviceRtcPortTokenRes, *ErrPkg.Err) {
	device, ok := c.getDevice(deviceId)
	if !ok {
		return nil, ErrPkg.NewErrE("设备不存在", nil)
	}
	if c.server.api == nil {
		return nil, ErrPkg.NewErrE("服务器连接已断开，无法获取端口映射Token", nil)
	}
	token, err2 := c.server.api.UserDeviceCtl.GetDeviceCtlToken(userDeviceCtl.GetDeviceCtlTokenReq{TbYunJiUserDeviceId: &device.TBYunJiUserDeviceId})
	if err2 != nil {
		logs.Info("设备临时控制码获取错误", err2)
		return nil, err2
	} else {
		logs.Info("设备临时控制码获取成功", token)
		return c.server.api.RtcCtl.GetDeviceRtcPortToken(rtcCtl.GetDeviceRtcPortTokenReq{Token: &token})
	}
}

// CenterGetDeviceH264AudioRtcToken 获取设备串流控制打洞Token
func (c *Core) CenterGetDeviceH264AudioRtcToken(deviceId uint64) (*rtcCtl.GetDeviceRtcTokenRes, *ErrPkg.Err) {
	device, ok := c.getDevice(deviceId)
	if !ok {
		return nil, ErrPkg.NewErrE("设备不存在", nil)
	}
	if c.server.api == nil {
		return nil, ErrPkg.NewErrE("服务器连接已断开，无法获取串流Token", nil)
	}
	token, err2 := c.server.api.UserDeviceCtl.GetDeviceCtlToken(userDeviceCtl.GetDeviceCtlTokenReq{TbYunJiUserDeviceId: &device.TBYunJiUserDeviceId})
	if err2 != nil {
		logs.Info("设备临时控制码获取错误", err2)
		return nil, err2
	}
	logs.Info("设备临时控制码获取成功", token)
	return c.server.api.RtcCtl.GetDeviceRtcToken(rtcCtl.GetDeviceRtcTokenReq{Token: &token})
}
