package database

import (
	"errors"
	"os"
	"path/filepath"
	"sync"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	db          *gorm.DB
	once        sync.Once
	initialized bool
)

// Init 初始化数据库连接
func Init(dataDir string) error {
	var initErr error
	once.Do(func() {
		// 确保数据目录存在
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			initErr = err
			return
		}

		dbPath := filepath.Join(dataDir, "rpa.db")
		var err error
		db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		})
		if err != nil {
			initErr = err
			return
		}

		// 自动迁移表结构
		if err := db.AutoMigrate(
			&DeviceRpaConfig{},
			&RpaFlow{},
			&RpaLog{},
		); err != nil {
			initErr = err
			return
		}

		initialized = true
	})
	return initErr
}

// IsInitialized 检查数据库是否已初始化
func IsInitialized() bool {
	return initialized && db != nil
}

// DB 获取数据库实例
func DB() *gorm.DB {
	return db
}

// ErrNotInitialized 数据库未初始化错误
var ErrNotInitialized = errors.New("database not initialized")
