package database

import (
	"fmt"
	"log"

	"boat/internal/config"
	"boat/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

var DB *gorm.DB

// Init 初始化 MySQL 连接并自动迁移表结构
func Init() error {
	cfg := config.Global.MySQL
	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: false, // 复数表名
		},
		Logger: logger.Default.LogMode(func() logger.LogLevel {
			if cfg.LogMode {
				return logger.Info
			}
			return logger.Silent
		}()),
	})
	if err != nil {
		return fmt.Errorf("连接 MySQL 失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)

	DB = db
	log.Println("[database] MySQL 连接成功")
	return nil
}

// AutoMigrate 自动建表
func AutoMigrate() error {
	if err := DB.AutoMigrate(model.AllModels...); err != nil {
		return fmt.Errorf("自动迁移失败: %w", err)
	}
	log.Println("[database] 表结构迁移完成")
	return nil
}
