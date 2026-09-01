package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// 使用 promauto 自动注册指标，可以简化代码
var (
	// HTTPRequestsTotal: 一个计数器(Counter)，用于统计HTTP请求的总数。
	// 它是一个向量(Vec)，可以通过标签(label)来区分不同的维度。
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orion_http_requests_total", // 指标名，最好带上项目前缀
			Help: "Total number of HTTP requests.",
		},
		[]string{"path", "method", "status"}, // 按路径、方法、状态码分类
	)

	// HTTPRequestDuration: 一个直方图(Histogram)，用于观察请求延迟的分布。
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "orion_http_request_duration_seconds",
			Help: "Histogram of HTTP request durations.",
			// Buckets定义了延迟的区间划分，用于统计落在各区间的请求数
			// 比如 {0.1, 0.5, 1, 5} 表示统计 <0.1s, 0.1-0.5s, 0.5-1s... 的请求数
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"path", "method"},
	)
)
