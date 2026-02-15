package centControlPlatform

import (
	"port-mapping-demo/third_party/JpyApiAgent/table/coreClass/deviceRtc"
	"port-mapping-demo/third_party/JpyApiAgent/table/coreClass/middleAgentRtc"
	"port-mapping-demo/third_party/JpyApiAgent/table/coreClass/portMapSocket5Rtc"
	"port-mapping-demo/third_party/JpyApiAgent/table/coreClass/publicStruct"
	"errors"
	"fmt"

	"cnb.cool/accbot/goTool/ErrPkg"
	"github.com/ghp3000/logs"
	"github.com/ghp3000/netclient/NetClient"
)

// CreatMiddlewareRtc 创建中间件Rtc连接对象
func (c *Core) CreatMiddlewareRtc(MiddlewareId uint64, parOpen NetClient.ConnectEvent, parClose NetClient.ConnectEvent, parCallBack NetClient.Callback) (*middleAgentRtc.TypeInfo, error) {
	//创建中间件RTC对象
	var pkt publicStruct.MiddleAgentTypeInfo
	pkt.MiddlewareId = MiddlewareId
	pkt.FuncOpen = parOpen
	pkt.FuncClose = parClose
	pkt.FuncCallBack = parCallBack
	pkt.GetToken = c.CenterGetMiddlewareRtcToken
	pkt.DelMiddleware = c.deleteMiddleAgent
	allDevices := c.GetAllDevice()

	for _, device := range allDevices {
		if uint64(device.DeviceInfo.TbProxyId) == MiddlewareId {
			pkt.Devices = append(pkt.Devices, device)
		}
	}
	middleWare, err := middleAgentRtc.New(&pkt)
	if err != nil {
		logs.Error("创建中间件[%d]对象失败%v", MiddlewareId, err)
		return nil, errors.New(fmt.Sprintf("创建中间件[%d]对象失败%v", MiddlewareId, err))
	}

	var err1 *ErrPkg.Err
	middleWare.TokenInfo, err1 = middleWare.GetToken(middleWare.MiddlewareId)
	if err1 != nil {
		logs.Error("创建中间件[%d]对象失败%v", MiddlewareId, err1)
		return nil, errors.New(fmt.Sprintf("创建中间件[%d]对象失败%v", MiddlewareId, err1))
	}
	middleWare.Connect()
	//保存中间件对象
	c.saveMiddleAgent(MiddlewareId, middleWare)
	return middleWare, nil
}

// CreatPortMapSocket5Rtc 创建端口映射或Socket5服务Rtc连接对象
func (c *Core) CreatPortMapSocket5Rtc(deviceId uint64, devicePort int, localPort int, mode int) (*portMapSocket5Rtc.TypeInfo, error) {
	// 创建端口映射打洞对象
	var pkt publicStruct.PortMapTypeInfo

	pkt.DeviceId = deviceId
	pkt.GetToken = c.CenterGetPortMapSocket5RtcToken
	pkt.DelPortMapId = c.deletePortMap
	pkt.Mode = mode
	pkt.PhonePort = devicePort
	pkt.LocalPort = localPort
	pkt.Code = 0
	pkt.Msg = ""

	portMapRtc, err := portMapSocket5Rtc.New(&pkt)
	if err != nil {
		logs.Error("创建端口映射[%d]对象失败%v", deviceId, err)
		return nil, errors.New(fmt.Sprintf("创建端口映射[%d]对象失败%v", deviceId, err))
	}

	var err1 *ErrPkg.Err
	portMapRtc.TokenInfo, err1 = portMapRtc.GetToken(portMapRtc.DeviceId)
	if err1 != nil {
		logs.Error("创建端口映射[%d]对象失败%v", deviceId, err1)
		return nil, errors.New(fmt.Sprintf("创建端口映射[%d]对象失败%v", deviceId, err1))
	}

	portMapRtc.Connect()
	//保存端口映射对象
	c.savePortMap(deviceId, portMapRtc)
	return portMapRtc, nil
}

// CreatDeviceH264AudioRtc 创建设备串流控制Rtc连接对象
func (c *Core) CreatDeviceH264AudioRtc(deviceId uint64, parOpen NetClient.ConnectEvent, parClose NetClient.ConnectEvent, parCallBack NetClient.Callback) (*deviceRtc.TypeInfo, error) {
	var pkt publicStruct.Device
	pkt.DeviceId = deviceId
	pkt.GetToken = c.CenterGetDeviceH264AudioRtcToken
	pkt.FuncOpen = parOpen
	pkt.FuncClose = parClose
	pkt.FuncCallBack = parCallBack
	device, err := deviceRtc.New(&pkt)
	if err != nil {
		logs.Error("创建设备[%d]串流控制对象失败%v", deviceId, err)
		return nil, errors.New(fmt.Sprintf("创建设备[%d]串流控制对象失败%v", deviceId, err))
	}
	var err1 *ErrPkg.Err
	device.TokenInfo, err1 = device.GetToken(device.DeviceId)
	if err1 != nil {
		logs.Error("创建设备[%d]串流控制对象失败%v", deviceId, err1)
		return nil, errors.New(fmt.Sprintf("创建设备[%d]串流控制对象失败%v", deviceId, err1))
	}
	device.Connect()
	return device, nil
}
