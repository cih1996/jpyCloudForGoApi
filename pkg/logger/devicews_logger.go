package logger

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ghp3000/logs"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	dwsWriter      *lumberjack.Logger
	dwsBroadcaster func(string)
)

// InitDeviceWSLogger 初始化 DeviceWS 专用日志（使用 lumberjack 轮转）
func InitDeviceWSLogger() error {
	if err := os.MkdirAll("logs", 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %v", err)
	}

	dwsWriter = &lumberjack.Logger{
		Filename:   "logs/devicews.log",
		MaxSize:    5,  // MB
		MaxBackups: 5,
		MaxAge:     30, // 天
		Compress:   false,
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
	if dwsWriter == nil {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	logLine := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level, msg)

	dwsWriter.Write([]byte(logLine))

	if dwsBroadcaster != nil {
		dwsBroadcaster(logLine)
	}
}

// QueryDeviceWSLogs 查询 DeviceWS 日志
// lines: 返回行数，keyword: 过滤关键词（空则不过滤）
func QueryDeviceWSLogs(lines int, keyword string) ([]string, error) {
	logPath := filepath.Join("logs", "devicews.log")

	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var allLines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if keyword == "" || strings.Contains(line, keyword) {
			allLines = append(allLines, line)
		}
	}

	if lines > 0 && len(allLines) > lines {
		allLines = allLines[len(allLines)-lines:]
	}

	return allLines, nil
}
