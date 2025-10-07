package logger

import (
	"io"
	"log"
	"os"

	"github.com/sirupsen/logrus"
)

// Log 是一个全局的、配置好的 logrus 实例
var Log *logrus.Logger

// InitLogger 初始化全局的Logger实例
func InitLogger() {
	Log = logrus.New()

	// 1. 设置日志格式为JSON
	// 这样做的好处是，日志将是结构化的，非常便于后续使用ELK、Loki等工具进行分析
	Log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05", // 自定义时间格式
	})

	// 2. 设置日志输出
	// 将日志同时输出到文件和控制台
	file, err := os.OpenFile("orion_live.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("无法打开日志文件: %v", err)
	}

	// io.MultiWriter可以同时向多个Writer输出
	// 日志将同时打印在控制台(os.Stdout)和文件(file)里
	mw := io.MultiWriter(os.Stdout, file)
	Log.SetOutput(mw)

	// 3. 设置日志级别
	// 只有大于等于这个级别的日志才会输出。开发时可以是Debug，生产环境可以是Info
	Log.SetLevel(logrus.InfoLevel)

	// (可选) 开启文件名和行号追踪
	// Log.SetReportCaller(true)
}
