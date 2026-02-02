package coreClass

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"port-mapping-demo/Packages/websocketClient"
	"strconv"
	"sync"

	"github.com/ghp3000/logs"
	"github.com/ghp3000/public"
	"github.com/ghp3000/utils"
	"github.com/goccy/go-json"
	"golang.org/x/image/webp"
)

func GetMiddleIdFromDeviceId(deviceId uint64) uint64 {
	mid := deviceId >> 8 //中间件授权id
	return mid
}
func GetSeatFromDeviceId(deviceId uint64) uint64 {
	seat := deviceId & 0xFF //座位号
	return seat
}
func GetDeviceIdFromMiddleIdAndSeat(mid uint64, seat uint8) uint64 {
	deviceId := (mid << 8) + uint64(seat)
	return deviceId
}
func textToUint64Auto(s string) (uint64, error) {
	// 基数设为 0，自动检测进制（如 "0x1A3" 视为十六进制）
	return strconv.ParseUint(s, 0, 64)
}
func (c *Core) DidGetPhoneInfo(deviceId uint64) (*DeviceInfo, bool) {
	value, ok := c.PhoneMap.Load(deviceId)
	if ok {
		info := value.(*DeviceInfo)
		return info, ok
	} else {
		return nil, false
	}
}
func (c *Core) DidSaveDeviceInfo(deviceId uint64, phone *DeviceInfo) {
	c.PhoneMap.Store(deviceId, phone)
}
func (c *Core) DidDelDeviceInfo(deviceId uint64) {
	c.PhoneMap.Delete(deviceId)
}
func (c *Core) GetAllDeviceInfo() (deviceIds []uint64, deviceInfos []*DeviceInfo) {
	c.PhoneMap.Range(func(key, value interface{}) bool {
		deviceId := key.(uint64)
		info := value.(*DeviceInfo)
		deviceIds = append(deviceIds, deviceId)
		deviceInfos = append(deviceInfos, info)
		return true
	})
	return deviceIds, deviceInfos
}
func (c *Core) MidGetConn(mid uint64) (*MiddleRtc, bool) {
	value, ok := c.MiddleRtcMap.Load(mid)
	if ok {
		conn := value.(*MiddleRtc)
		return conn, true
	} else {
		return nil, false
	}
}
func (c *Core) MidSaveConn(mid uint64, Mid *MiddleRtc) {
	c.MiddleRtcMap.Store(mid, Mid)
}
func (c *Core) MidDelConn(mid uint64) {
	c.MiddleRtcMap.Delete(mid)
}
func (c *Core) GetAllMid() []uint64 {
	var mids []uint64
	for _, device := range c.Devices {
		// 检查是否已经存在于 mids 中
		found := false
		for _, mid := range mids {
			if mid == device.ProxyId {
				found = true
				break
			}
		}
		// 如果不存在，则添加到 mids 中
		if !found {
			mids = append(mids, device.ProxyId)
		}
	}
	return mids
}
func (c *Core) DidGetDownloadMsg(deviceId uint64) (*DownMsgInfo, bool) {
	value, ok := c.DownloadMap.Load(deviceId)
	if ok {
		info := value.(*DownMsgInfo)
		return info, ok
	} else {
		return nil, false
	}
}
func (c *Core) DidSaveDownloadMsg(deviceId uint64, msg *DownMsgInfo) {
	c.DownloadMap.Store(deviceId, msg)
}
func (c *Core) DidDelDownloadMsg(deviceId uint64) {
	c.DownloadMap.Delete(deviceId)
}
func (c *Core) DownloadMapGetAll() (deviceIds []uint64, downloadMsgs []*DownMsgInfo) {
	c.DownloadMap.Range(func(key, value interface{}) bool {
		deviceId := key.(uint64)
		info := value.(*DownMsgInfo)
		deviceIds = append(deviceIds, deviceId)
		downloadMsgs = append(downloadMsgs, info)
		return true
	})
	return deviceIds, downloadMsgs
}
func (c *Core) SaveAppInfo(packageName string, app AppInfo) {
	c.AppList.Store(packageName, app)
}
func (c *Core) GetAppInfo(packageName string) (AppInfo, bool) {
	value, ok := c.AppList.Load(packageName)
	if ok {
		info := value.(AppInfo)
		return info, ok
	}
	return AppInfo{}, false
}
func (c *Core) DelAppInfo(packageName string) {
	c.AppList.Delete(packageName)
}
func (c *Core) GetAllAppInfo() []AppInfo {
	var appInfos []AppInfo
	c.AppList.Range(func(key, value interface{}) bool {
		info := value.(AppInfo)
		appInfos = append(appInfos, info)
		return true
	})
	return appInfos
}
func (c *Core) DelAllAppInfo() {
	c.AppList.Range(func(key, value interface{}) bool {
		c.AppList.Delete(key)
		return true
	})
}
func (c *Core) LoadConfig() *public.LoggerConfig {
	addr := filepath.Join(utils.GetAppPath(), "control", "logger.json")
	buf, err := os.ReadFile(addr)
	Cfg := new(public.LoggerConfig)
	if err == nil {
		err = json.Unmarshal(buf, Cfg)
		if err == nil {
			return Cfg
		} else {
			logs.Error(err)
		}
	}
	Cfg = &public.LoggerConfig{
		CallDepth:   2,
		Dir:         filepath.Join(utils.GetAppPath(), "control"),
		FileName:    "logger.log",
		Level:       logs.LevelError,
		Console:     false,
		FileMaxSize: 5 * logs.MB,
		FileMaxNum:  2,
		RollType:    uint8(logs.RollingFile),
		Gzip:        true,
	}
	buf, err = json.Marshal(Cfg)
	if err == nil {
		err = os.WriteFile(addr, buf, 0644)
		if err != nil {
			return Cfg
		}
	}
	return Cfg
}
func (c *Core) webp转Png(webpData []byte, quality int) ([]byte, error) {
	// 检查图片质量参数的有效性
	if quality < 1 || quality > 100 {
		quality = 85 // 设置默认质量为85
	}

	// 解码WebP数据为图片对象
	webpImg, err := webp.Decode(bytes.NewReader(webpData))
	if err != nil {
		logs.Error("WebP解码失败: %v", err)
		return nil, fmt.Errorf("WebP解码失败: %v", err)
	}

	// 创建一个buffer保存JPG数据
	buf := new(bytes.Buffer)

	// 编码为JPG图片

	//opts := jpeg.Options{
	//	Quality: quality,
	//}
	//

	err = png.Encode(buf, webpImg)
	if err != nil {
		logs.Error("png编码失败: %v", err)
		return nil, fmt.Errorf("png编码失败: %v", err)
	}

	// 返回JPG图片数据
	return buf.Bytes(), nil
}
func (c *Core) webp转jpg(webpData []byte, quality int) ([]byte, error) {
	// 检查图片质量参数的有效性
	if quality < 1 || quality > 100 {
		quality = 85 // 设置默认质量为85
	}

	// 解码WebP数据为图片对象
	webpImg, err := webp.Decode(bytes.NewReader(webpData))
	if err != nil {
		logs.Error("WebP解码失败: %v", err)
		return nil, fmt.Errorf("WebP解码失败: %v", err)
	}

	// 创建一个buffer保存JPG数据
	buf := new(bytes.Buffer)

	// 编码为JPG图片
	opts := jpeg.Options{
		Quality: quality,
	}
	err = jpeg.Encode(buf, webpImg, &opts)
	if err != nil {
		logs.Error("JPG编码失败: %v", err)
		return nil, fmt.Errorf("JPG编码失败: %v", err)
	}

	// 返回JPG图片数据
	return buf.Bytes(), nil
}
func Read通用读取数据(文件名 string) []byte {
	dir, err := os.Getwd()
	if err != nil {
		logs.Info("获取程序运行目录失败:%v", dir)
	}
	jsonData, err := os.ReadFile(dir + "/Data/" + 文件名)
	if err != nil {
		logs.Info("读取文件失败:", err)
		return nil
	}
	return jsonData
}
func (c *Core) R读取输入法包名() {
	jsonData := Read通用读取数据("输入法包名.json")
	type Package struct {
		PackageName string `json:"packageName"`
	}
	var pg Package
	err := json.Unmarshal(jsonData, &pg)
	if err != nil {
		logs.Error(err)
	}
	c.输入法包名 = pg.PackageName
	logs.Info("输入法包名:%s", c.输入法包名)
}

func NewCore() *Core {
	type Config struct {
		WsUrl            string `json:"wsUrl"`
		HttpUrl          string `json:"httpUrl"`
		SystemName       string `json:"平台名称"`
		SystemTenantName string `json:"代理商名称"`
	}
	var configret Config

	// 移除配置文件读取，改用参数传递或默认值
	// jsonData := Read通用读取数据("平台配置.json")
	// err := json.Unmarshal(jsonData, &configret)
	// if err != nil {
	// 	logs.Error(err)
	// }

	S = &Core{
		WssClient:    websocketClient.NewWSClient(configret.WsUrl),
		Devices:      []ReqPhoneInfo{},
		MiddleRtcMap: sync.Map{},
		PhoneMap:     sync.Map{},
		//增加callback
	}

	S.R系统TenantName = configret.SystemTenantName
	S.R系统名称 = configret.SystemName
	S.HttpUrl = configret.HttpUrl
	S.WsUrl = configret.WsUrl
	logs.Info("配置的系统信息:%s,%s,%s,%s", S.R系统名称, S.R系统TenantName, S.HttpUrl, S.WsUrl)
	return S
}
