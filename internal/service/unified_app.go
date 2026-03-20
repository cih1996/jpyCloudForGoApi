package service

import (
	"encoding/json"
	"fmt"
	"time"

	"adminApi/changeOsCtl"
	"adminApi/tbFileCtl"
)

func handleGetAppList(data interface{}) (interface{}, error) {
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
		return nil, fmt.Errorf("invalid data format for GetAppList: %v", err)
	}

	if tempReq.DeviceId == 0 {
		return nil, fmt.Errorf("deviceId is required")
	}

	info, err := findDeviceInfoWithCache(tempReq.DeviceId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	// F=290 for Get App List
	payload := map[string]interface{}{}

	res, err := SendGenericCommandToDevice(unifiedKey, []DeviceCommandInfo{*info}, 290, payload, true, true, 15*time.Second)
	if err != nil {
		return nil, err
	}
	return processResponse(res), nil
}

func handleDownLoadInstallApp(data interface{}) (interface{}, error) {
	// "data":{"devices":[21323,21043],"url":"...","install":true,...}
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid data format")
	}

	devicesInterface, ok := m["devices"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("devices list missing")
	}

	var deviceIds []DeviceCommandInfo
	for _, d := range devicesInterface {
		if did, ok := d.(float64); ok {
			info, err := findDeviceInfoWithCache(uint64(did))
			if err == nil {
				deviceIds = append(deviceIds, *info)
			}
		}
	}

	// Payload for F=293
	// Extract other fields from data
	payload := make(map[string]interface{})
	for k, v := range m {
		if k != "devices" {
			payload[k] = v
		}
	}
	// Ensure mandatory fields
	payload["receive"] = true

	// Use F=293 for Download/Install task
	res, err := SendGenericCommandToDevice(unifiedKey, deviceIds, 293, payload, true, true, 120*time.Second)
	if err != nil {
		return nil, err
	}
	return processResponse(res), nil
}

func handleGetDownloadProgress(data interface{}) (interface{}, error) {
	// Expected data: { "deviceId": 123, "id": 3 } (id 可以是 number 或 string)
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid data format")
	}

	deviceIdVal, ok := m["deviceId"].(float64)
	if !ok {
		return nil, fmt.Errorf("deviceId missing or invalid")
	}
	deviceId := uint64(deviceIdVal)

	// 兼容 id 为 number 或 string
	var idNum float64
	switch v := m["id"].(type) {
	case float64:
		idNum = v
	case string:
		if v == "" {
			return nil, fmt.Errorf("id (task id) missing or empty")
		}
		fmt.Sscanf(v, "%f", &idNum)
	default:
		return nil, fmt.Errorf("id (task id) missing or invalid type")
	}

	info, err := findDeviceInfoWithCache(deviceId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	payload := map[string]interface{}{
		"id": idNum, // 传 number 类型，匹配 extractUint32Field
	}

	// F=294 for Get Download Progress
	res, err := SendGenericCommandToDevice(unifiedKey, []DeviceCommandInfo{*info}, 294, payload, true, true, 15*time.Second)
	if err != nil {
		return nil, err
	}
	return processResponse(res), nil
}

func handleHideApp(data interface{}) (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}

	type TempReq struct {
		DeviceId    uint64 `json:"deviceId"`
		PackageName string `json:"packageName"`
		IsHide      bool   `json:"isHide"`
	}
	var tempReq TempReq
	if err := json.Unmarshal(dataBytes, &tempReq); err != nil {
		return nil, fmt.Errorf("invalid data format for HideApp: %v", err)
	}

	if tempReq.DeviceId == 0 {
		return nil, fmt.Errorf("deviceId is required")
	}
	if tempReq.PackageName == "" {
		return nil, fmt.Errorf("packageName is required")
	}

	info, err := findDeviceInfoWithCache(tempReq.DeviceId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	// Use DeviceId (int64)
	did := int64(info.DeviceId)
	req := changeOsCtl.HideAppReq{
		TbDeviceId:  &did,
		PackageName: &tempReq.PackageName,
		IsHide:      &tempReq.IsHide,
	}

	if errPkg := GetGlobalApi().ChangeOsCtl.HideApp(req); errPkg != nil {
		return nil, fmt.Errorf("%s", errPkg.Msg)
	}
	return "Success", nil
}

func handleGetUserFiles(data interface{}) (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}
	var req tbFileCtl.ListReq
	if err := json.Unmarshal(dataBytes, &req); err != nil {
		return nil, fmt.Errorf("invalid data format for GetUserFiles: %v", err)
	}

	// 1. 获取文件下载基础 URL
	baseUrl, errPkg := GetGlobalApi().TbFileCtl.GetDownloadUrl()
	if errPkg != nil {
		return nil, fmt.Errorf("failed to get download url: %s", errPkg.Msg)
	}

	// 2. 获取文件列表
	res, errPkg := GetGlobalApi().TbFileCtl.List(req)
	if errPkg != nil {
		return nil, fmt.Errorf("%s", errPkg.Msg)
	}

	// 3. Combine to form full URL
	var finalRes []map[string]interface{}
	for _, item := range res {
		itemMap := make(map[string]interface{})
		itemBytes, _ := json.Marshal(item)
		_ = json.Unmarshal(itemBytes, &itemMap)

		itemMap["url"] = baseUrl + "/" + item.Hash
		finalRes = append(finalRes, itemMap)
	}

	return finalRes, nil
}
