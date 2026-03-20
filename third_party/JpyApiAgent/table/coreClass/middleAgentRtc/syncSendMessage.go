package middleAgentRtc

import (
	"port-mapping-demo/third_party/JpyApiAgent/table/coreClass/publicStruct"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ghp3000/logs"
	"github.com/ghp3000/netclient/bufferPool"
	"github.com/ghp3000/public"
)

func (s *TypeInfo) deviceExists(deviceId uint64) bool {
	for _, device := range s.Devices {
		if device.DeviceId == deviceId {
			return true
		}
	}
	return false
}

// SyncCallToDevice 同步方式调用设备(对中间件下指定设备发送消息)
//
// 对中间件下指定设备发送消息,并等待回复
//
// 参数:
//
//	conn: 中间件RTC连接
//	msg: 要发送的消息
//	deviceId: 目标设备ID 如果为0,则对中间件发送消息
//	timeout: 超时时间
//
// 返回值:
//
//	*public.Message: 设备回复的消息
func (s *TypeInfo) syncCall(msg *public.Message, deviceId uint64, timeout time.Duration) *public.Message {
	if s.Rtc == nil {
		return msg.SetCode(public.NotOnline)
	}
	if err := s.Rtc.SendMsgpack(msg, deviceId); err != nil {
		return msg.SetCode(public.NotOnline)
	}
	ch := make(chan *public.Message)
	s.Wait.Store(msg.Seq, ch)
	ticker := time.NewTicker(timeout)
	defer func() {
		s.Wait.Delete(msg.Seq)
		close(ch)
		ticker.Stop()
	}()
	select {
	case <-ticker.C:
		return msg.SetCode(public.Timeout)
	case ret := <-ch:
		return ret
	}
}

// SyncSendToDevice 同步方式发送消息到设备(本中间件内)
//
// 对中间件下指定设备发送消息,并等待回复
//
// 参数:
//
//	msgType: 消息类型
//	msgBody: 消息体
//	deviceId: 目标设备ID 如果为0,则对中间件发送消息
//	logMsg: 日志消息
//
// 返回值:
//
//	string: 设备回复的消息
//	error: 错误信息
func (s *TypeInfo) SyncSendToDevice(msgType uint16, msgBody interface{}, deviceId uint64, logMsg string) (jsonStr string, err error) {
	return s.SyncSendToDeviceWithTimeout(msgType, msgBody, deviceId, logMsg, time.Second*30)
}

// SyncSendToDeviceWithTimeout 同步方式发送消息到设备，支持自定义超时
func (s *TypeInfo) SyncSendToDeviceWithTimeout(msgType uint16, msgBody interface{}, deviceId uint64, logMsg string, timeout time.Duration) (jsonStr string, err error) {
	if !s.deviceExists(deviceId) && deviceId != 0 {
		logs.Error("[同步消息]中间件[%d]%s设备[%d]不在本中间件管辖范围", s.MiddlewareId, logMsg, deviceId)
		return "", errors.New(fmt.Sprintf("[同步消息]中间件[%d]%s设备[%d]不在本中间件管辖范围", s.MiddlewareId, logMsg, deviceId))
	}
	msg := public.NewMessage(bufferPool.TypeMsgpack, msgType, atomic.AddUint32(&s.Seq, 1))
	logs.Info("[同步消息]中间件[%d]接口[%s]检查=seq:%d", s.MiddlewareId, logMsg, s.Seq)

	if msgBody != nil {
		if err := msg.Marshal(msgBody); err != nil {
			logs.Error("[同步消息]中间件[%d]%s数据构造失败,%s", s.MiddlewareId, logMsg, err.Error())
			return "", errors.New(fmt.Sprintf("[同步消息]中间件[%d]%s数据构造失败,%s", s.MiddlewareId, logMsg, err.Error()))
		}
	}
	retMsg := s.syncCall(msg, deviceId, timeout)
	if err = retMsg.Error(); err != nil {
		logs.Error("[同步消息]中间件[%d]%s设备[%d]无返回消息,%s", s.MiddlewareId, logMsg, deviceId, err.Error())
		return "", errors.New(fmt.Sprintf("[同步消息]中间件[%d]%s设备[%d]无返回消息,%s", s.MiddlewareId, logMsg, deviceId, err.Error()))
	}

	return retMsg.ToJsonString(), err
}

