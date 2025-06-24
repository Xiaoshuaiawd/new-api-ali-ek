package controller

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"one-api/common"
	constant2 "one-api/constant"
	"one-api/dto"
	"one-api/middleware"
	"one-api/model"
	"one-api/relay"
	"one-api/relay/constant"
	relayconstant "one-api/relay/constant"
	"one-api/relay/helper"
	"one-api/service"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func relayHandler(c *gin.Context, relayMode int) *dto.OpenAIErrorWithStatusCode {
	var err *dto.OpenAIErrorWithStatusCode
	switch relayMode {
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		err = relay.ImageHelper(c)
	case relayconstant.RelayModeAudioSpeech:
		fallthrough
	case relayconstant.RelayModeAudioTranslation:
		fallthrough
	case relayconstant.RelayModeAudioTranscription:
		err = relay.AudioHelper(c)
	case relayconstant.RelayModeRerank:
		err = relay.RerankHelper(c, relayMode)
	case relayconstant.RelayModeEmbeddings:
		err = relay.EmbeddingHelper(c)
	case relayconstant.RelayModeResponses:
		err = relay.ResponsesHelper(c)
	case relayconstant.RelayModeGemini:
		err = relay.GeminiHelper(c)
	default:
		err = relay.TextHelper(c)
	}

	return err
}

func Relay(c *gin.Context) {
	relayMode := constant.Path2RelayMode(c.Request.URL.Path)
	requestId := c.GetString(common.RequestIdKey)
	group := c.GetString("group")
	originalModel := c.GetString("original_model")
	var openaiErr *dto.OpenAIErrorWithStatusCode

	for i := 0; i <= common.RetryTimes; i++ {
		channel, err := getChannel(c, group, originalModel, i)
		if err != nil {
			common.LogError(c, err.Error())
			openaiErr = service.OpenAIErrorWrapperLocal(err, "get_channel_failed", http.StatusInternalServerError)
			break
		}

		// 检查RPM限制
		if service.CheckChannelRPMLimit(channel.Id) {
			common.LogInfo(c, fmt.Sprintf("渠道 #%d RPM限制超限，切换到其他渠道", channel.Id))
			// 记录RPM限制导致的重试
			if i > 0 {
				common.ChannelRetryCount.WithLabelValues(fmt.Sprintf("%d", channel.Id), channel.Name, "rpm_limit").Inc()
			}
			continue // 跳过此渠道，尝试下一个
		}

		// 记录RPM使用次数
		service.IncrementChannelRPMUsage(channel.Id)

		openaiErr = relayRequest(c, relayMode, channel)

		if openaiErr == nil {
			return // 成功处理请求，直接返回
		}

		// 检查是否需要立即禁用渠道并继续重试
		if service.ShouldImmediatelyDisableAndRetry(openaiErr) && channel.GetAutoBan() {
			// 立即禁用渠道
			service.DisableChannel(channel.Id, channel.Name, fmt.Sprintf("立即禁用 - 状态码: %d, 错误: %s", openaiErr.StatusCode, openaiErr.Error.Message))
			common.LogError(c, fmt.Sprintf("立即禁用渠道 #%d，状态码: %d，继续重试其他渠道", channel.Id, openaiErr.StatusCode))
			// 记录立即禁用导致的重试
			if i > 0 {
				retryReason := getRetryReason(openaiErr)
				common.ChannelRetryCount.WithLabelValues(fmt.Sprintf("%d", channel.Id), channel.Name, retryReason+"_disabled").Inc()
			}
			// 继续重试其他渠道
			continue
		}

		go processChannelError(c, channel.Id, channel.Type, channel.Name, channel.GetAutoBan(), openaiErr)

		if !shouldRetry(c, openaiErr, common.RetryTimes-i) {
			break
		}

		// 记录重试次数 (从第二次请求开始记录重试)
		if i > 0 {
			retryReason := getRetryReason(openaiErr)
			common.ChannelRetryCount.WithLabelValues(fmt.Sprintf("%d", channel.Id), channel.Name, retryReason).Inc()
		}
	}
	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		common.LogInfo(c, retryLogStr)
	}

	// 如果重试成功（没有错误），也要显示重试过程
	if openaiErr == nil && len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试成功：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		common.LogInfo(c, retryLogStr)
	}

	if openaiErr != nil {
		if openaiErr.StatusCode == http.StatusTooManyRequests {
			common.LogError(c, fmt.Sprintf("origin 429 error: %s", openaiErr.Error.Message))
			openaiErr.Error.Message = "当前分组上游负载已饱和，请稍后再试"
		}

		// 只有在最终失败时才记录错误日志（重试成功的不算错误）
		// 空回复错误不记录到错误日志中
		if constant2.ErrorLogEnabled && openaiErr.Error.Type != "empty_response" {
			userId := c.GetInt("id")
			tokenName := c.GetString("token_name")
			modelName := c.GetString("original_model")
			tokenId := c.GetInt("token_id")
			userGroup := c.GetString("group")
			channelId := c.GetInt("channel_id")
			other := make(map[string]interface{})
			other["error_type"] = openaiErr.Error.Type
			other["error_code"] = openaiErr.Error.Code
			other["status_code"] = openaiErr.StatusCode
			other["channel_id"] = channelId
			other["channel_name"] = c.GetString("channel_name")
			other["channel_type"] = c.GetInt("channel_type")
			// 记录重试信息
			useChannel := c.GetStringSlice("use_channel")
			if len(useChannel) > 1 {
				other["retry_channels"] = strings.Join(useChannel, "->")
			}

			model.RecordErrorLog(c, userId, channelId, modelName, tokenName, openaiErr.Error.Message, tokenId, 0, false, userGroup, other)
		}

		openaiErr.Error.Message = common.MessageWithRequestId(openaiErr.Error.Message, requestId)
		c.JSON(openaiErr.StatusCode, gin.H{
			"error": openaiErr.Error,
		})
	}
}

