package middleware

import (
	"strconv"
	"time"

	"github.com/X1Kun/orion-live/internal/metrics"
	"github.com/gin-gonic/gin"
)

// 指标中间件：1、设置为全局中间件，记录进入每个请求的时间 2、交给业务执行 3、回来后计算总耗时，以及响应的状态码 4、按照http请求路径和方法填写在响应的直方图中（请求延迟和请求数量）
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录每个http请求的开始时间
		start := time.Now()
		// 将请求交给后续的中间件或最终的Handler。程序会在这里“暂停”，等待业务处理完成
		c.Next()
		// 请求处理完毕后，计算总耗时
		duration := time.Since(start).Seconds()
		// 获取本次请求的 HTTP 响应状态码（如 200, 404, 500）
		status := strconv.Itoa(c.Writer.Status())
		// FullPath keeps metric labels bounded by using the registered route template.
		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		method := c.Request.Method
		// 根据 path 和 method 找到对应直方图，并将“耗时”输入
		metrics.HTTPRequestDuration.WithLabelValues(path, method).Observe(duration)
		// 根据 path 和 method 找到对应直方图，该请求的计数器加一
		metrics.HTTPRequestsTotal.WithLabelValues(path, method, status).Inc()
	}
}
