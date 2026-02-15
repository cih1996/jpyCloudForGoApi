package middleAgentRtc

import (
	"port-mapping-demo/third_party/JpyApiAgent/table/coreClass/publicStruct"
	"sync/atomic"

	"github.com/ghp3000/logs"
	"github.com/ghp3000/netclient/bufferPool"
	"github.com/ghp3000/public"
)

// AsyncSendToMiddleWare 异步方式向中间件发送指令(本中间件内)此处发送的指令类型对应文档：业务公共连接
//
// msgType 指令类型 对应文档参数：f
// msgBody 指令参数 对应文档参数：data
// returnMsg 是否返回指令执行结果 对应文档参数：req
// logMsg 日志信息  对应文档参数：功能名称
func (s *TypeInfo) AsyncSendToMiddleWare(msgType uint16, msgBody interface{}, req bool, logMsg string) {
	msg := public.NewMessage(bufferPool.TypeMsgpack, msgType, atomic.AddUint32(&s.Seq, 1))
	msg.Req = req
	if msgBody != nil {
		if err := msg.Marshal(msgBody); err != nil {
			logs.Error("中间件[%d]%s数据构造失败,%s", s.MiddlewareId, logMsg, err.Error())
			return
		}
	}
	if s.Rtc != nil {
		if err := s.Rtc.SendMsgpack(msg, 0); err != nil {
			logs.Error("中间件[%d]%s发送指令消息失败,%s", s.MiddlewareId, logMsg, err.Error())
			return
		}
	}
	return
}

// AsyncBatchSendToDevice 异步方式通过中间件批量发送手机指令(本中间件内，通过中间件向设备发送指令)此处发送的指令类型对应文档：单手机控制
//
// msgType 指令类型 对应文档参数：f
// msgBody 指令参数 对应文档参数：data
// dIds 设备deviceId数组 如果为空则发送给所有设备，如果指定的deviceId不在本中间件内则忽略
// returnMsg 是否允许发送返回消息
// returnMsg 是否返回指令执行结果 对应文档参数：returnMsg
// logMsg 日志信息  对应文档参数：功能名称
func (s *TypeInfo) AsyncBatchSendToDevice(msgType uint16, msgBody interface{}, dIds []uint64, returnMsg bool, logMsg string) {
	msg := public.NewMessage(bufferPool.TypeMsgpack, msgType, atomic.AddUint32(&s.Seq, 1))
	msg.Req = returnMsg
	if msgBody != nil {
		if err := msg.Marshal(msgBody); err != nil {
			logs.Error("[异步发送]中间件[%d]%s数据构造失败,%s", s.MiddlewareId, logMsg, err.Error())
			return
		}
	}
	pkt := bufferPool.Get()
	defer pkt.Put()
	var i int
	var deviceIds []uint64
	// 如果没有指定设备ID，则发送给所有设备
	if len(dIds) == 0 {
		for n := 0; n < len(s.Devices); n++ {
			dIds = append(dIds, s.Devices[n].DeviceId)
		}
	}
	for _, dId := range dIds {
		if s.DeviceIdGetMiddleId(dId) == s.MiddlewareId {
			deviceIds = append(deviceIds, dId)
			if len(deviceIds) == 20 {
				err := pkt.Write3(bufferPool.TypeMsgpack, deviceIds, &msg)
				if err == nil && s.Rtc != nil {
					if err = s.Rtc.SendPacket(pkt); err != nil {
						logs.Error("[异步发送]中间件[%d]%s失败,发送序号:[%d],%s", s.MiddlewareId, logMsg, i, err.Error())
						return
					}
				}
				deviceIds = deviceIds[:0]
				i++
				logs.Debug("[异步发送]中间件[%d]%s发送成功:发送序号[%d]", s.MiddlewareId, logMsg, i)
			}
		}
	}
	if len(deviceIds) > 0 {
		err := pkt.Write3(bufferPool.TypeMsgpack, deviceIds, &msg)
		if err == nil && s.Rtc != nil {
			if err = s.Rtc.SendPacket(pkt); err != nil {
				logs.Error("[异步发送]中间件[%d]%s失败,发送序号:[%d],%s", s.MiddlewareId, logMsg, i, err.Error())
				return
			}
		}

		i++
		logs.Debug("[异步发送]中间件[%d]%s发送成功:发送序号[%d]", s.MiddlewareId, logMsg, i)
	}
	return
}

// AsyncGetAllList 异步方式获取所有设备列表(本中间件内)注意:本方法需要在异步回调函数中获取返回值
// 异步方式获取所有设备列表(本中间件内)
func (s *TypeInfo) AsyncGetAllList() {
	s.AsyncSendToMiddleWare(public.FuncDevices, nil, true, "获取所有设备列表")
}

