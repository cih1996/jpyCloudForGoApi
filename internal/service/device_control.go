package service

import (
	"context"
	"encoding/json"
	"fmt"
	"port-mapping-demo/pkg/logger"
	"port-mapping-demo/third_party/JpyApiAgent/table/coreClass/publicStruct"
	"strings"
	"time"

	"adminApi/userDeviceCtl"

	"github.com/ghp3000/logs"
)

// DeviceCommandInfo 包含执行命令所需的设备信息
type DeviceCommandInfo struct {
	DeviceId            uint64 `json:"deviceId"`
	TbProxyId           uint64 `json:"tbProxyId"`
	TbYunJiUserDeviceId uint64 `json:"tbYunJiUserDeviceId"`
}

// SendGenericCommandToDevice 通过 JpyApiAgent 中间件发送命令到设备
// 根据 F 码分发到对应的 JpyApiAgent sync 方法
// 保持原有函数签名不变，确保 unified_handler.go 无需修改调用方式
func SendGenericCommandToDevice(key string, deviceIds []DeviceCommandInfo, f uint16, data interface{}, req bool, isSync bool, timeout time.Duration) (interface{}, error) {
	// 1. 确保已登录
	if err := EnsureLogin(key); err != nil {
		return nil, fmt.Errorf("登录失败: %v", err)
	}

	core := GetJpyCore()
	if core == nil {
		return nil, fmt.Errorf("JpyApiAgent Core 未初始化")
	}

	// 2. 遍历设备，通过 JpyApiAgent 中间件 sync 方法发送命令
	var allResults []interface{}

	for _, d := range deviceIds {
		middlewareId := publicStruct.GetMiddleIdFromDeviceId(d.DeviceId)
		logger.LogInfo("[device_control] 设备=%d, 中间件=%d, F=%d", d.DeviceId, middlewareId, f)

		// 获取中间件对象（JpyApiAgent 在 CenterGetDeviceList 时已自动创建 RTC 连接）
		middleAgent, ok := core.GetMiddleAgent(middlewareId)
		if !ok || middleAgent == nil {
			// 中间件未连接，尝试重新获取设备列表以触发连接
			logs.Info("[device_control] 中间件 %d 未连接，尝试重新获取设备列表...", middlewareId)
			if _, err := core.CenterGetDeviceList(); err != nil {
				return nil, fmt.Errorf("中间件 %d 连接失败: %v", middlewareId, err)
			}
			middleAgent, ok = core.GetMiddleAgent(middlewareId)
			if !ok || middleAgent == nil {
				return nil, fmt.Errorf("中间件 %d 不存在或未连接", middlewareId)
			}
		}

		// 3. 根据 F 码分发到对应的 JpyApiAgent sync 方法
		var result string
		var err error

		switch f {
		case 4: // 获取设备详细信息
			result, err = middleAgent.SyncGetDeviceDetails(d.DeviceId)
		case 6: // 获取中间件下所有设备状态
			result, err = middleAgent.SyncGetAllList()
		case 289: // 执行 Shell 命令
			shell := extractStringField(data, "shell")
			result, err = middleAgent.SyncShellCommand(d.DeviceId, shell)
		case 290: // 获取应用列表
			result, err = middleAgent.SyncGetAppList(d.DeviceId)
		case 291: // 启动应用
			pkgName := extractStringField(data, "packageName")
			result, err = middleAgent.SyncRunApp(d.DeviceId, pkgName)
		case 293: // 下载并安装应用
			install := extractDownloadInstall(data)
			result, err = middleAgent.SyncDownloadAndInstall(d.DeviceId, install)
		case 294: // 查询下载进度
			id := extractUint32Field(data, "id")
			result, err = middleAgent.SyncCheckProgress(d.DeviceId, id)
		case 516: // 设置 Root 权限
			pkg := extractStringField(data, "pkg")
			result, err = middleAgent.SyncRootApp(d.DeviceId, pkg)
		default:
			// 未知 F 码，尝试通用发送
			result, err = middleAgent.SyncSendToDevice(f, data, d.DeviceId, fmt.Sprintf("[通用命令F=%d]", f))
		}

		if err != nil {
			logs.Error("[device_control] 命令执行失败: 设备=%d, F=%d, err=%v", d.DeviceId, f, err)
			logger.LogError("[device_control] 命令执行失败: 设备=%d, F=%d, err=%v", d.DeviceId, f, err)
			if isSync {
				return nil, err
			}
			continue
		}

		logger.LogInfo("[device_control] 命令执行成功: 设备=%d, F=%d", d.DeviceId, f)

		if isSync {
			// 将 JSON 字符串解析为 interface{} 避免双重编码
			var parsed interface{}
			if jsonErr := json.Unmarshal([]byte(result), &parsed); jsonErr != nil {
				allResults = append(allResults, result)
			} else {
				allResults = append(allResults, parsed)
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

// ==================== 辅助函数：从 data 中提取字段 ====================

// extractStringField 从 data 中提取字符串字段
func extractStringField(data interface{}, field string) string {
	if m, ok := data.(map[string]interface{}); ok {
		if v, ok := m[field].(string); ok {
			return v
		}
	}
	// 尝试 JSON 序列化再解析
	b, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return ""
	}
	if v, ok := m[field].(string); ok {
		return v
	}
	return ""
}

// extractUint32Field 从 data 中提取 uint32 字段
func extractUint32Field(data interface{}, field string) uint32 {
	if m, ok := data.(map[string]interface{}); ok {
		if v, ok := m[field].(float64); ok {
			return uint32(v)
		}
		if v, ok := m[field].(string); ok {
			// 尝试解析字符串
			var n uint32
			fmt.Sscanf(v, "%d", &n)
			return n
		}
	}
	return 0
}

// extractDownloadInstall 从 data 中提取下载安装参数
func extractDownloadInstall(data interface{}) *publicStruct.DownloadAndInstall {
	b, err := json.Marshal(data)
	if err != nil {
		return &publicStruct.DownloadAndInstall{}
	}
	var install publicStruct.DownloadAndInstall
	if err := json.Unmarshal(b, &install); err != nil {
		return &publicStruct.DownloadAndInstall{}
	}
	return &install
}

func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "超时")
}

// ==================== 中间件命令 HTTP 接口 ====================

// MiddleCommandRequest 请求结构
type MiddleCommandRequest struct {
	Key      string `json:"key"`
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
	key := req.Key
	if key == "" {
		loginLock.Lock()
		key = currentKey
		loginLock.Unlock()
	}
	if key == "" {
		return nil, fmt.Errorf("authentication key is required")
	}

	// 查找设备信息
	deviceInfo, err := findDeviceInfo(key, req.DeviceId)
	if err != nil {
		return nil, err
	}

	// 发送命令
	res, err := SendGenericCommandToDevice(key, []DeviceCommandInfo{*deviceInfo}, req.Data.F, req.Data.Data, req.Data.Req, true, 20*time.Second)
	if err != nil {
		return nil, err
	}

	// JpyApiAgent 返回的已经是解析好的 JSON，直接返回
	return &res, nil
}

// processResponse 处理响应数据
// JpyApiAgent 的 sync 方法返回的是 JSON 字符串，已在 SendGenericCommandToDevice 中解析
// 此函数保留用于兼容 unified_handler.go 中的调用
func processResponse(res interface{}) interface{} {
	return res
}

// findDeviceInfo 查找设备信息
func findDeviceInfo(key string, deviceId uint64) (*DeviceCommandInfo, error) {
	if err := EnsureLogin(key); err != nil {
		return nil, err
	}

	globalApi := GetGlobalApi()
	if globalApi == nil {
		return nil, fmt.Errorf("未登录")
	}

	res, err := globalApi.UserDeviceCtl.GetUserDeviceList(&userDeviceCtl.GetUserDeviceListReq{
		PageNum:  1,
		PageSize: 999999,
	})
	if err != nil {
		return nil, fmt.Errorf("获取设备列表失败: %v", err)
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
	return nil, fmt.Errorf("设备 %d 未找到", deviceId)
}

// findDeviceInfoFromCore 从 JpyApiAgent Core 缓存中查找设备信息（不走 API）
func findDeviceInfoFromCore(deviceId uint64) (*DeviceCommandInfo, bool) {
	core := GetJpyCore()
	if core == nil {
		return nil, false
	}
	devices := core.GetAllDevice()
	for _, d := range devices {
		if d.DeviceId == deviceId {
			return &DeviceCommandInfo{
				DeviceId:            d.DeviceId,
				TbProxyId:           uint64(d.DeviceInfo.TbProxyId),
				TbYunJiUserDeviceId: uint64(d.TBYunJiUserDeviceId),
			}, true
		}
	}
	return nil, false
}