// SyncGetAllList 同步方式获取所有设备列表(本中间件内)
//
// 获取中间件下所有设备列表
//
// 返回值:
//
//	string: 所有设备列表的JSON字符串
//	error: 错误信息
func (s *TypeInfo) SyncGetAllList() (string, error) {
	return s.SyncSendToDevice(public.FuncDevices, nil, 0, "[同步][获取所有设备列表]")
}

// SyncGetOnlineList 同步方式获取所有设备在线列表(本中间件内)
//
// 获取中间件下所有设备在线列表
//
// 返回值:
//
//	string: 所有设备在线列表的JSON字符串
//	error: 错误信息
func (s *TypeInfo) SyncGetOnlineList() (rec string, err error) {
	return s.SyncSendToDevice(public.FuncOnlineList, nil, 0, "[同步][获取所有设备在线列表]")
}

// SyncModeSwitch 同步方式切换设备模式(本中间件内)
// 切换中间件下指定设备的模式(otg/usb)
//
// 参数:
//
//	deviceId: 设备ID
//	mode: 模式 0=otg, 1=usb
//
// 返回值:
//
//	string: 设备回复的消息
//	error: 错误信息
func (s *TypeInfo) SyncModeSwitch(deviceId uint64, mode int) (rec string, err error) {
	return s.SyncSendToDevice(public.FuncModeSwitch, &publicStruct.ModeSwitch{Seat: publicStruct.GetSeatFromDeviceId(deviceId), Mode: mode}, 0, "[同步][切换设备模式]")
}

// SyncGetPluginList 同步方式获取所有插件列表(本中间件内)
//
// 主动请求每个盘位对应的插件和插件版本号
//
// 返回值:
//
//	string: 所有插件列表的JSON字符串
//	error: 错误信息
func (s *TypeInfo) SyncGetPluginList() (rec string, err error) {
	return s.SyncSendToDevice(public.FuncDevice2Version, nil, 0, "[同步][获取插件列表]")
}

// SyncDevicePowerControl 同步方式控制设备电源(本中间件内)
//
// 仅适用于新机型,对老机箱发送此指令会返回不支持的函数错误.
//
// 参数:
//
//	deviceId: 设备ID
//	power: 0=断电,1=供电,2=强制重启(断电后再供电)
//
// 返回值:
//
//	string: 回复的正确消息
//	error: 错误信息
func (s *TypeInfo) SyncDevicePowerControl(deviceId uint64, power int) (rec string, err error) {
	return s.SyncSendToDevice(public.FuncDevicePowerControl, &publicStruct.DevicePowerControl{Seat: publicStruct.GetSeatFromDeviceId(deviceId), Mode: power}, 0, "[同步][控制设备电源]")
}

// SyncEnterFlashingMode 同步方式进入刷机模式(本中间件内)
//
// 仅适用于新机型,对老机箱发送此指令会返回不支持的函数错误.
//
// 参数:
//
//	deviceId: 设备ID
//	mode: 1=模式1[开关+音量减5秒];2=模式2[开关+音量减9秒];3=模式3[开关+音量加5秒]
//
// 返回值:
//
//	string: 回复的正确消息
//	error: 错误信息
func (s *TypeInfo) SyncEnterFlashingMode(deviceId uint64, mode int) (rec string, err error) {
	return s.SyncSendToDevice(public.FuncEnterFlashingMode, &publicStruct.EnterFlashingMode{Seat: publicStruct.GetSeatFromDeviceId(deviceId), Mode: mode}, 0, "[同步][进入闪烁模式]")
}

// SyncSetDeviceToFindMode 同步方式设置设备为查找模式(本中间件内)
//
// 仅适用于新机型,对老机箱发送此指令会返回不支持的函数错误.
//
// 参数:
//
//	deviceId: 设备ID
//	mode: 0=关闭,1=开启
//
// 返回值:
//
//	string: 回复的正确消息
//	error: 错误信息
func (s *TypeInfo) SyncSetDeviceToFindMode(deviceId uint64, mode int) (rec string, err error) {
	return s.SyncSendToDevice(public.FuncSetDeviceToFindMode, &publicStruct.SetDeviceToFindMode{Seat: publicStruct.GetSeatFromDeviceId(deviceId), Mode: mode}, 0, "[同步][设置设备查找模式]")
}