// AsyncGetOnlineList 异步方式获取在线列表(本中间件内)注意:本方法需要在异步回调函数中获取返回值
func (s *TypeInfo) AsyncGetOnlineList() {
	s.AsyncSendToMiddleWare(public.FuncOnlineList, nil, true, "获取在线列表")
}

// AsyncGetAllDeviceDetails 批量获取指定设备组设备详情(本中间件内) 本方法不可同步使用，返回信息是多条信息
// 参数
// deviceIds 设备deviceId数组 如果为空则发送给所有设备，如果指定的deviceId不在本中间件内则忽略
func (s *TypeInfo) AsyncGetAllDeviceDetails(deviceIds []uint64) {
	s.AsyncBatchSendToDevice(public.FuncDevice, nil, deviceIds, true, "批量获取所有设备详情")
}

// AsyncSetAllDevicesScreenOn 批量设置所有设备屏幕保持常亮(本中间件内)，本方法不可同步使用，返回信息是多条信息
// 参数
// deviceIds 设备deviceId数组 如果为空则发送给所有设备，如果指定的deviceId不在本中间件内则忽略
func (s *TypeInfo) AsyncSetAllDevicesScreenOn(deviceIds []uint64) {
	s.AsyncBatchSendToDevice(public.FuncScreenOn, &publicStruct.ScreenOnOff{
		State:    "on",
		TimeLong: 31536000,
	}, deviceIds, true, "批量设置屏幕保持常亮")
}

// AsyncSetAllDevicesScreenOff 批量设置所有设备屏幕保持常亮(本中间件内)，本方法不可同步使用，返回信息是多条信息
// 参数
// deviceIds 设备deviceId数组 如果为空则发送给所有设备，如果指定的deviceId不在本中间件内则忽略
func (s *TypeInfo) AsyncSetAllDevicesScreenOff(deviceIds []uint64) {
	s.AsyncBatchSendToDevice(public.FuncScreenOff, &publicStruct.ScreenOnOff{
		State:    "off",
		TimeLong: 31536000,
	}, deviceIds, true, "批量设置屏幕保持常亮")
}

// AsyncTouch 异步方式批量触摸(本中间件内,自动删除越权的deviceId)，本方法不可同步使用，无返回消息
// 参数
// msgBody 指令参数 对应文档参数：data
// deviceIds 设备deviceId数组 如果为空则发送给所有设备，如果指定的deviceId不在本中间件内则忽略
func (s *TypeInfo) AsyncTouch(msgBody *publicStruct.TouchRequest, deviceIds []uint64) {
	s.AsyncBatchSendToDevice(public.FuncTouch, msgBody, deviceIds, false, "批量触摸")
}

// AsyncScroll 异步方式批量鼠标滚轮(本中间件内,自动删除越权的deviceId)，本方法不可同步使用，无返回消息
// 参数
// msgBody 指令参数 对应文档参数：data
// deviceIds 设备deviceId数组 如果为空则发送给所有设备，如果指定的deviceId不在本中间件内则忽略
func (s *TypeInfo) AsyncScroll(msgBody *publicStruct.ScrollRequest, deviceIds []uint64) {
	s.AsyncBatchSendToDevice(public.FuncScroll, msgBody, deviceIds, false, "批量鼠标滚轮")
}

// AsyncKeyBoard 异步方式批量键盘输入(本中间件内,自动删除越权的deviceId)，本方法不可同步使用，无返回消息
// 参数
// msgBody 指令参数 对应文档参数：data
// deviceIds 设备deviceId数组 如果为空则发送给所有设备，如果指定的deviceId不在本中间件内则忽略
func (s *TypeInfo) AsyncKeyBoard(msgBody *publicStruct.KeyBoardRequest, deviceIds []uint64) {
	s.AsyncBatchSendToDevice(public.FuncKey, msgBody, deviceIds, false, "批量键盘输入")
}

// AsyncRunAppFromPackageName 异步方式批量启动应用(本中间件内,自动删除越权的deviceId)，本方法不可同步使用，无返回消息
// 参数
// msgBody 指令参数 对应文档参数：data
// deviceIds 设备deviceId数组 如果为空则发送给所有设备，如果指定的deviceId不在本中间件内则忽略
func (s *TypeInfo) AsyncRunAppFromPackageName(msgBody *publicStruct.RunApp, deviceIds []uint64) {
	s.AsyncBatchSendToDevice(public.FuncRunApp, msgBody, deviceIds, false, "批量启动应用")
}

// AsyncEnterText 异步方式批量输入文本(本中间件内,自动删除越权的deviceId)，本方法不可同步使用，无返回消息
// 参数
// deviceIds 设备deviceId数组 如果为空则发送给所有设备，如果指定的deviceId不在本中间件内则忽略
// text: 文本
func (s *TypeInfo) AsyncEnterText(deviceIds []uint64, text string) {
	s.AsyncBatchSendToDevice(public.FuncEnterText, &publicStruct.EnterText{Text: text}, deviceIds, false, "批量输入文本")
}
