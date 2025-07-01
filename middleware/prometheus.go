package middleware

import (
	"fmt"
	"one-api/common"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// PrometheusMiddleware 用于收集请求指标的中间件
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 开始时间
		startTime := time.Now()

		// 检查是否为需要监控的接口
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		// 只监控 /v1/chat/completions 接口
		shouldMonitor := strings.HasSuffix(path, "/v1/chat/completions")

		// 如果需要监控，增加活跃连接数
		if shouldMonitor {
			common.ActiveConnections.Inc()
		}

		// 处理请求
		c.Next()

		// 如果不需要监控，直接返回
		if !shouldMonitor {
			return
		}

		// 减少活跃连接数
		common.ActiveConnections.Dec()

		// 请求结束时间
		duration := time.Since(startTime).Seconds()

		// 获取请求信息
		status := c.Writer.Status()
		method := c.Request.Method

		// 固定路径为 /v1/chat/completions
		monitorPath := "/v1/chat/completions"

		// 记录 Chat Completions API 请求指标
		common.ChatCompletionsRequestsTotal.WithLabelValues(method, monitorPath, strconv.Itoa(status)).Inc()
		common.ChatCompletionsRequestDuration.WithLabelValues(method, monitorPath, strconv.Itoa(status)).Observe(duration)

		// 根据状态码区分成功和失败请求
		if status >= 200 && status < 400 {
			common.ChatCompletionsRequestsSuccess.WithLabelValues(method, monitorPath).Inc()
		} else {
			common.ChatCompletionsRequestsFailure.WithLabelValues(method, monitorPath, strconv.Itoa(status)).Inc()
		}

		// 获取渠道信息并记录渠道指标
		channelId := c.GetInt("channel_id")
		channelName := c.GetString("channel_name")
		channelStatus := c.GetInt("channel_status")

		if channelId > 0 && channelName != "" {
			// 获取渠道请求的实际状态码，优先使用channel_status，如果没有则使用响应状态码
			if channelStatus == 0 {
				// 如果没有设置channel_status，使用API状态码
				channelStatus = status
			}

			// 记录渠道请求指标
			channelIdStr := fmt.Sprintf("%d", channelId)
			channelStatusStr := strconv.Itoa(channelStatus)

			// 记录总请求数（用于状态码分布图）
			common.ChannelRequestsTotal.WithLabelValues(channelIdStr, channelName, channelStatusStr).Inc()

			// 记录成功/失败请求数（用于成功率计算）
			if channelStatus >= 200 && channelStatus < 400 {
				common.ChannelRequestsSuccess.WithLabelValues(channelIdStr, channelName).Inc()
			} else {
				common.ChannelRequestsFailure.WithLabelValues(channelIdStr, channelName, channelStatusStr).Inc()
			}

			// 记录渠道模型调用详情
			model := c.GetString("request_model")
			if model == "" {
				model = "unknown"
			}

			errorType := ""
			errorMessage := ""
			if channelStatus >= 400 {
				// 获取错误类型和详细错误信息
				if errorMsg := c.GetString("error_message"); errorMsg != "" {
					errorType = "api_error"
					errorMessage = errorMsg
				} else {
					errorType = fmt.Sprintf("http_%d", channelStatus)
					errorMessage = fmt.Sprintf("HTTP %d Error", channelStatus)
				}
			} else {
				errorType = "success"
				errorMessage = "success"
			}

			// 记录统计数据
			common.ChannelModelCalls.WithLabelValues(channelIdStr, channelName, model, channelStatusStr, errorType).Inc()

			// 记录详细调用日志
			requestId := c.GetString(common.RequestIdKey)
			if requestId == "" {
				requestId = "unknown"
			}

			userId := fmt.Sprintf("%d", c.GetInt("user_id"))
			tokenId := fmt.Sprintf("%d", c.GetInt("token_id"))

			// 限制错误信息长度，避免标签过长
			if len(errorMessage) > 100 {
				errorMessage = errorMessage[:100] + "..."
			}

			// 生成时间戳标签（填充零的毫秒时间戳，确保字符串排序正确）
			timestamp := fmt.Sprintf("%013d", time.Now().UnixMilli())

			common.ChannelModelCallsLog.WithLabelValues(
				channelIdStr,
				channelName,
				model,
				channelStatusStr,
				errorType,
				errorMessage,
				requestId,
				userId,
				tokenId,
				timestamp,
			).Inc()
		}
	}
}