// SyncGetMiddlewareWorkMode 同步方式获取中间件工作模式(本中间件内)
//
// 参数:无
//
// 返回值:
//
//	string:  回复的正确消息  {"mode":1}   0=无hid支持android ;1=有hid支持android ;2=无hid支持ios ;3 =有hid支持ios
//	error: 错误信息
func (s *TypeInfo) SyncGetMiddlewareWorkMode() (rec string, err error) {
	return s.SyncSendToDevice(public.FuncGetMiddlewareWorkMode, nil, 0, "[同步][获取中间件工作模式]")

}

// SyncGetDeviceDetails 同步方式获取设备详细信息(本中间件内)
//
// 参数:
//
//	deviceId: 设备ID
//
// 返回值:
//
//	string:  回复的正确消息,数据样板 {"code":0,"data":{"sysVersion":"android13","country":"United States(US)","appVersion":"","orientation":1,"os":"android","timezone":"China Standard Time","sysPer":1,"uuid":"2089f524","seat":115,"osVersion":"Xiaomi_8SE_android_13_20250623_v1.0","androidVersion":"13","vendor":"Redmi 21121119SC","width":1080,"signalMode":"","self":"1.1","model":"21121119SC","location":"{\"latitude\":39.937857,\"longitude\":-101.458819}","lang":"en","brand":"Redmi","height":2400},"f":4,"msg":"操作成功","req":false,"seq":0,"moduleId":0}
//	error: 错误信息
func (s *TypeInfo) SyncGetDeviceDetails(deviceId uint64) (rec string, err error) {
	return s.SyncSendToDevice(public.FuncDevice, nil, deviceId, "[同步][获取设备详细信息]")
}

// SyncShellCommand 同步方式发送Shell命令并取返回值
//
// 参数
//
//	deviceId: 设备ID
//	shell: shell命令
//
// 返回值:
//
//	string:
//	error: 错误信息
func (s *TypeInfo) SyncShellCommand(deviceId uint64, shell string) (rec string, err error) {
	return s.SyncSendToDevice(public.FuncCMDWithResult, &publicStruct.ShellCmd{Shell: shell}, deviceId, "[同步][执行shell命令]")
}

// SyncGetAppList 同步方式获取设备应用列表(本中间件内)
//
// 参数:
//
//	deviceId: 设备ID
//
// 返回值:
//
//	string:  回复的正确消息,数据样板:{"code":0,"data":[{"firstInstallTime":1750015774111,"appname":"作业帮","packageName":"com.baidu.homework","lastUpdateTime":1750015797589},{"firstInstallTime":1698844472534,"appname":"Shazam","packageName":"com.shazam.android","lastUpdateTime":1701675355534},{"firstInstallTime":1701118540148,"appname":"Whoscall","packageName":"gogolook.callgogolook2","lastUpdateTime":1701912941148},{"firstInstallTime":1697409932634,"appname":"Viber","packageName":"com.viber.voip","lastUpdateTime":1701820573634},{"firstInstallTime":1699257173109,"appname":"TrueMoney","packageName":"th.co.truemoney.wallet","lastUpdateTime":1702011418109},{"firstInstallTime":1697932342010,"appname":"KiQS Learning App","packageName":"com.kidskq","lastUpdateTime":1702167559010},{"firstInstallTime":1700911315671,"appname":"sso -push2","packageName":"com.ghiaeirbe.uuiirwhy","lastUpdateTime":1701712338671}],"f":290,"msg":"操作成功","req":false,"seq":15,"moduleId":1}
//	error: 错误信息
func (s *TypeInfo) SyncGetAppList(deviceId uint64) (rec string, err error) {
	return s.SyncSendToDevice(public.FuncGetAppList, nil, deviceId, "[同步][获取设备应用列表]")
}

