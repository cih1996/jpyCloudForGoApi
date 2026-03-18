package service

import (
	"encoding/json"
	"time"

	"github.com/ghp3000/logs"
)

// DeviceCommandRequest 设备命令请求结构
type DeviceCommandRequest struct {
	DeviceId  uint64      `json:"deviceId"`
	Type      string      `json:"type"`
	Func      int         `json:"func"`
	ParamsAll interface{} `json:"paramsAll"`
}

func processDeviceCommands(data interface{}) (interface{}, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var commands []DeviceCommandRequest
	if err := json.Unmarshal(dataBytes, &commands); err != nil {
		return nil, err
	}

	var results []interface{}

	for _, cmd := range commands {
		info, err := findDeviceInfoWithCache(cmd.DeviceId)
		if err != nil {
			logs.Error("Device %d not found: %v", cmd.DeviceId, err)
			results = append(results, map[string]interface{}{"deviceId": cmd.DeviceId, "error": err.Error()})
			continue
		}

		// Send command
		res, err := SendGenericCommandToDevice(unifiedKey, []DeviceCommandInfo{*info}, uint16(cmd.Func), cmd.ParamsAll, true, true, 15*time.Second)
		if err != nil {
			logs.Error("Failed to send command to device %d: %v", cmd.DeviceId, err)
			results = append(results, map[string]interface{}{"deviceId": cmd.DeviceId, "error": err.Error()})
		} else {
			results = append(results, map[string]interface{}{"deviceId": cmd.DeviceId, "result": processResponse(res)})
		}
	}

	return results, nil
}
