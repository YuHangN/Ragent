package db

import (
	"fmt"
	"log"
	"time"

	"github.com/YuHangN/ragent-go/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewMySQL 负责初始化 MySQL 连接，并返回全局可复用的 *gorm.DB。
func NewMySQL(cfg *config.DBConfig) *gorm.DB {
	// 先通过 GORM 打开连接, mysql.Open(cfg.DSN) 表示使用 MySQL 驱动，并传入连接字符串。
	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{
		// 设置日志级别
		Logger:                                   logger.Default.LogMode(logger.Info),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		log.Fatalf("连接 MySQL 失败: %v", err)
	}

	// 获取底层的 *sql.DB 来设置连接池参数。
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("获取底层 *sql.DB 失败: %v", err)
	}

	// 设置连接池参数
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	// 测试连接是否成功
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("测试 MySQL 连接失败: %v", err)
	}

	fmt.Println("成功连接到 MySQL 数据库")
	return db
}