var upgrader = websocket.Upgrader{
	Subprotocols: []string{"realtime"}, // WS 握手支持的协议，如果有使用 Sec-WebSocket-Protocol，则必须在此声明对应的 Protocol TODO add other protocol
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许跨域
	},
}

func WssRelay(c *gin.Context) {
	// 将 HTTP 连接升级为 WebSocket 连接

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	defer ws.Close()

	if err != nil {
		openaiErr := service.OpenAIErrorWrapper(err, "get_channel_failed", http.StatusInternalServerError)
		helper.WssError(c, ws, openaiErr.Error)
		return
	}

	relayMode := constant.Path2RelayMode(c.Request.URL.Path)
	requestId := c.GetString(common.RequestIdKey)
	group := c.GetString("group")
	//wss://api.openai.com/v1/realtime?model=gpt-4o-realtime-preview-2024-10-01
	originalModel := c.GetString("original_model")
	var openaiErr *dto.OpenAIErrorWithStatusCode

	for i := 0; i <= common.RetryTimes; i++ {
		channel, err := getChannel(c, group, originalModel, i)
		if err != nil {
			common.LogError(c, err.Error())
			openaiErr = service.OpenAIErrorWrapperLocal(err, "get_channel_failed", http.StatusInternalServerError)
			break
		}

		// 检查RPM限制
		if service.CheckChannelRPMLimit(channel.Id) {
			common.LogInfo(c, fmt.Sprintf("渠道 #%d RPM限制超限，切换到其他渠道", channel.Id))
			// 记录RPM限制导致的重试
			if i > 0 {
				common.ChannelRetryCount.WithLabelValues(fmt.Sprintf("%d", channel.Id), channel.Name, "rpm_limit").Inc()
			}
			continue // 跳过此渠道，尝试下一个
		}

		// 记录RPM使用次数
		service.IncrementChannelRPMUsage(channel.Id)

		openaiErr = wssRequest(c, ws, relayMode, channel)

		if openaiErr == nil {
			return // 成功处理请求，直接返回
		}

		// 检查是否需要立即禁用渠道并继续重试
		if service.ShouldImmediatelyDisableAndRetry(openaiErr) && channel.GetAutoBan() {
			// 立即禁用渠道
			service.DisableChannel(channel.Id, channel.Name, fmt.Sprintf("立即禁用 - 状态码: %d, 错误: %s", openaiErr.StatusCode, openaiErr.Error.Message))
			common.LogError(c, fmt.Sprintf("立即禁用渠道 #%d，状态码: %d，继续重试其他渠道", channel.Id, openaiErr.StatusCode))
			// 记录立即禁用导致的重试
			if i > 0 {
				retryReason := getRetryReason(openaiErr)
				common.ChannelRetryCount.WithLabelValues(fmt.Sprintf("%d", channel.Id), channel.Name, retryReason+"_disabled").Inc()
			}
			// 继续重试其他渠道
			continue
		}

		go processChannelError(c, channel.Id, channel.Type, channel.Name, channel.GetAutoBan(), openaiErr)

		if !shouldRetry(c, openaiErr, common.RetryTimes-i) {
			break
		}

		// 记录重试次数 (从第二次请求开始记录重试)
		if i > 0 {
			retryReason := getRetryReason(openaiErr)
			common.ChannelRetryCount.WithLabelValues(fmt.Sprintf("%d", channel.Id), channel.Name, retryReason).Inc()
		}
	}
	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		common.LogInfo(c, retryLogStr)
	}

	// 如果重试成功（没有错误），也要显示重试过程
	if openaiErr == nil && len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试成功：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		common.LogInfo(c, retryLogStr)
	}

	if openaiErr != nil {
		if openaiErr.StatusCode == http.StatusTooManyRequests {
			openaiErr.Error.Message = "当前分组上游负载已饱和，请稍后再试"
		}

		// 只有在最终失败时才记录错误日志（重试成功的不算错误）
		// 空回复错误不记录到错误日志中
		if constant2.ErrorLogEnabled && openaiErr.Error.Type != "empty_response" {
			userId := c.GetInt("id")
			tokenName := c.GetString("token_name")
			modelName := c.GetString("original_model")
			tokenId := c.GetInt("token_id")
			userGroup := c.GetString("group")
			channelId := c.GetInt("channel_id")
			other := make(map[string]interface{})
			other["error_type"] = openaiErr.Error.Type
			other["error_code"] = openaiErr.Error.Code
			other["status_code"] = openaiErr.StatusCode
			other["channel_id"] = channelId
			other["channel_name"] = c.GetString("channel_name")
			other["channel_type"] = c.GetInt("channel_type")
			// 记录重试信息
			useChannel := c.GetStringSlice("use_channel")
			if len(useChannel) > 1 {
				other["retry_channels"] = strings.Join(useChannel, "->")
			}

			model.RecordErrorLog(c, userId, channelId, modelName, tokenName, openaiErr.Error.Message, tokenId, 0, false, userGroup, other)
		}

		openaiErr.Error.Message = common.MessageWithRequestId(openaiErr.Error.Message, requestId)
		helper.WssError(c, ws, openaiErr.Error)
	}
}

