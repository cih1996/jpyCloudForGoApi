package public

import "github.com/ghp3000/logs"

type Config struct {
	System    System       `mapstructure:"system" json:"system" yaml:"system"`
	LogConfig LoggerConfig `mapstructure:"LogConfig" json:"LogConfig" yaml:"LogConfig"`
}
type ConfigWithDB struct {
	System    System       `mapstructure:"system" json:"system" yaml:"system"`
	LogConfig LoggerConfig `mapstructure:"LogConfig" json:"LogConfig" yaml:"LogConfig"`
	Database  Database     `mapstructure:"database" json:"database" yaml:"database"`
}

type Database struct {
	Driver       string `mapstructure:"driver" json:"driver" yaml:"driver"`
	Server       string `mapstructure:"server" json:"server" yaml:"server"`
	Username     string `mapstructure:"username" json:"username" yaml:"username"`
	Password     string `mapstructure:"password" json:"password" yaml:"password"`
	Dbname       string `mapstructure:"dbname" json:"dbname" yaml:"dbname"`
	Config       string `mapstructure:"config" json:"config" yaml:"config"`
	MaxIdleConns int    `mapstructure:"max-idle-conns" json:"max-idle-conns" yaml:"max-idle-conns"`
	MaxOpenConns int    `mapstructure:"max-open-conns" json:"max-open-conns" yaml:"max-open-conns"`
	LogMode      int    `mapstructure:"log-mode" json:"logMode"` //mysql: 1=Silent、2=Error、3=Warn、4=Info
}

type System struct {
	Mode          string `mapstructure:"mode" json:"mode" yaml:"mode"`                               // 模式
	Host          string `mapstructure:"host" json:"host" yaml:"host"`                               // 监听地址 0.0.0.0
	Port          int    `mapstructure:"port" json:"port" yaml:"port"`                               // 端口
	Name          string `mapstructure:"name" json:"name" yaml:"name"`                               //app名称
	ReadTimeOut   int    `mapstructure:"read-timeout" json:"read-timeout" yaml:"read-timeout"`       // 读取超时
	WriterTimeOut int    `mapstructure:"writer-timeout" json:"writer-timeout" yaml:"writer-timeout"` // 写入超时
	SSLEnable     bool   `mapstructure:"ssl-enable" json:"ssl_enable" yaml:"ssl-enable"`
	SSLKey        string `mapstructure:"ssl-key" json:"ssl-key" yaml:"ssl-key"`
	SSLPem        string `mapstructure:"ssl-pem" json:"ssl-pem" yaml:"ssl-pem"`
}

type LoggerConfig struct {
	CallDepth   int        `json:"CallDepth"`
	Level       logs.LEVEL `json:"Level"`
	Console     bool       `json:"Console"`
	Dir         string     `json:"Dir"`
	FileName    string     `json:"FileName"`
	FileMaxSize int64      `json:"FileMaxSize"`
	FileMaxNum  int        `json:"FileMaxNum"`
	RollType    uint8      `json:"RollType"`
	Gzip        bool       `json:"Gzip"`
}

func (l *LoggerConfig) Setup(trim string) {
	logs.SetCallDepth(l.CallDepth)
	if l.Console {
		logs.AddAdapter(logs.NewConsoleLog(l.Level, logs.DefaultTimeFormatShort, logs.DefaultLogFormat, trim))
	} else {
		logs.RemoveAdapter("console")
	}
	log, err := logs.NewFileLog(l.Level, &logs.FileConfig{
		Dir:         l.Dir,
		FileName:    l.FileName,
		FileMaxSize: l.FileMaxSize,
		FileMaxNum:  l.FileMaxNum,
		RollType:    logs.RollingType(l.RollType),
		Gzip:        l.Gzip,
	}, 100, trim)
	if err != nil {
		logs.Fatal(err.Error())
	} else {
		logs.AddAdapter(log)
	}
}
