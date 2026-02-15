package centControlPlatform

import (
	"adminApi/rtcCtl"
	"adminApi/userDeviceCtl"

	"cnb.cool/accbot/goTool/ErrPkg"
	"github.com/ghp3000/logs"
)

// CenterGetMiddlewareRtcToken 获取中间件控制打洞Token
func (c *Core) CenterGetMiddlewareRtcToken(middlewareId uint64) (*rtcCtl.GetRtcTokenRes, *ErrPkg.Err) {
	midId := int64(middlewareId)
	midToken, err1 := c.server.api.RtcCtl.GetRtcToken(rtcCtl.GetRtcTokenReq{
		TbProxyId: &midId,
	})
	if err1 != nil {
		logs.Error("中间件RTC Token获取错误%s", err1.Msg)
		return nil, err1
	}
	//logs.Info("中间件[%d]获取Token成功：url=%s,token=%s", TbProxyId, midToken.Url, midToken.Token)
	return midToken, nil
}

// CenterGetPortMapSocket5RtcToken 获取设备端口映射打洞Token
func (c *Core) CenterGetPortMapSocket5RtcToken(deviceId uint64) (*rtcCtl.GetDeviceRtcPortTokenRes, *ErrPkg.Err) {
	device, ok := c.getDevice(deviceId)
	if !ok {
		return nil, ErrPkg.NewErrE("设备不存在", nil)
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
	token, err2 := c.server.api.UserDeviceCtl.GetDeviceCtlToken(userDeviceCtl.GetDeviceCtlTokenReq{TbYunJiUserDeviceId: &device.TBYunJiUserDeviceId})
	if err2 != nil {
		logs.Info("设备临时控制码获取错误", err2)
		return nil, err2
	}
	logs.Info("设备临时控制码获取成功", token)
	return c.server.api.RtcCtl.GetDeviceRtcToken(rtcCtl.GetDeviceRtcTokenReq{Token: &token})
}
