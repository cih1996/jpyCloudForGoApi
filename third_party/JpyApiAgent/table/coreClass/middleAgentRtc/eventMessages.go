package middleAgentRtc

import (
	"port-mapping-demo/third_party/JpyApiAgent/table/coreClass/publicStruct"

	"github.com/ghp3000/logs"
	"github.com/ghp3000/netclient/NetClient"
	"github.com/ghp3000/public"
)

// EventOnlineOfflineMessage 处理中间件rtc连接收到的上线离线信息
// conn 中间件rtc连接
// msg 收到的消息
// header 消息头
func (s *TypeInfo) eventOnlineOfflineMessage(_ NetClient.NetClient, msg *public.Message, _ uint64) {
	if msg.Code != 0 {
		logs.Error("中间件[%d]收到在线离线信息失败,msg=%s", s.MiddlewareId, msg.Error())
		return
	}
	// 解析离线信息列表
	var List []publicStruct.OnlineInfo
	var devices []uint64
	if err := msg.Unmarshal(&List); err != nil {
		logs.Error("中间件[%d]收到的在线离线信息数据反序列化失败: %v", s.MiddlewareId, err)
		return
	}
	s.DevicesLock.Lock()
	defer s.DevicesLock.Unlock()

	for n := 0; n < len(s.Devices); n++ {
		for _, onlineOffline := range List {
			deviceId := publicStruct.GetDeviceIdFromMiddleIdAndSeat(s.MiddlewareId, onlineOffline.Seat)
			if s.Devices[n].DeviceId == deviceId {
				s.Devices[n].DeviceStatus.Online = publicStruct.OnlineJudgment(onlineOffline.Online)
				if s.Devices[n].DeviceStatus.Online.Business {
					devices = append(devices, deviceId)
					//					logs.Info("中间件[%d],deviceId[%d],online:%v", s.MiddlewareId, s.Devices[n].DeviceId, s.Devices[n].DeviceStatus.Online)
				}
				break
			}
		}
	}
	// 批量设置所有设备屏幕常亮
	s.AsyncSetAllDevicesScreenOn(devices)
}

// EventScreenRotation 处理中间件rtc连接收到的屏幕旋转事件
// conn 中间件rtc连接
// msg 收到的消息
// header 消息头
func (s *TypeInfo) eventScreenRotation(_ NetClient.NetClient, msg *public.Message, header uint64) {
	deviceId := publicStruct.GetDeviceIdFromMiddleIdAndSeat(s.MiddlewareId, uint8(header))
	var Rotation publicStruct.ScreenRotation
	if err := msg.Unmarshal(&Rotation); err != nil {
		logs.Error("中间件[%d]返回的消息反序列化失败,", s.MiddlewareId, err)
	}
	s.DevicesLock.Lock()
	defer s.DevicesLock.Unlock()
	// 更新设备旋转角度
	for n := 0; n < len(s.Devices); n++ {
		if s.Devices[n].DeviceId == deviceId {
			s.Devices[n].DeviceStatus.Orientation = Rotation.Orientation
			logs.Info("中间件[%d]收到屏幕旋转事件,设备编号:%d,%s", s.MiddlewareId, deviceId, Rotation.Orientation)
			break
		}
	}
}

// EventReceiveDeviceList 处理中间件rtc连接收到的设备列表信息
func (s *TypeInfo) eventReceiveDeviceList(msg *public.Message) {
	// 解析设备列表信息
	var List []publicStruct.MiddleAgentDeviceInfo
	if err := msg.Unmarshal(&List); err != nil {
		logs.Error("中间件[%d]收到的设备列表数据反序列化失败: %v", s.MiddlewareId, err)
		return
	}
	s.DevicesLock.Lock()
	defer s.DevicesLock.Unlock()
	// 更新设备列表

	for i := 0; i < len(s.Devices); i++ {
		for _, device := range List {
			if s.Devices[i].DeviceId == publicStruct.GetDeviceIdFromMiddleIdAndSeat(s.MiddlewareId, device.Seat) {
				s.Devices[i].MiddleAgentDevice = device
				break
			}
		}

	}

}
