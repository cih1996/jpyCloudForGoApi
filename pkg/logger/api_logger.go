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
	apiWriter      *lumberjack.Logger
	apiBroadcaster func(string)
)

// InitAPILogger 初始化 API 能力层专用日志（使用 lumberjack 轮转）
func InitAPILogger() error {
	if err := os.MkdirAll("logs", 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %v", err)
	}

	apiWriter = &lumberjack.Logger{
		Filename:   "logs/api.log",
		MaxSize:    5,  // MB
		MaxBackups: 5,
		MaxAge:     30, // 天
		Compress:   false,
	}
	return nil
}

// SetAPIBroadcaster 设置 API 日志广播函数（用于实时推送到前端）
func SetAPIBroadcaster(fn func(string)) {
	apiBroadcaster = fn
}

// APIInfo 写入 API Info 日志
func APIInfo(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	writeAPILog("INFO", msg)
	logs.Info("[API] "+format, v...)
}

// APIWarn 写入 API Warn 日志
func APIWarn(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	writeAPILog("WARN", msg)
	logs.Warn("[API] "+format, v...)
}

// APIError 写入 API Error 日志
func APIError(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	writeAPILog("ERROR", msg)
	logs.Error("[API] "+format, v...)
}

func writeAPILog(level, msg string) {
	if apiWriter == nil {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	logLine := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level, msg)

	apiWriter.Write([]byte(logLine))

	if apiBroadcaster != nil {
		apiBroadcaster(logLine)
	}
}

// QueryAPILogs 查询 API 日志
func QueryAPILogs(lines int, keyword string) ([]string, error) {
	logPath := filepath.Join("logs", "api.log")

	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	allLines := make([]string, 0)
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