// SyncRunApp 同步方式启动应用(本中间件内)
//
// 参数:
//
//	deviceId: 设备ID
//	packageName: 应用包名
//
// 返回值:
//
//	string:  回复的正确消息,数据样板:{"code":0,"data":true,"f":291,"msg":"操作成功","req":false,"seq":17,"moduleId":1}
//	error: 错误信息
func (s *TypeInfo) SyncRunApp(deviceId uint64, packageName string) (rec string, err error) {
	return s.SyncSendToDevice(public.FuncRunApp, &publicStruct.RunApp{PackageName: packageName}, deviceId, "[同步][启动应用]")

}

// SyncDownloadAndInstall 同步方式下载并安装应用
//
// 参数:
//
//	deviceId: 设备ID
//
// 返回值:
//
//	string:  返回json文本
//	error: 错误信息
func (s *TypeInfo) SyncDownloadAndInstall(deviceId uint64, install *publicStruct.DownloadAndInstall) (rec string, err error) {
	return s.SyncSendToDeviceWithTimeout(public.FuncFileDownload, install, deviceId, "[同步][下载并安装应用]", time.Second*120)
}

// SyncCheckProgress 同步方式查询下载并安装进度
//
// 参数:
//
//	deviceId: 设备ID
//	Id: 任务ID 来自于SyncRunApp返回值中
//
// 返回值:
//
//	string:  返回json文本
//	error: 错误信息
func (s *TypeInfo) SyncCheckProgress(deviceId uint64, id uint32) (rec string, err error) {
	return s.SyncSendToDevice(public.FuncDownloadAndInstallMessage, &publicStruct.GetDownloadAndInstallMessage{Id: id}, deviceId, "[同步][查询下载并安装的进度]")
}

// SyncScreenOff 同步方式关闭屏幕
//
// 参数:
//
//	deviceId: 设备ID
//
// 返回值:
//
//	string:  返回json文本
//	error: 错误信息
func (s *TypeInfo) SyncScreenOff(deviceId uint64) (rec string, err error) {
	return s.SyncSendToDevice(public.FuncScreenOff, &publicStruct.ScreenOnOff{State: "off", TimeLong: 31536000}, deviceId, "[同步][关闭屏幕]")
}

// SyncScreenOn 同步方式开启屏幕
//
// 参数:
//
//	deviceId: 设备ID
//
// 返回值:
//
//	string:  返回json文本
//	error: 错误信息
func (s *TypeInfo) SyncScreenOn(deviceId uint64) (rec string, err error) {
	return s.SyncSendToDevice(public.FuncScreenOn, &publicStruct.ScreenOnOff{State: "on", TimeLong: 31536000}, deviceId, "[同步][开启屏幕]")
}

// SyncRootApp 同步方式开启Root权限
//
// 参数:
//
//	deviceId: 设备ID
//	pkg: 应用包名
//
// 返回值:
//
//	string:  返回json文本
//	error: 错误信息
func (s *TypeInfo) SyncRootApp(deviceId uint64, pkg string) (rec string, err error) {
	return s.SyncSendToDevice(public.FuncSetRootApp, &publicStruct.RootApp{Pkg: pkg}, deviceId, "[同步][设置AppRoot权限]")
}

// SyncCancelRootApp 同步方式取消Root权限
//
// 参数:
//
//	deviceId: 设备ID
//	pkg: 应用包名
//
// 返回值:
//
//	string:  返回json文本
//	error: 错误信息
func (s *TypeInfo) SyncCancelRootApp(deviceId uint64, pkg string) (rec string, err error) {
	return s.SyncSendToDevice(public.FuncCancelSetRootApp, &publicStruct.RootApp{Pkg: pkg}, deviceId, "[同步][取消AppRoot权限]")
}

// SyncSwitchBack 同步方式切换到后置摄像头
//
// 参数:
//
//	deviceId: 设备ID
//
// 返回值:
//
//	string:  返回json文本
//	error: 错误信息
func (s *TypeInfo) SyncSwitchBack(deviceId uint64) (rec string, err error) {
	var streamReq publicStruct.SetSwitchBack
	streamReq.Type = "setting"
	streamReq.Params.SwitchBack = nil
	return s.SyncSendToDevice(public.FuncChangSwitch, streamReq, deviceId, "[同步][切换到后置摄像头]")
}

