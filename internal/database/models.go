package database

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// ========== 设备 RPA 配置表 ==========

// DeviceStatus 设备执行状态
type DeviceStatus string

const (
	StatusIdle      DeviceStatus = "idle"      // 空闲
	StatusRunning   DeviceStatus = "running"   // 运行中
	StatusPaused    DeviceStatus = "paused"    // 暂停
	StatusError     DeviceStatus = "error"     // 错误
	StatusCompleted DeviceStatus = "completed" // 完成
)

// RunMode 运行模式
type RunMode string

const (
	ModeSingle RunMode = "single" // 单次执行
	ModeLoop   RunMode = "loop"   // 循环执行
)

// StepContext 步骤上下文（JSON 存储）
type StepContext map[string]interface{}

func (c StepContext) Value() (driver.Value, error) {
	return json.Marshal(c)
}

func (c *StepContext) Scan(value interface{}) error {
	if value == nil {
		*c = make(StepContext)
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		*c = make(StepContext)
		return nil
	}
	return json.Unmarshal(bytes, c)
}

// DeviceRpaConfig 设备 RPA 配置
type DeviceRpaConfig struct {
	ID       uint `gorm:"primaryKey" json:"id"`
	DeviceID int  `gorm:"uniqueIndex;not null" json:"deviceId"`

	// RPA 关联
	RpaID uint `gorm:"default:0" json:"rpaId"`

	// 执行状态
	Status DeviceStatus `gorm:"type:varchar(20);default:'idle'" json:"status"`
	Mode   RunMode      `gorm:"type:varchar(20);default:'single'" json:"mode"`

	// 进度追踪
	CurrentStep int         `gorm:"default:0" json:"currentStep"`
	SubStep     int         `gorm:"default:0" json:"subStep"`
	StepContext StepContext `gorm:"type:text" json:"stepContext"`

	// 流程变量（步骤间传递的数据）
	FlowVariables StepContext `gorm:"type:text" json:"flowVariables"`

	// 统计
	LoopCount    int   `gorm:"default:0" json:"loopCount"`
	SuccessCount int   `gorm:"default:0" json:"successCount"`
	FailCount    int   `gorm:"default:0" json:"failCount"`
	TotalTime    int64 `gorm:"default:0" json:"totalTime"` // 总耗时（秒）

	// 当前循环开始时间（用于计算单次耗时）
	LoopStartAt *time.Time `json:"loopStartAt"`

	// 脚本反馈（用于前端展示）
	ScriptStatus   string `gorm:"type:varchar(100)" json:"scriptStatus"`
	ScriptProgress int    `gorm:"default:0" json:"scriptProgress"`

	// 时间
	LastActiveAt *time.Time `gorm:"index" json:"lastActiveAt"`
	StartedAt    *time.Time `json:"startedAt"`
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime" json:"updatedAt"`

	// 错误信息
	LastError string `gorm:"type:text" json:"lastError"`
}

func (DeviceRpaConfig) TableName() string {
	return "device_rpa_config"
}

// ========== RPA 流程定义表 ==========

// RpaStep 单个步骤定义
type RpaStep struct {
	ID     string                 `json:"id"`     // 步骤ID
	Name   string                 `json:"name"`   // 步骤名称
	Type   string                 `json:"type"`   // 步骤类型
	Params map[string]interface{} `json:"params"` // 步骤参数
}

// RpaSteps 步骤列表（JSON 存储）
type RpaSteps []RpaStep

func (s RpaSteps) Value() (driver.Value, error) {
	return json.Marshal(s)
}

func (s *RpaSteps) Scan(value interface{}) error {
	if value == nil {
		*s = make(RpaSteps, 0)
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		*s = make(RpaSteps, 0)
		return nil
	}
	return json.Unmarshal(bytes, s)
}

// RpaFlow RPA 流程定义
type RpaFlow struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Name        string `gorm:"type:varchar(100);not null" json:"name"`
	Description string `gorm:"type:text" json:"description"`

	// 步骤定义
	Steps RpaSteps `gorm:"type:text" json:"steps"`

	// 配置
	TimeoutPerStep int `gorm:"default:300" json:"timeoutPerStep"`
	RetryCount     int `gorm:"default:0" json:"retryCount"`

	// 状态
	Enabled bool `gorm:"default:true" json:"enabled"`

	// 时间
	CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

func (RpaFlow) TableName() string {
	return "rpa_flow"
}

// ========== 执行日志表 ==========

// LogLevel 日志级别
type LogLevel string

const (
	LogInfo    LogLevel = "info"
	LogWarn    LogLevel = "warn"
	LogError   LogLevel = "error"
	LogSuccess LogLevel = "success"
)

// RpaLog 执行日志
type RpaLog struct {
	ID       uint `gorm:"primaryKey" json:"id"`
	DeviceID int  `gorm:"index;not null" json:"deviceId"`
	RpaID    uint `gorm:"index;not null" json:"rpaId"`

	// 日志内容
	StepIndex int      `gorm:"default:-1" json:"stepIndex"`
	SubStep   int      `gorm:"default:-1" json:"subStep"`
	Level     LogLevel `gorm:"type:varchar(20);default:'info'" json:"level"`
	Message   string   `gorm:"type:text;not null" json:"message"`
	Context   string   `gorm:"type:text" json:"context"`

	CreatedAt time.Time `gorm:"autoCreateTime;index" json:"createdAt"`
}

func (RpaLog) TableName() string {
	return "rpa_log"
}
