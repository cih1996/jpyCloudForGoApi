package service

import (
	"encoding/json"
	"fmt"

	"adminApi/changeOsCtl"
)

// For Changephones (Type 3)
// Data: [{"deviceId":..., "type":"changeDevice", "func":1, "paramsAll":...}]
func handleChangePhones(data interface{}) (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}

	var reqs []*changeOsCtl.ChangeOsReq
	if err := json.Unmarshal(dataBytes, &reqs); err != nil {
		return nil, fmt.Errorf("invalid data format for ChangeOs: %v", err)
	}

	res, errPkg := GetGlobalApi().ChangeOsCtl.ChangeOs(reqs)
	if errPkg != nil {
		return nil, fmt.Errorf("%s", errPkg.Msg)
	}

	return parseChangeOsRes(res), nil
}

func handleGetTaskStatus(data interface{}) (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}

	var req changeOsCtl.GetChangeOsStatusReq
	if err := json.Unmarshal(dataBytes, &req); err != nil {
		return nil, fmt.Errorf("invalid data format for GetTaskStatus: %v", err)
	}

	res, errPkg := GetGlobalApi().ChangeOsCtl.GetChangeOsStatus(req)
	if errPkg != nil {
		return nil, fmt.Errorf("%s", errPkg.Msg)
	}
	return parseGetChangeOsStatusRes(res), nil
}

// Helper to parse Data field in responses
func parseChangeOsRes(res []*changeOsCtl.ChangeOsRes) []map[string]interface{} {
	var finalRes []map[string]interface{}
	for _, s := range res {
		sMap := make(map[string]interface{})
		sBytes, _ := json.Marshal(s)
		_ = json.Unmarshal(sBytes, &sMap)

		if s.Data != "" {
			var dataObj interface{}
			if err := json.Unmarshal([]byte(s.Data), &dataObj); err == nil {
				sMap["dataObj"] = dataObj
			}
		}
		finalRes = append(finalRes, sMap)
	}
	return finalRes
}

func parseGetChangeOsStatusRes(res []*changeOsCtl.GetChangeOsStatusRes) []map[string]interface{} {
	var finalRes []map[string]interface{}
	for _, s := range res {
		sMap := make(map[string]interface{})
		sBytes, _ := json.Marshal(s)
		_ = json.Unmarshal(sBytes, &sMap)

		if s.Data != "" {
			var dataObj interface{}
			if err := json.Unmarshal([]byte(s.Data), &dataObj); err == nil {
				sMap["dataObj"] = dataObj
			}
		}
		finalRes = append(finalRes, sMap)
	}
	return finalRes
}

// handleChangeOldOsReq 还原机型
func handleChangeOldOsReq(data interface{}) (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}

	type TempReq struct {
		DeviceId   uint64 `json:"deviceId"`
		ChangeOsId int64  `json:"changeOsId"`
	}
	var tempReq TempReq
	if err := json.Unmarshal(dataBytes, &tempReq); err != nil {
		return nil, fmt.Errorf("invalid data format for ChangeOldOsReq: %v", err)
	}

	if tempReq.DeviceId == 0 {
		return nil, fmt.Errorf("deviceId is required")
	}
	if tempReq.ChangeOsId == 0 {
		return nil, fmt.Errorf("changeOsId is required")
	}

	info, err := findDeviceInfoWithCache(tempReq.DeviceId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	did := int64(info.DeviceId)
	req := changeOsCtl.ChangeOldOsReqReq{
		DeviceId:   &did,
		ChangeOsId: &tempReq.ChangeOsId,
	}

	errPkg := GetGlobalApi().ChangeOsCtl.ChangeOldOsReq(req)
	if errPkg != nil {
		return nil, fmt.Errorf("%s", errPkg.Msg)
	}
	return map[string]interface{}{
		"deviceId":   tempReq.DeviceId,
		"changeOsId": tempReq.ChangeOsId,
		"result":     "success",
	}, nil
}

// handleGetChangeOsList 获取可还原的机型列表（通过 session.SendFun 直接调用 changeOsCtl.getChangeOsList）
func handleGetChangeOsList(data interface{}) (interface{}, error) {
	if err := ensureGlobalApi(); err != nil {
		return nil, err
	}

	core := GetJpyCore()
	if core == nil {
		return nil, fmt.Errorf("JpyCore not initialized")
	}

	session := core.GetSession()
	if session == nil {
		return nil, fmt.Errorf("session not available")
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
		return nil, fmt.Errorf("invalid data format for GetChangeOsList: %v", err)
	}

	if tempReq.DeviceId == 0 {
		return nil, fmt.Errorf("deviceId is required")
	}

	// 构造请求参数
	reqData := map[string]interface{}{
		"deviceId": tempReq.DeviceId,
	}

	// 直接调用 changeOsCtl.getChangeOsList
	resBuf, errPkg := session.SendFun("changeOsCtl", "getChangeOsList", reqData, 30*1000)
	if errPkg != nil {
		return nil, fmt.Errorf("%s", errPkg.Msg)
	}

	// 解析返回结果
	var result interface{}
	if err := json.Unmarshal(resBuf, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return result, nil
}