// SyncSwitchFront 同步方式切换到前置摄像头
//
// 参数:
//
//	deviceId: 设备ID
//
// 返回值:
//
//	string:  返回json文本
//	error: 错误信息
func (s *TypeInfo) SyncSwitchFront(deviceId uint64) (rec string, err error) {
	var streamReq publicStruct.SetSwitchFront
	streamReq.Type = "setting"
	streamReq.Params.SwitchFront = nil
	return s.SyncSendToDevice(public.FuncChangSwitch, streamReq, deviceId, "[同步][切换到前置摄像头]")
}

// SyncGetClipboard 同步方式获取剪贴板内容
//
// 参数:
//
//	deviceId: 设备ID
//
// 返回值:
//
//	rec string:  返回json文本
//	error: 错误信息
func (s *TypeInfo) SyncGetClipboard(deviceId uint64) (rec string, err error) {
	return s.SyncSendToDevice(public.FuncGetClipboard, nil, deviceId, "[同步][获取剪贴板内容]")
}

// SyncGetDeviceImage 同步方式获取设备图片 字节集
//
// 参数:
//
//	deviceId: 设备ID
//	width: 宽度
//	imageType: 图片类型 0:jpg 1:png
//
// 返回值:
// rec []byte:  返回字节集
// error: 错误信息
func (s *TypeInfo) SyncGetDeviceImage(deviceId uint64, width int, imageType int) ([]byte, error) {
	if !s.deviceExists(deviceId) && deviceId != 0 {
		logs.Error("[同步消息]中间件[%d]%s设备[%d]不在本中间件管辖范围", s.MiddlewareId, "[同步][获取屏幕墙图片]", deviceId)
		return nil, errors.New(fmt.Sprintf("[同步消息]中间件[%d]%s设备[%d]不在本中间件管辖范围", s.MiddlewareId, "[同步][获取屏幕墙图片]", deviceId))
	}
	msgBody := &publicStruct.ImageRequest{
		Width:  0,
		Height: 0,
		Qua:    70,
		Scale:  width,
		X:      0,
		Y:      0,
		Type:   imageType,
	}
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncImg, atomic.AddUint32(&s.Seq, 1))
	if err := msg.Marshal(msgBody); err != nil {
		logs.Error("[同步消息]中间件[%d]%s数据构造失败,%s", s.MiddlewareId, err.Error())
		return nil, errors.New(fmt.Sprintf("[同步消息]中间件[%d]%s数据构造失败,%s", s.MiddlewareId, "[同步][获取屏幕墙图片]", err.Error()))
	}
	var imgData []byte
	receive := s.syncCall(msg, deviceId, time.Second*5)
	if err := receive.Unmarshal(&imgData); err != nil {
		logs.Error("[同步消息]中间件[%d]%s msg解码失败", s.MiddlewareId, "[同步][获取屏幕墙图片]")
		return nil, errors.New(fmt.Sprintf("[同步消息]中间件[%d]%s msg解码失败", s.MiddlewareId, "[同步][获取屏幕墙图片]"))
	}
	return imgData, nil
}

// SyncGetDeviceImageToBase64 同步方式获取设备图片 base64文本
func (s *TypeInfo) SyncGetDeviceImageToBase64(deviceId uint64, width int, imageType int) (string, error) {
	imgData, err := s.SyncGetDeviceImage(deviceId, width, imageType)
	if err != nil {
		return "", err
	}
	img := publicStruct.Base64Image{
		Base64Img: base64.StdEncoding.EncodeToString(imgData),
	}
	jsonData, err := json.Marshal(img)
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}

// SyncDelayedetection 同步方式测延迟
func (s *TypeInfo) SyncDelayedetection() int {
	rec, err := s.SyncSendToDevice(public.FuncTestDelay, &publicStruct.DelayRequest{Timestamp: time.Now().UnixMicro()}, 0, "[同步][测试延迟]")
	if err != nil {
		return -1
	}
	var v publicStruct.DelayRequest
	err = json.Unmarshal([]byte(rec), &v)
	if err != nil {
		return -1
	}
	delay := int((time.Now().UnixMicro() - v.Timestamp) / 1000)
	return delay
}
