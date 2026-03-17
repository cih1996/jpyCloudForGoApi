package centControlPlatform

import (
	"adminApi"
	"port-mapping-demo/third_party/JpyApiAgent/table/coreClass/middleAgentRtc"
	"port-mapping-demo/third_party/JpyApiAgent/table/coreClass/portMapSocket5Rtc"
	"port-mapping-demo/third_party/JpyApiAgent/table/coreClass/publicStruct"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"cnb.cool/accbot/goTool/sessionPkg"
	"github.com/ghp3000/logs"
	"github.com/ghp3000/public"
)

type Core struct {
	server       apiInfo
	Devices      sync.Map // 设备列表
	MiddleAgents sync.Map // 中间件列表
	PortMaps     sync.Map // 端口映射列表

	middleWareId sync.Map // 中间件id列表内部使用

	url    string
	apiKey string
	token  string
}

type apiInfo struct {
	session *sessionPkg.Session
	api     *adminApi.AdminApi
}
type LoginReq struct {
	Apikey string `json:"secretKey"`
}
type DevicesAndMiddleIds struct {
	MiddleWareIds []uint64             `json:"middleWareIds"`
	Devices       []*publicStruct.Device `json:"devices"`
}

func (c *Core) saveDevice(deviceId uint64, device *publicStruct.Device) {
	c.Devices.Store(deviceId, device)
}
func (c *Core) getDevice(deviceId uint64) (*publicStruct.Device, bool) {
	device, ok := c.Devices.Load(deviceId)
	if ok {
		return device.(*publicStruct.Device), ok
	}
	return nil, ok
}
func (c *Core) deleteDevice(deviceId uint64) {
	c.Devices.Delete(deviceId)
}
func (c *Core) GetAllDevice() []*publicStruct.Device {
	var devices []*publicStruct.Device
	c.Devices.Range(func(key, value any) bool {
		devices = append(devices, value.(*publicStruct.Device))
		return true
	})
	// 按照 DeviceId 从小到大排序
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].DeviceId < devices[j].DeviceId
	})
	return devices
}
func (c *Core) deleteAllDevices() {
	c.Devices.Range(func(key, value any) bool {
		c.Devices.Delete(key)
		return true
	})
}

// saveMiddleAgent 保存中间件对象
func (c *Core) saveMiddleAgent(MiddlewareId uint64, middleAgent *middleAgentRtc.TypeInfo) {
	c.MiddleAgents.Store(MiddlewareId, middleAgent)
}

// GetMiddleAgent 获取中间件对象
func (c *Core) GetMiddleAgent(MiddlewareId uint64) (*middleAgentRtc.TypeInfo, bool) {
	value, ok := c.MiddleAgents.Load(MiddlewareId)
	if ok {
		return value.(*middleAgentRtc.TypeInfo), ok
	}
	return nil, ok
}

// deleteMiddleAgent 删除中间件对象
func (c *Core) deleteMiddleAgent(MiddlewareId uint64) {
	c.MiddleAgents.Delete(MiddlewareId)
}

// GetAllMiddleAgents 获取所有中间件对象
func (c *Core) GetAllMiddleAgents() []*middleAgentRtc.TypeInfo {
	var middleAgents []*middleAgentRtc.TypeInfo
	c.MiddleAgents.Range(func(key, value any) bool {
		middleAgents = append(middleAgents, value.(*middleAgentRtc.TypeInfo))
		logs.Info("[GetAllMiddleAgents]:%v", value.(*publicStruct.MiddleAgentTypeInfo).MiddlewareId)
		return true
	})
	// 按照 MiddleId 从小到大排序
	sort.Slice(middleAgents, func(i, j int) bool {
		return middleAgents[i].MiddlewareId < middleAgents[j].MiddlewareId
	})
	return middleAgents
}

// DeleteAllMiddleAgents 删除所有中间件对象
func (c *Core) DeleteAllMiddleAgents() {
	c.MiddleAgents.Range(func(key, value any) bool {
		c.MiddleAgents.Delete(key)
		return true
	})
}

