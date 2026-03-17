package main

import (
	"encoding/json"
	"fmt"
	"time"

	"adminApi/userDeviceCtl"
	"port-mapping-demo/internal/config"
	"port-mapping-demo/internal/service"

	"github.com/ghp3000/logs"
	"github.com/ghp3000/public"
)

// PlatformKey 平台Key，请在此处填入或通过环境变量获取
var PlatformKey = "10002ac0e042f7331325739102ef2c489d5ae1770013344585"
var host = "114.67.244.162"

func main() {
	logs.SetLevel("INFO", logs.LevelInfo)

	if PlatformKey == "YOUR_PLATFORM_KEY_HERE" {
		logs.Error("Please set PlatformKey in test/middleTest.go")
		return
	}

	// 0. 初始化配置并临时修改 Host
	config.LoadConfig()
	originalUrl := config.GetWsUrl()
	testUrl := fmt.Sprintf("wss://%s/ws", host)
	logs.Info("Setting temporary WS URL: %s", testUrl)
	config.SetWsUrlMemory(testUrl)

	// 恢复配置
	defer func() {
		logs.Info("Restoring original WS URL: %s", originalUrl)
		config.SetWsUrlMemory(originalUrl)
	}()

	// 1. 初始化 AdminApi 并登录 (通过 service.EnsureLogin)
	if err := service.EnsureLogin(PlatformKey, ""); err != nil {
		logs.Error("Login failed: %v", err)
		return
	}

	// 复用 GlobalApi
	api := service.GetGlobalApi()
	if api == nil {
		logs.Error("GlobalApi is nil")
		return
	}

	// 2. 获取设备列表
	devicesRes, errApi := api.UserDeviceCtl.GetUserDeviceList(&userDeviceCtl.GetUserDeviceListReq{
		PageNum:  0,
		PageSize: 100,
	})
	if errApi != nil {
		logs.Error("Get device list failed: %v", errApi)
		return
	}

	if len(devicesRes.Records) == 0 {
		logs.Error("No devices found")
		return
	}

	// 3. 对设备发送命令 (批量模式)
	logs.Info("开始批量发送 Shell 命令...")

	// 3.1 构造所有设备信息
	var targetDevs []service.DeviceCommandInfo
	for _, device := range devicesRes.Records {
		targetDevs = append(targetDevs, service.DeviceCommandInfo{
			DeviceId:            uint64(device.DeviceInfo.DeviceId),
			TbProxyId:           uint64(device.DeviceInfo.TbProxyId),
			TbYunJiUserDeviceId: uint64(device.TbYunJiUserDeviceId),
		})
	}
	// 只测试第一个设备
	if len(targetDevs) > 0 {
		targetDevs = targetDevs[:1]
	}

	// 3.2 第一次调用: Shell 命令
	cmdData := map[string]interface{}{
		"shell": "pm list package",
	}
	res, err := service.SendGenericCommandToDevice(PlatformKey, targetDevs, 289, cmdData, true, true, 20*time.Second)
	if err != nil {
		logs.Error("Shell 命令执行失败: %v", err)
	} else {
		if msg, ok := res.(*public.Message); ok {
			var rawData interface{}
			if err := msg.Unmarshal(&rawData); err == nil {
				jsonBytes, _ := json.Marshal(rawData)
				logs.Info("Shell 命令执行成功 (Raw): %s", string(jsonBytes))
			} else {
				logs.Info("Shell 命令执行成功 (解码失败): %v, err: %v", res, err)
			}
		} else {
			logs.Info("Shell 命令执行成功: %v", res)
		}
	}

	// 2. 循环发送 Shell 命令 (测试复用与稳定性)
	logs.Info("开始循环发送 Shell 命令 (测试复用)...")

	for i := 0; i < 5; i++ {
		logs.Info("=== 第 %d 次循环 ===", i+1)
		shellCmd2 := map[string]interface{}{
			"shell": fmt.Sprintf("echo 'Loop %d'; date", i+1),
		}
		res2, err2 := service.SendGenericCommandToDevice(PlatformKey, targetDevs, 289, shellCmd2, true, true, 10*time.Second)
		if err2 != nil {
			logs.Error("第 %d 次 Shell 命令执行失败: %v", i+1, err2)
		} else {
			if msg, ok := res2.(*public.Message); ok {
				var rawData interface{}
				if err := msg.Unmarshal(&rawData); err == nil {
					jsonBytes, _ := json.Marshal(rawData)
					logs.Info("第 %d 次 Shell 命令执行成功 (Raw): %s", i+1, string(jsonBytes))
				} else {
					logs.Info("第 %d 次 Shell 命令执行成功 (解码失败): %v, err: %v", i+1, res2, err)
				}
			} else {
				logs.Info("第 %d 次 Shell 命令执行成功: %v", i+1, res2)
			}
		}
		// 稍微间隔一下
		time.Sleep(1 * time.Second)
	}

	logs.Info("测试结束")
	// 保持程序运行一会以便观察日志
	time.Sleep(2 * time.Second)
}
