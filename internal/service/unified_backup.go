package service

import (
	"encoding/json"
	"fmt"

	"adminApi/changeOsCtl"

	"github.com/ghp3000/logs"
)

// handleBackupApp 应用备份
func handleBackupApp(data interface{}) (interface{}, error) {
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
	}
	var tempReq TempReq
	if err := json.Unmarshal(dataBytes, &tempReq); err != nil {
		return nil, fmt.Errorf("invalid data format for BackupApp: %v", err)
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

	did := int64(info.DeviceId)
	req := changeOsCtl.BackupAppReq{
		TbDeviceId:  &did,
		PackageName: &tempReq.PackageName,
	}

	path, errPkg := GetGlobalApi().ChangeOsCtl.BackupApp(req)
	if errPkg != nil {
		return nil, fmt.Errorf("%s", errPkg.Msg)
	}
	return map[string]interface{}{
		"deviceId":    tempReq.DeviceId,
		"packageName": tempReq.PackageName,
		"path":        path,
	}, nil
}

// handleGetBackupAppStatus 应用备份状态查询
func handleGetBackupAppStatus(data interface{}) (interface{}, error) {
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
	}
	var tempReq TempReq
	if err := json.Unmarshal(dataBytes, &tempReq); err != nil {
		return nil, fmt.Errorf("invalid data format for GetBackupAppStatus: %v", err)
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

	did := int64(info.DeviceId)
	req := changeOsCtl.GetBackupAppStatusReq{
		TbDeviceId:  &did,
		PackageName: &tempReq.PackageName,
	}

	res, errPkg := GetGlobalApi().ChangeOsCtl.GetBackupAppStatus(req)
	if errPkg != nil {
		return nil, fmt.Errorf("%s", errPkg.Msg)
	}
	return map[string]interface{}{
		"deviceId":    tempReq.DeviceId,
		"packageName": tempReq.PackageName,
		"code":        res.Code,
		"msg":         res.Msg,
	}, nil
}

// handleGetBackupList 获取应用备份列表（通过 session.SendFun 直接调用 appBackupCtl.list）
func handleGetBackupList(data interface{}) (interface{}, error) {
	logs.Info("[getBackupList] 开始处理请求")

	if err := ensureGlobalApi(); err != nil {
		logs.Error("[getBackupList] ensureGlobalApi 失败: %v", err)
		return nil, err
	}
	logs.Info("[getBackupList] ensureGlobalApi 成功, currentHost=%s", GetCurrentHost())

	core := GetJpyCore()
	if core == nil {
		logs.Error("[getBackupList] JpyCore is nil")
		return nil, fmt.Errorf("JpyCore not initialized")
	}

	session := core.GetSession()
	if session == nil {
		logs.Error("[getBackupList] session is nil")
		return nil, fmt.Errorf("session not available")
	}
	logs.Info("[getBackupList] session 获取成功")

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}

	type TempReq struct {
		DeviceId uint64 `json:"deviceId"`
		PageNum  int    `json:"pageNum"`
		PageSize int    `json:"pageSize"`
	}
	var tempReq TempReq
	if err := json.Unmarshal(dataBytes, &tempReq); err != nil {
		return nil, fmt.Errorf("invalid data format for GetBackupList: %v", err)
	}

	if tempReq.DeviceId == 0 {
		return nil, fmt.Errorf("deviceId is required")
	}

	// 设置默认分页参数
	pageNum := tempReq.PageNum
	pageSize := tempReq.PageSize
	if pageSize == 0 {
		pageSize = 1000
	}

	// 构造请求参数
	// 用户传入的 deviceId 就是 tbDeviceId（集控平台显示的设备ID），直接使用
	reqData := map[string]interface{}{
		"tbDeviceId": tempReq.DeviceId,
		"pageNum":    pageNum,
		"pageSize":   pageSize,
	}

	logs.Info("[getBackupList] 请求参数: tbDeviceId=%d, pageNum=%d, pageSize=%d", tempReq.DeviceId, pageNum, pageSize)

	// 直接调用 appBackupCtl.list
	resBuf, errPkg := session.SendFun("appBackupCtl", "list", reqData, 30*1000)
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

// handleRestoreApp 应用还原
func handleRestoreApp(data interface{}) (interface{}, error) {
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
		Path        string `json:"path"`
	}
	var tempReq TempReq
	if err := json.Unmarshal(dataBytes, &tempReq); err != nil {
		return nil, fmt.Errorf("invalid data format for RestoreApp: %v", err)
	}

	if tempReq.DeviceId == 0 {
		return nil, fmt.Errorf("deviceId is required")
	}
	if tempReq.PackageName == "" {
		return nil, fmt.Errorf("packageName is required")
	}
	if tempReq.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	info, err := findDeviceInfoWithCache(tempReq.DeviceId)
	if err != nil {
		return nil, fmt.Errorf("device not found: %v", err)
	}

	did := int64(info.DeviceId)
	req := changeOsCtl.RestoreAppReq{
		TbDeviceId:  &did,
		PackageName: &tempReq.PackageName,
		Path:        &tempReq.Path,
	}

	errPkg := GetGlobalApi().ChangeOsCtl.RestoreApp(req)
	if errPkg != nil {
		return nil, fmt.Errorf("%s", errPkg.Msg)
	}
	return map[string]interface{}{
		"deviceId":    tempReq.DeviceId,
		"packageName": tempReq.PackageName,
		"path":        tempReq.Path,
		"result":      "success",
	}, nil
}

// handleGetRestoreAppStatus 应用还原状态查询
func handleGetRestoreAppStatus(data interface{}) (interface{}, error) {
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
	}
	var tempReq TempReq
	if err := json.Unmarshal(dataBytes, &tempReq); err != nil {
		return nil, fmt.Errorf("invalid data format for GetRestoreAppStatus: %v", err)
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

	did := int64(info.DeviceId)
	req := changeOsCtl.GetRestoreAppStatusReq{
		TbDeviceId:  &did,
		PackageName: &tempReq.PackageName,
	}

	res, errPkg := GetGlobalApi().ChangeOsCtl.GetRestoreAppStatus(req)
	if errPkg != nil {
		return nil, fmt.Errorf("%s", errPkg.Msg)
	}
	return map[string]interface{}{
		"deviceId":    tempReq.DeviceId,
		"packageName": tempReq.PackageName,
		"code":        res.Code,
		"msg":         res.Msg,
	}, nil
}
