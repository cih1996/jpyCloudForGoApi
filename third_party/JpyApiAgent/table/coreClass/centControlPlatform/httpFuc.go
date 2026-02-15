package centControlPlatform

import (
	"port-mapping-demo/third_party/JpyApiAgent/table/coreClass/publicStruct"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (c *Core) httpLogin(ss *gin.Context, bi *bodyInfo) bool {
	var req LoginReq
	if err := json.Unmarshal(bi.Data, &req); err != nil {
		ss.JSON(http.StatusOK, gin.H{"code": -1, "msg": "invalid data"})
		return true
	}

	// 更新 apiKey
	c.apiKey = req.Apikey

	// 调用 Login 函数
	if success := c.Login(); success {
		ss.JSON(http.StatusOK, gin.H{"code": 0, "msg": "login success", "data": c.token})
	} else {
		ss.JSON(http.StatusOK, gin.H{"code": -1, "msg": "login failed", "data": ""})
	}
	return true
}

func (c *Core) httpGetDeviceList(ss *gin.Context, bi *bodyInfo) bool {
	list, err := c.CenterGetDeviceList()
	if err != nil {
		ss.JSON(http.StatusOK, gin.H{"code": -1, "msg": err.Error(), "data": nil})
		return true
	}
	ss.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": list})
	return true
}

// ==================== 同步命令通用数据结构 ====================

type syncReqData struct {
	MiddleWareId uint64 `json:"middleWareId"`
	DeviceId     uint64 `json:"deviceId"`
	Mode         int    `json:"mode"`
	Shell        string `json:"shell"`
	PackageName  string `json:"packageName"`
	Pkg          string `json:"pkg"`
	Width        int    `json:"width"`
	Type         int    `json:"type"`
	Id           uint32 `json:"id"`
	Url          string `json:"url"`
	Sha256       string `json:"sha256"`
	Install      bool   `json:"install"`
	Name         string `json:"name"`
}

// httpMiddleAgentSync 处理中间件同步命令的 HTTP 请求
func (c *Core) httpMiddleAgentSync(ss *gin.Context, bi *bodyInfo) bool {
	// 端口映射命令走独立处理（文档中 app=middleAgent, fun=portMapXxx）
	//switch bi.Fun {
	//case "portMapCreate":
	//	bi.Fun = "create"
	//	return c.httpPortMap(ss, bi)
	//case "portMapStatus":
	//	bi.Fun = "status"
	//	return c.httpPortMap(ss, bi)
	//case "portMapDelete":
	//	bi.Fun = "delete"
	//	return c.httpPortMap(ss, bi)
	//}

	var req syncReqData
	if err := json.Unmarshal(bi.Data, &req); err != nil {
		ss.JSON(http.StatusOK, gin.H{"code": -1, "msg": "invalid data"})
		return true
	}

	// connect 命令：主动创建中间件 RTC 连接
	if bi.Fun == "connect" {
		middleAgent, err := c.CreatMiddlewareRtc(req.MiddleWareId, nil, nil, nil)
		if err != nil {
			ss.JSON(http.StatusOK, gin.H{"code": -1, "msg": err.Error(), "data": nil})
			return true
		}
		ss.JSON(http.StatusOK, gin.H{"code": 0, "msg": "中间件连接成功", "data": gin.H{"middleWareId": middleAgent.MiddlewareId}})
		return true
	}

	// 获取中间件对象
	middleAgent, ok := c.GetMiddleAgent(req.MiddleWareId)
	if !ok || middleAgent == nil {
		ss.JSON(http.StatusOK, gin.H{"code": -1, "msg": "中间件不存在或未连接", "data": nil})
		return true
	}

	var result interface{}
	var err error

	switch bi.Fun {
	case "syncGetAllList":
		result, err = middleAgent.SyncGetAllList()
	case "syncGetOnlineList":
		result, err = middleAgent.SyncGetOnlineList()
	case "syncGetDeviceDetails":
		result, err = middleAgent.SyncGetDeviceDetails(req.DeviceId)
	case "syncModeSwitch":
		result, err = middleAgent.SyncModeSwitch(req.DeviceId, req.Mode)
	case "syncGetPluginList":
		result, err = middleAgent.SyncGetPluginList()
	case "syncDevicePowerControl":
		result, err = middleAgent.SyncDevicePowerControl(req.DeviceId, req.Mode)
	case "syncEnterFlashingMode":
		result, err = middleAgent.SyncEnterFlashingMode(req.DeviceId, req.Mode)
	case "syncSetDeviceToFindMode":
		result, err = middleAgent.SyncSetDeviceToFindMode(req.DeviceId, req.Mode)
	case "syncGetMiddlewareWorkMode":
		result, err = middleAgent.SyncGetMiddlewareWorkMode()
	case "syncShellCommand":
		result, err = middleAgent.SyncShellCommand(req.DeviceId, req.Shell)
	case "syncGetAppList":
		result, err = middleAgent.SyncGetAppList(req.DeviceId)
	case "syncRunApp":
		result, err = middleAgent.SyncRunApp(req.DeviceId, req.PackageName)
	case "syncDownloadAndInstall":
		install := &publicStruct.DownloadAndInstall{
			Url:     req.Url,
			Sha256:  req.Sha256,
			Install: req.Install,
			Name:    req.Name,
		}
		result, err = middleAgent.SyncDownloadAndInstall(req.DeviceId, install)
	case "syncCheckProgress":
		result, err = middleAgent.SyncCheckProgress(req.DeviceId, req.Id)
	case "syncScreenOff":
		result, err = middleAgent.SyncScreenOff(req.DeviceId)
	case "syncScreenOn":
		result, err = middleAgent.SyncScreenOn(req.DeviceId)
	case "syncRootApp":
		result, err = middleAgent.SyncRootApp(req.DeviceId, req.Pkg)
	case "syncCancelRootApp":
		result, err = middleAgent.SyncCancelRootApp(req.DeviceId, req.Pkg)
	case "syncSwitchBack":
		result, err = middleAgent.SyncSwitchBack(req.DeviceId)
	case "syncSwitchFront":
		result, err = middleAgent.SyncSwitchFront(req.DeviceId)
	case "syncGetClipboard":
		result, err = middleAgent.SyncGetClipboard(req.DeviceId)
	case "syncGetDeviceImage":
		result, err = middleAgent.SyncGetDeviceImageToBase64(req.DeviceId, req.Width, req.Type)
	case "syncDelayDetection":
		delay := middleAgent.SyncDelayedetection()
		ss.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": delay})
		return true
	default:
		ss.JSON(http.StatusOK, gin.H{"code": -1, "msg": "未知的同步命令: " + bi.Fun})
		return true
	}

	if err != nil {
		ss.JSON(http.StatusOK, gin.H{"code": -1, "msg": err.Error(), "data": nil})
		return true
	}

	// result 是 JSON 字符串，尝试解析为 interface{} 以避免双重编码
	var parsedData interface{}
	if strResult, ok := result.(string); ok {
		if jsonErr := json.Unmarshal([]byte(strResult), &parsedData); jsonErr != nil {
			parsedData = strResult
		}
	} else {
		parsedData = result
	}

	ss.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": parsedData})
	return true
}

// ==================== 端口映射 HTTP 接口 ====================

type portMapReqData struct {
	DeviceId  uint64 `json:"deviceId"`
	PhonePort int    `json:"phonePort"`
	LocalPort int    `json:"localPort"`
	Mode      int    `json:"mode"`
}

// httpPortMap 处理端口映射的 HTTP 请求
func (c *Core) httpPortMap(ss *gin.Context, bi *bodyInfo) bool {
	var req portMapReqData
	if err := json.Unmarshal(bi.Data, &req); err != nil {
		ss.JSON(http.StatusOK, gin.H{"code": -1, "msg": "invalid data"})
		return true
	}

	switch bi.Fun {
	case "create":
		portMap, err := c.CreatPortMapSocket5Rtc(req.DeviceId, req.PhonePort, req.LocalPort, req.Mode)
		if err != nil {
			ss.JSON(http.StatusOK, gin.H{"code": -1, "msg": err.Error(), "data": nil})
			return true
		}
		ss.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": gin.H{
			"deviceId":  portMap.DeviceId,
			"phonePort": portMap.PhonePort,
			"localPort": portMap.LocalPort,
			"mode":      portMap.Mode,
		}})
		return true

	case "status":
		portMap, ok := c.GetPortMap(req.DeviceId)
		if !ok || portMap == nil {
			ss.JSON(http.StatusOK, gin.H{"code": -1, "msg": "端口映射不存在", "data": nil})
			return true
		}
		ss.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": gin.H{
			"code": portMap.Code,
			"msg":  portMap.Msg,
		}})
		return true

	case "delete":
		portMap, ok := c.GetPortMap(req.DeviceId)
		if !ok || portMap == nil {
			ss.JSON(http.StatusOK, gin.H{"code": -1, "msg": "端口映射不存在", "data": nil})
			return true
		}
		if portMap.Rtc != nil {
			portMap.Reconnect = 1
			_ = portMap.Rtc.Close()
		}
		c.deletePortMap(req.DeviceId)
		ss.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": nil})
		return true

	default:
		ss.JSON(http.StatusOK, gin.H{"code": -1, "msg": "未知的端口映射命令: " + bi.Fun})
		return true
	}
}
