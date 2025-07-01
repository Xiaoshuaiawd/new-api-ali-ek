package common

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/gin-gonic/gin"
)

var (
	// Chat Completions API 请求总数
	ChatCompletionsRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chat_completions_requests_total",
			Help: "Total number of chat completions API requests",
		},
		[]string{"method", "path", "status"},
	)

	// Chat Completions API 请求成功数
	ChatCompletionsRequestsSuccess = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chat_completions_requests_success_total",
			Help: "Total number of successful chat completions API requests",
		},
		[]string{"method", "path"},
	)

	// Chat Completions API 请求失败数
	ChatCompletionsRequestsFailure = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chat_completions_requests_failure_total",
			Help: "Total number of failed chat completions API requests",
		},
		[]string{"method", "path", "status"},
	)

	// Chat Completions API 请求延迟
	ChatCompletionsRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "chat_completions_request_duration_seconds",
			Help:    "Chat completions API request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	// 渠道请求总数
	ChannelRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "channel_requests_total",
			Help: "Total number of requests per channel",
		},
		[]string{"channel_id", "channel_name", "status"},
	)

	// 渠道请求成功数
	ChannelRequestsSuccess = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "channel_requests_success_total",
			Help: "Total number of successful requests per channel",
		},
		[]string{"channel_id", "channel_name"},
	)

	// 渠道请求失败数
	ChannelRequestsFailure = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "channel_requests_failure_total",
			Help: "Total number of failed requests per channel",
		},
		[]string{"channel_id", "channel_name", "status"},
	)

	// 活跃连接数
	ActiveConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_connections",
			Help: "Number of active connections",
		},
	)

	// 渠道模型调用详情
	ChannelModelCalls = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "channel_model_calls_total",
			Help: "Total number of model calls per channel with details",
		},
		[]string{"channel_id", "channel_name", "model", "status", "error_type"},
	)

	// 渠道模型调用日志详情 - 使用Gauge以便Grafana能够看到每次调用的详细信息
	ChannelModelCallsLog = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "channel_model_calls_detail",
			Help: "Detailed model call information per channel",
		},
		[]string{"channel_id", "channel_name", "model", "status", "error_type", "error_detail", "request_id", "user_id", "token_id", "timestamp"},
	)

	// 渠道重试次数
	ChannelRetryCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "channel_retry_count_total",
			Help: "Total number of retries per channel",
		},
		[]string{"channel_id", "channel_name", "retry_reason"},
	)
)

func init() {
	// 注册指标
	prometheus.MustRegister(ChatCompletionsRequestsTotal)
	prometheus.MustRegister(ChatCompletionsRequestsSuccess)
	prometheus.MustRegister(ChatCompletionsRequestsFailure)
	prometheus.MustRegister(ChatCompletionsRequestDuration)
	prometheus.MustRegister(ChannelRequestsTotal)
	prometheus.MustRegister(ChannelRequestsSuccess)
	prometheus.MustRegister(ChannelRequestsFailure)
	prometheus.MustRegister(ActiveConnections)
	prometheus.MustRegister(ChannelModelCalls)
	prometheus.MustRegister(ChannelModelCallsLog)
	prometheus.MustRegister(ChannelRetryCount)
}

// PrometheusHandler 返回Prometheus指标处理函数
func PrometheusHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}
