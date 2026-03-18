package service

import (
	"encoding/json"
	"fmt"
	"time"

	"adminApi/userDeviceCtl"
)

func handleGetDeviceList() (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}

	// 获取全部设备列表（分页设大值）
	ret, err := GetGlobalApi().UserDeviceCtl.GetUserDeviceList(&userDeviceCtl.GetUserDeviceListReq{
		PageNum:  1,
		PageSize: 999999,
	})

	if err != nil {
		return nil, fmt.Errorf(err.Msg)
	}
	return ret, nil
}

// CheckDeviceOnline 检查设备是否在线（通过 API 接口）
func CheckDeviceOnline(deviceID int) (bool, error) {
	if err := ensureGlobalApi(); err != nil {
		return false, err
	}

	ret, err := GetGlobalApi().UserDeviceCtl.GetUserDeviceList(&userDeviceCtl.GetUserDeviceListReq{
		PageNum:  1,
		PageSize: 999999,
	})

	if err != nil {
		return false, fmt.Errorf(err.Msg)
	}

	for _, d := range ret.Records {
		if int(d.DeviceInfo.DeviceId) == deviceID {
			return d.DeviceInfo.Online, nil
		}
	}

	return false, fmt.Errorf("设备 %d 未找到", deviceID)
}

func handleGetDeviceDetail(data interface{}) (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}

	type TempReq struct {
		DeviceId uint64 `json:"deviceId"`
	}
	var tempReq TempReq
	if err := json.Unmarshal(dataBytes, &tempReq); err != nil {
		return nil, fmt.Errorf("invalid data format for GetDeviceDetail: %v", err)
	}

	if tempReq.DeviceId == 0 {
		return nil, fmt.Errorf("deviceId is required")
	}

	info, err := findDeviceInfoWithCache(tempReq.DeviceId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	// F=4 for Get Device Detail
	payload := map[string]interface{}{}

	res, err := SendGenericCommandToDevice(unifiedKey, []DeviceCommandInfo{*info}, 4, payload, true, true, 15*time.Second)
	if err != nil {
		return nil, err
	}
	return processResponse(res), nil
}

func handleGetDeviceStatus(data interface{}) (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}

	type TempReq struct {
		DeviceId uint64 `json:"deviceId"`
	}
	var tempReq TempReq
	if err := json.Unmarshal(dataBytes, &tempReq); err != nil {
		return nil, fmt.Errorf("invalid data format for GetDeviceStatus: %v", err)
	}

	if tempReq.DeviceId == 0 {
		return nil, fmt.Errorf("deviceId is required")
	}

	info, err := findDeviceInfoWithCache(tempReq.DeviceId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	// F=6 for Get Device Status
	payload := map[string]interface{}{}

	res, err := SendGenericCommandToDevice(unifiedKey, []DeviceCommandInfo{*info}, 6, payload, true, true, 15*time.Second)
	if err != nil {
		return nil, err
	}
	return processResponse(res), nil
}

func handleExecShell(data interface{}) (interface{}, error) {
	// Expected data: { "deviceId": 123, "shell": "ls -l" }
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid data format")
	}

	deviceIdVal, ok := m["deviceId"].(float64)
	if !ok {
		return nil, fmt.Errorf("deviceId missing or invalid")
	}
	deviceId := uint64(deviceIdVal)

	shellCmd, ok := m["shell"].(string)
	if !ok || shellCmd == "" {
		return nil, fmt.Errorf("shell command missing or empty")
	}

	info, err := findDeviceInfoWithCache(deviceId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	payload := map[string]interface{}{
		"shell": shellCmd,
	}

	// F=289 for Shell Execution
	res, err := SendGenericCommandToDevice(unifiedKey, []DeviceCommandInfo{*info}, 289, payload, true, true, 15*time.Second)
	if err != nil {
		return nil, err
	}
	return processResponse(res), nil
}

func handleStartApp(data interface{}) (interface{}, error) {
	// Expected data: { "deviceId": 123, "packageName": "com.example" }
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid data format")
	}

	deviceIdVal, ok := m["deviceId"].(float64)
	if !ok {
		return nil, fmt.Errorf("deviceId missing or invalid")
	}
	deviceId := uint64(deviceIdVal)

	pkgName, ok := m["packageName"].(string)
	if !ok || pkgName == "" {
		return nil, fmt.Errorf("packageName missing or empty")
	}

	info, err := findDeviceInfoWithCache(deviceId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	payload := map[string]interface{}{
		"packageName": pkgName,
	}

	// F=291 for Start App
	res, err := SendGenericCommandToDevice(unifiedKey, []DeviceCommandInfo{*info}, 291, payload, true, true, 15*time.Second)
	if err != nil {
		return nil, err
	}
	return processResponse(res), nil
}

func handleGetRoot(data interface{}) (interface{}, error) {
	// Expected data: { "deviceId": 123, "pkg": "com.android.shell" }
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid data format")
	}

	deviceIdVal, ok := m["deviceId"].(float64)
	if !ok {
		return nil, fmt.Errorf("deviceId missing or invalid")
	}
	deviceId := uint64(deviceIdVal)

	// pkg is optional, defaults to "com.android.shell" if not present
	pkgName := "com.android.shell"
	if v, ok := m["pkg"].(string); ok && v != "" {
		pkgName = v
	}

	info, err := findDeviceInfoWithCache(deviceId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	payload := map[string]interface{}{
		"pkg": pkgName,
	}

	// F=516 for Get Root
	res, err := SendGenericCommandToDevice(unifiedKey, []DeviceCommandInfo{*info}, 516, payload, true, true, 15*time.Second)
	if err != nil {
		return nil, err
	}
	return processResponse(res), nil
}

func handleScreenshot(data interface{}) (interface{}, error) {
	// Expected data: { "deviceId": 123 }
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid data format")
	}

	deviceIdVal, ok := m["deviceId"].(float64)
	if !ok {
		return nil, fmt.Errorf("deviceId missing or invalid")
	}
	deviceId := uint64(deviceIdVal)

	info, err := findDeviceInfoWithCache(deviceId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	payload := map[string]interface{}{}

	// F=288 for Screenshot
	res, err := SendGenericCommandToDevice(unifiedKey, []DeviceCommandInfo{*info}, 288, payload, true, true, 30*time.Second)
	if err != nil {
		return nil, err
	}
	return processResponse(res), nil
}