func RelayClaude(c *gin.Context) {
	//relayMode := constant.Path2RelayMode(c.Request.URL.Path)
	requestId := c.GetString(common.RequestIdKey)
	group := c.GetString("group")
	originalModel := c.GetString("original_model")
	var claudeErr *dto.ClaudeErrorWithStatusCode

	for i := 0; i <= common.RetryTimes; i++ {
		channel, err := getChannel(c, group, originalModel, i)
		if err != nil {
			common.LogError(c, err.Error())
			claudeErr = service.ClaudeErrorWrapperLocal(err, "get_channel_failed", http.StatusInternalServerError)
			break
		}

		// 检查RPM限制
		if service.CheckChannelRPMLimit(channel.Id) {
			common.LogInfo(c, fmt.Sprintf("渠道 #%d RPM限制超限，切换到其他渠道", channel.Id))
			// 记录RPM限制导致的重试
			if i > 0 {
				common.ChannelRetryCount.WithLabelValues(fmt.Sprintf("%d", channel.Id), channel.Name, "rpm_limit").Inc()
			}
			continue // 跳过此渠道，尝试下一个
		}

		// 记录RPM使用次数
		service.IncrementChannelRPMUsage(channel.Id)

		claudeErr = claudeRequest(c, channel)

		if claudeErr == nil {
			return // 成功处理请求，直接返回
		}

		openaiErr := service.ClaudeErrorToOpenAIError(claudeErr)

		// 检查是否需要立即禁用渠道并继续重试
		if service.ShouldImmediatelyDisableAndRetry(openaiErr) && channel.GetAutoBan() {
			// 立即禁用渠道
			service.DisableChannel(channel.Id, channel.Name, fmt.Sprintf("立即禁用 - 状态码: %d, 错误: %s", openaiErr.StatusCode, openaiErr.Error.Message))
			common.LogError(c, fmt.Sprintf("立即禁用渠道 #%d，状态码: %d，继续重试其他渠道", channel.Id, openaiErr.StatusCode))
			// 记录立即禁用导致的重试
			if i > 0 {
				retryReason := getRetryReason(openaiErr)
				common.ChannelRetryCount.WithLabelValues(fmt.Sprintf("%d", channel.Id), channel.Name, retryReason+"_disabled").Inc()
			}
			// 继续重试其他渠道
			continue
		}

		go processChannelError(c, channel.Id, channel.Type, channel.Name, channel.GetAutoBan(), openaiErr)

		if !shouldRetry(c, openaiErr, common.RetryTimes-i) {
			break
		}

		// 记录重试次数 (从第二次请求开始记录重试)
		if i > 0 {
			retryReason := getRetryReason(openaiErr)
			common.ChannelRetryCount.WithLabelValues(fmt.Sprintf("%d", channel.Id), channel.Name, retryReason).Inc()
		}
	}
	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		common.LogInfo(c, retryLogStr)
	}

	// 如果重试成功（没有错误），也要显示重试过程
	if claudeErr == nil && len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试成功：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		common.LogInfo(c, retryLogStr)
	}

	if claudeErr != nil {
		// 只有在最终失败时才记录错误日志（重试成功的不算错误）
		// 空回复错误不记录到错误日志中
		if constant2.ErrorLogEnabled {
			// 将Claude错误转换为OpenAI错误格式进行检查
			openaiErr := service.ClaudeErrorToOpenAIError(claudeErr)
			if openaiErr.Error.Type != "empty_response" {
				userId := c.GetInt("id")
				tokenName := c.GetString("token_name")
				modelName := c.GetString("original_model")
				tokenId := c.GetInt("token_id")
				userGroup := c.GetString("group")
				channelId := c.GetInt("channel_id")
				other := make(map[string]interface{})
				other["error_type"] = claudeErr.Error.Type
				other["error_code"] = "claude_error"
				other["status_code"] = claudeErr.StatusCode
				other["channel_id"] = channelId
				other["channel_name"] = c.GetString("channel_name")
				other["channel_type"] = c.GetInt("channel_type")
				// 记录重试信息
				useChannel := c.GetStringSlice("use_channel")
				if len(useChannel) > 1 {
					other["retry_channels"] = strings.Join(useChannel, "->")
				}

				model.RecordErrorLog(c, userId, channelId, modelName, tokenName, claudeErr.Error.Message, tokenId, 0, false, userGroup, other)
			}
		}

		claudeErr.Error.Message = common.MessageWithRequestId(claudeErr.Error.Message, requestId)
		c.JSON(claudeErr.StatusCode, gin.H{
			"type":  "error",
			"error": claudeErr.Error,
		})
	}
}

