package config

import (
	"net/url"
	"sync"
)

// AppConfig 纯内存配置，不再依赖 config.json 文件
// CLI 每次调用都会传入地址和 KEY，前端用 localStorage 存储
type AppConfig struct {
	WsUrl string `json:"wsUrl"`
}

var (
	cfg  *AppConfig
	lock sync.RWMutex
)

func init() {
	cfg = &AppConfig{
		WsUrl: "", // 默认为空，需要通过 Login 设置
	}
}

// LoadConfig 保留接口兼容，但不再读取文件
func LoadConfig() {
	// 不再从文件加载，纯内存模式
}

// SaveConfig 保留接口兼容，但不再写入文件
func SaveConfig() error {
	// 不再保存到文件
	return nil
}

// GetWsUrl returns the current WsUrl safely
func GetWsUrl() string {
	lock.RLock()
	defer lock.RUnlock()
	return cfg.WsUrl
}

// SetWsUrl updates the WsUrl in memory only
func SetWsUrl(wsUrl string) error {
	lock.Lock()
	defer lock.Unlock()
	cfg.WsUrl = wsUrl
	return nil
}

// GetTableIP 从 WsUrl 中提取集控平台主机地址
// 例如 "wss://minio.accjs.cn/ws" → "minio.accjs.cn"
func GetTableIP() string {
	lock.RLock()
	defer lock.RUnlock()
	wsUrl := cfg.WsUrl
	if wsUrl == "" {
		return ""
	}
	u, err := url.Parse(wsUrl)
	if err != nil {
		return wsUrl
	}
	if u.Host != "" {
		return u.Host
	}
	return wsUrl
}
