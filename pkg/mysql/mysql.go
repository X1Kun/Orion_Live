package mysql

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// 初始化GORM的数据库连接：1、从.env文件中加载环境变量 2、构建 DSN (Data Source Name) 3、打开数据库连接
func InitMySQL() (*gorm.DB, error) {
	// 读取项目根目录下的 .env 文件，加载到当前程序的环境变量中
	_ = godotenv.Load()

	user := os.Getenv("DB_USER")
	if user == "" {
		user = "root"
	}
	password := os.Getenv("DB_PASSWORD")
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "3306"
	}
	dbname := os.Getenv("DB_NAME")
	if dbname == "" {
		return nil, fmt.Errorf("DB_NAME 环境变量未设置") // 数据库名通常是必须的
	}
	// 设置字符集为 utf8mb4，将数据库中的 DATE 和 DATETIME 类型自动解析为 Go 的 time.Time
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, password, host, port, dbname)

	// 慢查询日志中间件
	newLogger := gormlogger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		gormlogger.Config{
			SlowThreshold: 100 * time.Millisecond,
			LogLevel:      gormlogger.Info,
			Colorful:      true,
		},
	)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: newLogger})
	// 默认配置
	// db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("无法连接到MySQL at %s: %w", host, err)
	}

	return db, nil
}