func relayRequest(c *gin.Context, relayMode int, channel *model.Channel) *dto.OpenAIErrorWithStatusCode {
	addUsedChannel(c, channel.Id)
	requestBody, _ := common.GetRequestBody(c)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
	return relayHandler(c, relayMode)
}

func wssRequest(c *gin.Context, ws *websocket.Conn, relayMode int, channel *model.Channel) *dto.OpenAIErrorWithStatusCode {
	addUsedChannel(c, channel.Id)
	requestBody, _ := common.GetRequestBody(c)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
	return relay.WssHelper(c, ws)
}

func claudeRequest(c *gin.Context, channel *model.Channel) *dto.ClaudeErrorWithStatusCode {
	addUsedChannel(c, channel.Id)
	requestBody, _ := common.GetRequestBody(c)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
	return relay.ClaudeHelper(c)
}

func addUsedChannel(c *gin.Context, channelId int) {
	useChannel := c.GetStringSlice("use_channel")
	useChannel = append(useChannel, fmt.Sprintf("%d", channelId))
	c.Set("use_channel", useChannel)
}

func getChannel(c *gin.Context, group, originalModel string, retryCount int) (*model.Channel, error) {
	if retryCount == 0 {
		autoBan := c.GetBool("auto_ban")
		autoBanInt := 1
		if !autoBan {
			autoBanInt = 0
		}
		return &model.Channel{
			Id:      c.GetInt("channel_id"),
			Type:    c.GetInt("channel_type"),
			Name:    c.GetString("channel_name"),
			AutoBan: &autoBanInt,
		}, nil
	}
	channel, _, err := model.CacheGetRandomSatisfiedChannel(c, group, originalModel, retryCount)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("获取重试渠道失败: %s", err.Error()))
	}
	middleware.SetupContextForSelectedChannel(c, channel, originalModel)
	return channel, nil
}