// Reset 重置 Core 状态（用于强制重新登录）
func (c *Core) Reset() {
	logs.Info("[Core.Reset] 开始重置 Core 状态...")
	// 关闭旧的 session 连接
	if c.server.session != nil {
		logs.Info("[Core.Reset] 关闭旧的 session 连接...")
		c.server.session.Close("forceRelogin")
		c.server.session = nil
		c.server.api = nil
	}
	// 清理 token，强制下次登录使用 apiKey
	c.token = ""
	// 清理设备列表
	c.Devices.Range(func(key, value any) bool {
		c.Devices.Delete(key)
		return true
	})
	// 清理中间件列表
	c.DeleteAllMiddleAgents()
	// 清理中间件 ID 列表
	c.deleteAllMiddleWareIds()
	// 清理端口映射列表
	c.PortMaps.Range(func(key, value any) bool {
		c.PortMaps.Delete(key)
		return true
	})
	logs.Info("[Core.Reset] Core 状态已重置")
}

// deleteAllMiddleWareIds 删除所有中间件id 内部使用
func (c *Core) deleteAllMiddleWareIds() {
	c.middleWareId.Range(func(key, value any) bool {
		c.middleWareId.Delete(key)
		return true
	})
}

// saveMiddleWareId 保存中间件id 内部使用
func (c *Core) saveMiddleWareId(middleWareId uint64) {
	c.middleWareId.Store(middleWareId, middleWareId)
}

// GetAllMiddleWareIds 获取所有中间件id 内部使用
func (c *Core) GetAllMiddleWareIds() (middleWareIds []uint64) {
	c.middleWareId.Range(func(key, value any) bool {
		middleWareIds = append(middleWareIds, value.(uint64))
		return true
	})
	// 按照 MiddleWareId 从小到大排序
	sort.Slice(middleWareIds, func(i, j int) bool {
		return middleWareIds[i] < middleWareIds[j]
	})
	return middleWareIds
}

// savePortMap 保存端口映射对象
func (c *Core) savePortMap(deviceId uint64, portMap *portMapSocket5Rtc.TypeInfo) {
	c.PortMaps.Store(deviceId, portMap)
}

// GetPortMap 获取端口映射对象
func (c *Core) GetPortMap(deviceId uint64) (*portMapSocket5Rtc.TypeInfo, bool) {
	value, ok := c.PortMaps.Load(deviceId)
	if ok {
		return value.(*portMapSocket5Rtc.TypeInfo), ok
	}
	return nil, ok
}

// deletePortMap 删除端口映射对象
func (c *Core) deletePortMap(deviceId uint64) {
	c.PortMaps.Delete(deviceId)
}

// DeleteAllPortMaps 删除所有端口映射对象
func (c *Core) DeleteAllPortMaps() {
	c.PortMaps.Range(func(key, value any) bool {
		c.PortMaps.Delete(key)
		return true
	})
}

// GetAllPortMaps 获取所有端口映射对象
func (c *Core) GetAllPortMaps() []*portMapSocket5Rtc.TypeInfo {
	var portMaps []*portMapSocket5Rtc.TypeInfo
	c.PortMaps.Range(func(key, value any) bool {
		portMaps = append(portMaps, value.(*portMapSocket5Rtc.TypeInfo))
		return true
	})
	// 按照 DeviceId 从小到大排序
	sort.Slice(portMaps, func(i, j int) bool {
		return portMaps[i].DeviceId < portMaps[j].DeviceId
	})
	return portMaps
}

func msgPackToJson(msg *public.Message, logMsg string) (string, error) {
	var a interface{}
	err := msg.Unmarshal(&a)
	if err != nil {
		return "", fmt.Errorf("[%s]消息解析错误,err=%s", logMsg, err.Error())
	}
	jsonData, err := json.Marshal(a)
	if err != nil {
		return "", fmt.Errorf("[%s]消息解析错误,err=%s", logMsg, err.Error())
	}
	return string(jsonData), nil
}
