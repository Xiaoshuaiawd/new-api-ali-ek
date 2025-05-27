package service

import (
	"fmt"
	"net/http"
	"one-api/common"
	"one-api/dto"
	"one-api/model"
	"one-api/setting/operation_setting"
	"strings"
)

func formatNotifyType(channelId int, status int) string {
	return fmt.Sprintf("%s_%d_%d", dto.NotifyTypeChannelUpdate, channelId, status)
}

// disable & notify
func DisableChannel(channelId int, channelName string, reason string) {
	success := model.UpdateChannelStatusById(channelId, common.ChannelStatusAutoDisabled, reason)
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被禁用", channelName, channelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被禁用，原因：%s", channelName, channelId, reason)
		NotifyRootUser(formatNotifyType(channelId, common.ChannelStatusAutoDisabled), subject, content)
	}
}

func EnableChannel(channelId int, channelName string) {
	success := model.UpdateChannelStatusById(channelId, common.ChannelStatusEnabled, "")
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		NotifyRootUser(formatNotifyType(channelId, common.ChannelStatusEnabled), subject, content)
	}
}

// ShouldImmediatelyDisableAndRetryTask 检查任务错误是否应该立即禁用渠道并重试其他渠道
func ShouldImmediatelyDisableAndRetryTask(err *dto.TaskError) bool {
	if err == nil || err.LocalError {
		return false
	}

	// 检查是否在配置的立即禁用状态码列表中
	disableStatusCodes := operation_setting.GetGeneralSetting().RetryDisableStatusCodes
	if disableStatusCodes != "" {
		statusCodeStr := fmt.Sprintf("%d", err.StatusCode)
		statusCodes := strings.Split(disableStatusCodes, ",")
		for _, code := range statusCodes {
			if strings.TrimSpace(code) == statusCodeStr {
				return true
			}
		}
	}

	return false
}

// ShouldImmediatelyDisableAndRetry 检查是否应该立即禁用渠道并重试其他渠道
func ShouldImmediatelyDisableAndRetry(err *dto.OpenAIErrorWithStatusCode) bool {
	if err == nil || err.LocalError {
		return false
	}

	// 检查是否在配置的立即禁用状态码列表中
	disableStatusCodes := operation_setting.GetGeneralSetting().RetryDisableStatusCodes
	if disableStatusCodes != "" {
		statusCodeStr := fmt.Sprintf("%d", err.StatusCode)
		statusCodes := strings.Split(disableStatusCodes, ",")
		for _, code := range statusCodes {
			if strings.TrimSpace(code) == statusCodeStr {
				return true
			}
		}
	}

	return false
}

func ShouldDisableChannel(channelType int, err *dto.OpenAIErrorWithStatusCode) bool {
	if !common.AutomaticDisableChannelEnabled {
		return false
	}
	if err == nil {
		return false
	}
	if err.LocalError {
		return false
	}

	// 检查是否在配置的禁用状态码列表中
	disableStatusCodes := operation_setting.GetGeneralSetting().RetryDisableStatusCodes
	if disableStatusCodes != "" {
		statusCodeStr := fmt.Sprintf("%d", err.StatusCode)
		statusCodes := strings.Split(disableStatusCodes, ",")
		for _, code := range statusCodes {
			if strings.TrimSpace(code) == statusCodeStr {
				return true
			}
		}
		// 如果指定了状态码列表，且当前状态码不在列表中，则不禁用
		return false
	}

	// 如果没有配置状态码列表，则使用原有的逻辑
	if err.StatusCode == http.StatusUnauthorized {
		return true
	}
	if err.StatusCode == http.StatusForbidden {
		switch channelType {
		case common.ChannelTypeGemini:
			return true
		}
	}
	switch err.Error.Code {
	case "invalid_api_key":
		return true
	case "account_deactivated":
		return true
	case "billing_not_active":
		return true
	}
	switch err.Error.Type {
	case "insufficient_quota":
		return true
	case "insufficient_user_quota":
		return true
	// https://docs.anthropic.com/claude/reference/errors
	case "authentication_error":
		return true
	case "permission_error":
		return true
	case "forbidden":
		return true
	}

	lowerMessage := strings.ToLower(err.Error.Message)
	search, _ := AcSearch(lowerMessage, operation_setting.AutomaticDisableKeywords, true)
	if search {
		return true
	}

	return false
}

func ShouldEnableChannel(err error, openaiWithStatusErr *dto.OpenAIErrorWithStatusCode, status int) bool {
	if !common.AutomaticEnableChannelEnabled {
		return false
	}
	if err != nil {
		return false
	}
	if openaiWithStatusErr != nil {
		return false
	}
	if status != common.ChannelStatusAutoDisabled {
		return false
	}
	return true
}

// CheckChannelRPMLimit 检查渠道RPM限制
func CheckChannelRPMLimit(channelId int) bool {
	channel, err := model.GetChannelById(channelId, true)
	if err != nil {
		return false // 获取渠道信息失败，不限制
	}

	return channel.CheckRPMLimit()
}

// IncrementChannelRPMUsage 增加渠道RPM使用次数
func IncrementChannelRPMUsage(channelId int) {
	channel, err := model.GetChannelById(channelId, true)
	if err != nil {
		return
	}

	if !channel.GetRPMLimitEnabled() {
		return
	}

	// 增加RPM使用计数
	channel.IncrementRPMUsage()

	// 保存到数据库
	err = model.DB.Model(channel).Select("last_minute_time", "current_minute_used").Updates(model.Channel{
		LastMinuteTime:    channel.LastMinuteTime,
		CurrentMinuteUsed: channel.CurrentMinuteUsed,
	}).Error
	if err != nil {
		common.SysError(fmt.Sprintf("failed to update channel RPM usage: %s", err.Error()))
	}
}
