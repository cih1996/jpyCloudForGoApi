package service

import (
	"encoding/json"
	"fmt"

	"adminApi/changeOsCtl"
	"adminApi/userDeviceCtl"
)

func handleSetSocket5(data interface{}) (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}

	type TempReq struct {
		DeviceId uint64 `json:"deviceId"`
		Id       uint64 `json:"id"`
		S5Url    string `json:"s5Url"`
		NOutSwID int    `json:"nOutSwID"`
		LineType int    `json:"lineType"`
	}
	var tempReq TempReq
	if err := json.Unmarshal(dataBytes, &tempReq); err != nil {
		return nil, fmt.Errorf("invalid data format for SetS5: %v", err)
	}

	if tempReq.NOutSwID == 0 {
		if tempReq.LineType == 1 {
			// If lineType is 1, default to 10006
			tempReq.NOutSwID = 11211
		} else if tempReq.LineType != 0 {
			// If lineType is not 0 and not 1, return error
			return nil, fmt.Errorf("unsupported line type: %d", tempReq.LineType)
		}
	}

	targetId := tempReq.DeviceId
	if targetId == 0 {
		targetId = tempReq.Id
	}
	if targetId == 0 {
		return nil, fmt.Errorf("deviceId is required")
	}

	info, err := findDeviceInfoWithCache(targetId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	tbId := int64(info.TbYunJiUserDeviceId)
	req := userDeviceCtl.SetS5Req{
		TbYunJiUserDeviceId: &tbId,
		S5Url:               &tempReq.S5Url,
		NOutSwID:            &tempReq.NOutSwID,
	}

	if errPkg := GetGlobalApi().UserDeviceCtl.SetS5(req); errPkg != nil {
		return nil, fmt.Errorf("%s", errPkg.Msg)
	}
	return nil, nil
}

func handleGetSocket5(data interface{}) (interface{}, error) {
	return nil, fmt.Errorf("use getS5outLine or check device list for S5 info")
}

func handleGetS5outLine(data interface{}) (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}

	type TempReq struct {
		DeviceId uint64 `json:"deviceId"`
		Id       uint64 `json:"id"`
	}
	var tempReq TempReq
	if err := json.Unmarshal(dataBytes, &tempReq); err != nil {
		return nil, fmt.Errorf("invalid data format for GetS5outLine: %v", err)
	}

	targetId := tempReq.DeviceId
	if targetId == 0 {
		targetId = tempReq.Id
	}
	if targetId == 0 {
		return nil, fmt.Errorf("deviceId is required")
	}

	info, err := findDeviceInfoWithCache(targetId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	tbId := int64(info.TbYunJiUserDeviceId)
	req := userDeviceCtl.GetOutLineReq{
		TbYunJiUserDeviceId: &tbId,
	}

	res, errPkg := GetGlobalApi().UserDeviceCtl.GetOutLine(req)
	if errPkg != nil {
		return nil, fmt.Errorf("%s", errPkg.Msg)
	}
	return res, nil
}

func handleSetLocation(data interface{}) (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}

	type SetLocationItem struct {
		DeviceId uint64      `json:"deviceId"`
		Lat      interface{} `json:"lat"`
		Lng      interface{} `json:"lng"`
	}

	var items []SetLocationItem
	// Support both array and single object
	if err := json.Unmarshal(dataBytes, &items); err != nil {
		var item SetLocationItem
		if err2 := json.Unmarshal(dataBytes, &item); err2 == nil {
			items = append(items, item)
		} else {
			return nil, fmt.Errorf("invalid data format: %v", err)
		}
	}

	var results []interface{}

	for _, item := range items {
		if item.DeviceId == 0 {
			results = append(results, map[string]interface{}{"deviceId": 0, "error": "deviceId is required"})
			continue
		}

		info, err := findDeviceInfoWithCache(item.DeviceId)
		if err != nil {
			results = append(results, map[string]interface{}{"deviceId": item.DeviceId, "error": fmt.Sprintf("device not found: %v", err)})
			continue
		}

		did := int64(info.DeviceId)
		latStr := fmt.Sprintf("%v", item.Lat)
		lngStr := fmt.Sprintf("%v", item.Lng)

		req := changeOsCtl.SetLocationReq{
			TbDeviceId: &did,
			Latitude:   &latStr,
			Longitude:  &lngStr,
		}

		errPkg := GetGlobalApi().ChangeOsCtl.SetLocation(req)
		if errPkg != nil {
			results = append(results, map[string]interface{}{"deviceId": item.DeviceId, "error": errPkg.Msg})
		} else {
			results = append(results, map[string]interface{}{"deviceId": item.DeviceId, "result": "success"})
		}
	}

	return results, nil
}
