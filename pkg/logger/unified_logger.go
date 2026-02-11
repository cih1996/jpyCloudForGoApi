package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ghp3000/logs"
)

// InitUnifiedLogger initializes a logger specifically for Unified Request and Main flow
func InitUnifiedLogger() error {
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	// Truncate (clear) the log file on startup
	logFile := filepath.Join(logDir, "unified_service.log")
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		return fmt.Errorf("failed to truncate log file: %v", err)
	}
	f.Close()

	return nil
}

// LogInfo writes info log to unified_service.log
func LogInfo(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	writeLog("INFO", msg)
	// Also print to standard logs for console visibility
	logs.Info(format, v...)
}

// LogError writes error log to unified_service.log
func LogError(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	writeLog("ERROR", msg)
	// Also print to standard logs for console visibility
	logs.Error(format, v...)
}

var logBroadcaster func(string)

// SetLogBroadcaster sets the function to broadcast logs to WebSocket clients
func SetLogBroadcaster(fn func(string)) {
	logBroadcaster = fn
}

func writeLog(level, msg string) {
	logDir := "logs"
	_ = os.MkdirAll(logDir, 0755)
	logFile := filepath.Join(logDir, "unified_service.log")

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Failed to write to log file: %v\n", err)
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	logLine := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level, msg)

	if _, err := f.WriteString(logLine); err != nil {
		fmt.Printf("Failed to write to log file: %v\n", err)
	}

	// Broadcast log line
	if logBroadcaster != nil {
		logBroadcaster(logLine)
	}
}
