package centControlPlatform

import (
	"adminApi"
	"adminApi/loginCtl"
	"adminApi/userDeviceCtl"
	"port-mapping-demo/third_party/JpyApiAgent/table/coreClass/publicStruct"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"cnb.cool/accbot/goTool/sessionPkg"
	"cnb.cool/accbot/goTool/toolPkg"
	"cnb.cool/accbot/goTool/wsPkg"
	"github.com/ghp3000/logs"
	"github.com/gorilla/websocket"
)

var core *Core

func NewCore(tableIP string, apiKey string) *Core {
	if core == nil {
		core = &Core{}
	}
	core.url = tableIP
	core.apiKey = apiKey
	core.InitHttpServer()
	return core
}

// Login 创建一个Api用户并登录，默认自动登录
func (c *Core) Login() bool {
	dialer := &websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // 关键：跳过证书验证
		},
	}
	con, _, err := dialer.Dial(fmt.Sprintf("wss://%s/ws", c.url), nil)
	if err != nil {
		logs.Error("连接底层服务 ", fmt.Sprintf("尝试连接底层服务失败,请重试"))
		toolPkg.SafeGo(func() {
			time.Sleep(time.Second * 3)
			c.Login()
		})
		return false
	}
	conn := wsPkg.NewWSConnByConn(con)
	c.server.session = sessionPkg.CreateSession(sessionPkg.SessionType_ws, conn)
	c.server.api = adminApi.NewAdminApi(c.server.session)

	if c.token == "" {
		res, err1 := c.server.api.LoginCtl.SecretKeyLogin(&loginCtl.SecretKeyLoginReq{SecretKey: &c.apiKey})
		if err1 != nil {
			logs.Error("集控平台apiKey登录失败", err1.Msg)
			return false
		} else {
			logs.Info("集控平台apiKey登录成功token=[%s],userName=[%s]", res.Token, res.UserInfo.UserName)
			c.token = res.Token
		}

	} else {
		res, err1 := c.server.api.LoginCtl.SafeLogin(&loginCtl.SafeLoginReq{Token: &c.token})
		if err1 != nil {
			logs.Error("集控平台安全登录失败%s", err1.Msg)
			res, err1 := c.server.api.LoginCtl.SecretKeyLogin(&loginCtl.SecretKeyLoginReq{SecretKey: &c.apiKey})
			if err1 != nil {
				logs.Error("集控平台apiKey登录失败%s", err1.Msg)
				return false
			}

			logs.Info("集控平台apiKey登录成功token=[%s],userName=[%s]", res.Token, res.UserInfo.UserName)
			c.token = res.Token

		} else {
			logs.Info("集控平台apiKey登录成功token=[%s],userName=[%s]", res.Token, res.UserInfo.UserName)
			c.token = res.Token
		}

	}

	c.server.session.RegExpiryCallback(func(msg string) {
		logs.Error("集控平台和服务端连接断开了%s", msg)
		toolPkg.SafeGo(func() {
			time.Sleep(time.Second * 3)
			c.Login()
		})
	})
	return true
}

// CenterGetDeviceList 获取所有设备列表 json文本
func (c *Core) CenterGetDeviceList() (*DevicesAndMiddleIds, error) {
	//获取设备列表
	if c.server.api == nil {
		return nil, errors.New("api为nil,获取所有设备列表失败")
	}

	ret, err := c.server.api.UserDeviceCtl.GetUserDeviceList(&userDeviceCtl.GetUserDeviceListReq{
		PageNum:                               0,
		PageSize:                              999999,
		GetUserDeviceListgetUserDeviceListReq: userDeviceCtl.GetUserDeviceListgetUserDeviceListReq{},
	})
	if err != nil {
		logs.Info("设备列表获取错误", err)
		return nil, errors.New("获取设备列表错误")
	} else {

		var devices []*publicStruct.Device
		var device *publicStruct.Device
		c.deleteAllMiddleWareIds()
		c.deleteAllDevices()
		for n := 0; n < len(ret.Records); n++ {
			device = &publicStruct.Device{}
			device.DeviceId = uint64(ret.Records[n].DeviceId)
			device.TBYunJiUserDeviceId = ret.Records[n].TbYunJiUserDeviceId
			device.CreatedAt = ret.Records[n].CreatedAt
			device.UserId = ret.Records[n].UserId
			device.ExpiresTime = ret.Records[n].ExpiresTime
			device.BuyType = ret.Records[n].BuyType
			device.YunjiUserGroupId = ret.Records[n].YunjiUserGroupId
			device.Remark = ret.Records[n].Remark
			device.DeviceInfo = ret.Records[n].DeviceInfo
			err1 := json.Unmarshal([]byte(device.DeviceInfo.Info), &device.MiddleAgentDevice)
			if err1 != nil {
				logs.Info("设备列表Info解析错误", err1)
			} else {
				//logs.Info("设备列表Info", device.DeviceId, device.MiddleAgentDevice)
			}
			c.saveDevice(device.DeviceId, device)
			c.saveMiddleWareId(publicStruct.GetMiddleIdFromDeviceId(device.DeviceId))
			devices = append(devices, device)
		}
		mids := c.GetAllMiddleWareIds()
		for t := 0; t < len(mids); t++ {
			_, _ = c.CreatMiddlewareRtc(mids[t], nil, nil, nil)
		}

		list := DevicesAndMiddleIds{
			MiddleWareIds: c.GetAllMiddleWareIds(),
			Devices:       devices,
		}
		//jsonStr, err1 := json.Marshal(&devicesAndMiddleIds{
		//	Devices:       devices,
		//	MiddleWareIds: c.GetAllMiddleWareIds(),
		//})
		//if err1 != nil {
		//	logs.Info("设备列表转换JSON错误", err1)
		//	return "", err1
		//}
		//logs.Info("设备列表JSON字符串", string(jsonStr))
		//str := string(jsonStr)
		return &list, nil
	}
}
