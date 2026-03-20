package logger

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ghp3000/logs"
)

const (
	deviceWSLogFile    = "devicews.log"
	deviceWSLogDir     = "logs"
	deviceWSMaxSize    = 5 * 1024 * 1024 // 5MB
	deviceWSMaxBackups = 5
)

var (
	dwsMu          sync.Mutex
	dwsBroadcaster func(string)
)

// InitDeviceWSLogger 初始化 DeviceWS 专用日志
func InitDeviceWSLogger() error {
	if err := os.MkdirAll(deviceWSLogDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %v", err)
	}
	return nil
}

// SetDeviceWSBroadcaster 设置日志广播函数（用于实时推送到前端）
func SetDeviceWSBroadcaster(fn func(string)) {
	dwsBroadcaster = fn
}

// DeviceWSInfo 写入 DeviceWS Info 日志
func DeviceWSInfo(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	writeDeviceWSLog("INFO", msg)
	logs.Info("[DeviceWS] "+format, v...)
}

// DeviceWSWarn 写入 DeviceWS Warn 日志
func DeviceWSWarn(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	writeDeviceWSLog("WARN", msg)
	logs.Warn("[DeviceWS] "+format, v...)
}

// DeviceWSError 写入 DeviceWS Error 日志
func DeviceWSError(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	writeDeviceWSLog("ERROR", msg)
	logs.Error("[DeviceWS] "+format, v...)
}

// DeviceWSDebug 写入 DeviceWS Debug 日志
func DeviceWSDebug(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	writeDeviceWSLog("DEBUG", msg)
	logs.Debug("[DeviceWS] "+format, v...)
}

func writeDeviceWSLog(level, msg string) {
	dwsMu.Lock()
	defer dwsMu.Unlock()

	logPath := filepath.Join(deviceWSLogDir, deviceWSLogFile)

	// 检查是否需要轮转
	rotateIfNeeded(logPath)

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	logLine := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level, msg)

	f.WriteString(logLine)

	if dwsBroadcaster != nil {
		dwsBroadcaster(logLine)
	}
}

// rotateIfNeeded 检查文件大小，超过阈值则轮转
func rotateIfNeeded(logPath string) {
	info, err := os.Stat(logPath)
	if err != nil || info.Size() < deviceWSMaxSize {
		return
	}

	// 删除最老的备份
	oldest := fmt.Sprintf("%s.%d", logPath, deviceWSMaxBackups)
	os.Remove(oldest)

	// 依次重命名: .4 → .5, .3 → .4, ...
	for i := deviceWSMaxBackups - 1; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", logPath, i)
		dst := fmt.Sprintf("%s.%d", logPath, i+1)
		os.Rename(src, dst)
	}

	// 当前文件 → .1
	os.Rename(logPath, logPath+".1")
}

// QueryDeviceWSLogs 查询 DeviceWS 日志
// lines: 返回行数，keyword: 过滤关键词（空则不过滤）
func QueryDeviceWSLogs(lines int, keyword string) ([]string, error) {
	logPath := filepath.Join(deviceWSLogDir, deviceWSLogFile)

	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	// 读取所有行（日志文件不会太大，5MB 以内）
	var allLines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if keyword == "" || strings.Contains(line, keyword) {
			allLines = append(allLines, line)
		}
	}

	// 取最后 N 行
	if lines > 0 && len(allLines) > lines {
		allLines = allLines[len(allLines)-lines:]
	}

	return allLines, nil
}
