package logger

import (
	"io"
	"log"
	"os"

	"github.com/sirupsen/logrus"
)

// Log 是一个全局的、配置好的 logrus 实例
var Log *logrus.Logger

// InitLogger 初始化全局的Logger实例：1、初始化创建实例 2、设置log输出为JSON格式，以及时间戳样式 3、设置目的地，将日志同时输出到文件和控制台 4、设置过滤规则
func InitLogger() {

	Log = logrus.New()
	Log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05", // 自定义时间格式
	})
	// 无则创建，只写+追加模式
	// 666：文件所有者，文件所属用户组，其他人的权限：都是可读可写
	file, err := os.OpenFile("orion_live.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("无法打开日志文件: %v", err)
	}
	// io.MultiWriter可以同时向多个Writer输出，日志将同时打印在控制台(os.Stdout)和文件(file)里
	mw := io.MultiWriter(os.Stdout, file)
	Log.SetOutput(mw)
	// 只有大于等于这个级别的日志才会输出。开发时可以是Debug，生产环境可以是Info
	Log.SetLevel(logrus.InfoLevel)
}
