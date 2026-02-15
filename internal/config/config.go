package config

import (
	"encoding/json"
	"net/url"
	"os"
	"sync"

	"github.com/ghp3000/logs"
)

const ConfigFile = "config.json"

type AppConfig struct {
	WsUrl string `json:"wsUrl"`
}

var (
	cfg  *AppConfig
	lock sync.RWMutex
)

func init() {
	cfg = &AppConfig{
		WsUrl: "wss://minio.accjs.cn/ws", // Default value
	}
}

// LoadConfig loads configuration from file
func LoadConfig() {
	lock.Lock()
	defer lock.Unlock()

	data, err := os.ReadFile(ConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			logs.Info("Config file not found, using default values")
			saveConfigInternal() // Save default config
			return
		}
		logs.Error("Failed to read config file", err)
		return
	}

	err = json.Unmarshal(data, cfg)
	if err != nil {
		logs.Error("Failed to parse config file", err)
		return
	}
	logs.Info("Config loaded successfully", cfg.WsUrl)
}

// SaveConfig saves current configuration to file
func SaveConfig() error {
	lock.Lock()
	defer lock.Unlock()
	return saveConfigInternal()
}

func saveConfigInternal() error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigFile, data, 0644)
}

// GetWsUrl returns the current WsUrl safely
func GetWsUrl() string {
	lock.RLock()
	defer lock.RUnlock()
	return cfg.WsUrl
}

// SetWsUrl updates the WsUrl and saves the config
func SetWsUrl(url string) error {
	lock.Lock()
	defer lock.Unlock()
	cfg.WsUrl = url
	return saveConfigInternal()
}

// SetWsUrlMemory updates the WsUrl only in memory (for testing)
func SetWsUrlMemory(url string) {
	lock.Lock()
	defer lock.Unlock()
	cfg.WsUrl = url
}

// GetTableIP 从 WsUrl 中提取集控平台主机地址
// 例如 "wss://minio.accjs.cn/ws" → "minio.accjs.cn"
// JpyApiAgent 的 NewCore 需要纯主机地址，内部会自行拼接 wss:// 和 /ws
func GetTableIP() string {
	lock.RLock()
	defer lock.RUnlock()
	wsUrl := cfg.WsUrl
	if wsUrl == "" {
		return ""
	}
	u, err := url.Parse(wsUrl)
	if err != nil {
		// 解析失败，尝试直接返回（可能本身就是纯主机地址）
		return wsUrl
	}
	if u.Host != "" {
		return u.Host
	}
	return wsUrl
}