func shouldRetry(c *gin.Context, openaiErr *dto.OpenAIErrorWithStatusCode, retryTimes int) bool {
	if openaiErr == nil {
		return false
	}
	if openaiErr.LocalError {
		return false
	}
	if retryTimes <= 0 {
		return false
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		return false
	}

	// 特殊处理：空回复错误需要静默重试
	if openaiErr.Error.Type == "empty_response" {
		return true
	}

	// 2xx 状态码表示成功，不需要重试
	if openaiErr.StatusCode/100 == 2 {
		return false
	}
	// 对于超时相关的状态码，不重试（保持原有逻辑）
	if openaiErr.StatusCode == 504 || openaiErr.StatusCode == 524 || openaiErr.StatusCode == 408 {
		return false
	}

	// 所有其他错误都重试
	return true
}

func processChannelError(c *gin.Context, channelId int, channelType int, channelName string, autoBan bool, err *dto.OpenAIErrorWithStatusCode) {
	// 不要使用context获取渠道信息，异步处理时可能会出现渠道信息不一致的情况
	// do not use context to get channel info, there may be inconsistent channel info when processing asynchronously

	// 空回复错误不记录日志，直接返回
	if err.Error.Type == "empty_response" {
		return
	}

	common.LogError(c, fmt.Sprintf("relay error (channel #%d, status code: %d): %s", channelId, err.StatusCode, err.Error.Message))
	if service.ShouldDisableChannel(channelType, err) && autoBan {
		service.DisableChannel(channelId, channelName, err.Error.Message)
	}
}

func RelayMidjourney(c *gin.Context) {
	relayMode := c.GetInt("relay_mode")
	var err *dto.MidjourneyResponse
	switch relayMode {
	case relayconstant.RelayModeMidjourneyNotify:
		err = relay.RelayMidjourneyNotify(c)
	case relayconstant.RelayModeMidjourneyTaskFetch, relayconstant.RelayModeMidjourneyTaskFetchByCondition:
		err = relay.RelayMidjourneyTask(c, relayMode)
	case relayconstant.RelayModeMidjourneyTaskImageSeed:
		err = relay.RelayMidjourneyTaskImageSeed(c)
	case relayconstant.RelayModeSwapFace:
		err = relay.RelaySwapFace(c)
	default:
		err = relay.RelayMidjourneySubmit(c, relayMode)
	}
	//err = relayMidjourneySubmit(c, relayMode)
	log.Println(err)
	if err != nil {
		statusCode := http.StatusBadRequest
		if err.Code == 30 {
			err.Result = "当前分组负载已饱和，请稍后再试，或升级账户以提升服务质量。"
			statusCode = http.StatusTooManyRequests
		}
		c.JSON(statusCode, gin.H{
			"description": fmt.Sprintf("%s %s", err.Description, err.Result),
			"type":        "upstream_error",
			"code":        err.Code,
		})
		channelId := c.GetInt("channel_id")
		common.LogError(c, fmt.Sprintf("relay error (channel #%d, status code %d): %s", channelId, statusCode, fmt.Sprintf("%s %s", err.Description, err.Result)))
	}
}

func RelayNotImplemented(c *gin.Context) {
	err := dto.OpenAIError{
		Message: "API not implemented",
		Type:    "new_api_error",
		Param:   "",
		Code:    "api_not_implemented",
	}
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": err,
	})
}

func RelayNotFound(c *gin.Context) {
	err := dto.OpenAIError{
		Message: fmt.Sprintf("Invalid URL (%s %s)", c.Request.Method, c.Request.URL.Path),
		Type:    "invalid_request_error",
		Param:   "",
		Code:    "",
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error": err,
	})
}

func RelayTask(c *gin.Context) {
	relayMode := c.GetInt("relay_mode")
	group := c.GetString("group")
	originalModel := c.GetString("original_model")
	var taskErr *dto.TaskError

	for i := 0; i <= common.RetryTimes; i++ {
		channel, err := getChannel(c, group, originalModel, i)
		if err != nil {
			common.LogError(c, err.Error())
			taskErr = service.TaskErrorWrapperLocal(err, "get_channel_failed", http.StatusInternalServerError)
			break
		}

		// 使用新的渠道ID更新used_channel列表
		useChannel := c.GetStringSlice("use_channel")
		useChannel = append(useChannel, fmt.Sprintf("%d", channel.Id))
		c.Set("use_channel", useChannel)

		taskErr = taskRelayHandler(c, relayMode)

		if taskErr == nil {
			return // 成功处理请求，直接返回
		}

		// 检查是否需要立即禁用渠道并继续重试
		if service.ShouldImmediatelyDisableAndRetryTask(taskErr) && channel.GetAutoBan() {
			// 立即禁用渠道
			service.DisableChannel(channel.Id, channel.Name, fmt.Sprintf("立即禁用 - 状态码: %d, 错误: %s", taskErr.StatusCode, taskErr.Message))
			common.LogError(c, fmt.Sprintf("立即禁用渠道 #%d，状态码: %d，继续重试其他渠道", channel.Id, taskErr.StatusCode))
			// 继续重试其他渠道
			continue
		}

		if !shouldRetryTaskRelay(c, channel.Id, taskErr, common.RetryTimes-i) {
			break
		}
	}
	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		common.LogInfo(c, retryLogStr)
	}

	// 如果重试成功（没有错误），也要显示重试过程
	if taskErr == nil && len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试成功：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		common.LogInfo(c, retryLogStr)
	}

	if taskErr != nil {
		if taskErr.StatusCode == http.StatusTooManyRequests {
			taskErr.Message = "当前分组上游负载已饱和，请稍后再试"
		}

		// 只有在最终失败时才记录错误日志（重试成功的不算错误）
		if constant2.ErrorLogEnabled {
			userId := c.GetInt("id")
			tokenName := c.GetString("token_name")
			modelName := c.GetString("original_model")
			tokenId := c.GetInt("token_id")
			userGroup := c.GetString("group")
			channelId := c.GetInt("channel_id")
			other := make(map[string]interface{})
			other["error_type"] = "task_error"
			other["error_code"] = taskErr.Code
			other["status_code"] = taskErr.StatusCode
			other["channel_id"] = channelId
			other["channel_name"] = c.GetString("channel_name")
			other["channel_type"] = c.GetInt("channel_type")
			// 记录重试信息
			useChannel := c.GetStringSlice("use_channel")
			if len(useChannel) > 1 {
				other["retry_channels"] = strings.Join(useChannel, "->")
			}

			model.RecordErrorLog(c, userId, channelId, modelName, tokenName, taskErr.Message, tokenId, 0, false, userGroup, other)
		}

		c.JSON(taskErr.StatusCode, taskErr)
	}
}

func taskRelayHandler(c *gin.Context, relayMode int) *dto.TaskError {
	var err *dto.TaskError
	switch relayMode {
	case relayconstant.RelayModeSunoFetch, relayconstant.RelayModeSunoFetchByID:
		err = relay.RelayTaskFetch(c, relayMode)
	default:
		err = relay.RelayTaskSubmit(c, relayMode)
	}
	return err
}

func shouldRetryTaskRelay(c *gin.Context, channelId int, taskErr *dto.TaskError, retryTimes int) bool {
	if taskErr == nil {
		return false
	}
	if retryTimes <= 0 {
		return false
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		return false
	}
	if taskErr.LocalError {
		return false
	}
	if taskErr.StatusCode/100 == 2 {
		return false
	}
	// 超时不重试
	if taskErr.StatusCode == 504 || taskErr.StatusCode == 524 || taskErr.StatusCode == 408 {
		return false
	}

	// 所有其他错误都重试
	return true
}

// getRetryReason 获取重试原因
func getRetryReason(openaiErr *dto.OpenAIErrorWithStatusCode) string {
	if openaiErr == nil {
		return "unknown"
	}

	if openaiErr.StatusCode == http.StatusTooManyRequests {
		return "rate_limit"
	}
	if openaiErr.StatusCode == http.StatusInternalServerError {
		return "server_error"
	}
	if openaiErr.StatusCode == http.StatusBadGateway {
		return "bad_gateway"
	}
	if openaiErr.StatusCode == http.StatusServiceUnavailable {
		return "service_unavailable"
	}
	if openaiErr.StatusCode == http.StatusGatewayTimeout {
		return "gateway_timeout"
	}
	if openaiErr.StatusCode == http.StatusUnauthorized {
		return "unauthorized"
	}
	if openaiErr.StatusCode == http.StatusForbidden {
		return "forbidden"
	}

	// 根据错误类型分类
	if openaiErr.Error.Type == "empty_response" {
		return "empty_response"
	}
	if openaiErr.Error.Type == "insufficient_quota" {
		return "insufficient_quota"
	}

	return fmt.Sprintf("http_%d", openaiErr.StatusCode)
}
