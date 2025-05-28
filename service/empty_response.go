package service

import (
	"one-api/dto"
	"one-api/setting/operation_setting"
	"strings"
)

// IsEmptyResponse 检测响应是否为空内容
func IsEmptyResponse(response *dto.OpenAITextResponse) bool {
	// 检查是否启用了空回复重试
	generalSetting := operation_setting.GetGeneralSetting()
	if !generalSetting.EmptyResponseRetryEnabled {
		return false
	}

	// 检查响应是否有效
	if response == nil || len(response.Choices) == 0 {
		return true
	}

	// 检查第一个choice的内容
	choice := response.Choices[0]
	content := choice.Message.StringContent()

	// 去除空白字符后检查是否为空
	content = strings.TrimSpace(content)
	if content == "" {
		return true
	}

	return false
}

// IsEmptyTextResponse 检测TextResponse是否为空内容 (用于Cloudflare等特殊适配器)
func IsEmptyTextResponse(response *dto.TextResponse) bool {
	// 检查是否启用了空回复重试
	generalSetting := operation_setting.GetGeneralSetting()
	if !generalSetting.EmptyResponseRetryEnabled {
		return false
	}

	// 检查响应是否有效
	if response == nil || len(response.Choices) == 0 {
		return true
	}

	// 检查第一个choice的内容
	choice := response.Choices[0]
	content := choice.Message.StringContent()

	// 去除空白字符后检查是否为空
	content = strings.TrimSpace(content)
	if content == "" {
		return true
	}

	return false
}

// IsEmptyStreamResponse 检测流式响应是否为空内容
func IsEmptyStreamResponse(responseText string) bool {
	// 检查是否启用了空回复重试
	generalSetting := operation_setting.GetGeneralSetting()
	if !generalSetting.EmptyResponseRetryEnabled {
		return false
	}

	// 去除空白字符后检查是否为空
	responseText = strings.TrimSpace(responseText)
	return responseText == ""
}

// CreateEmptyResponseError 创建空回复错误
func CreateEmptyResponseError() *dto.OpenAIErrorWithStatusCode {
	return &dto.OpenAIErrorWithStatusCode{
		Error: dto.OpenAIError{
			Message: "empty_response_retry", // 简化错误信息，便于识别
			Type:    "empty_response",
			Code:    "empty_response_retry",
		},
		StatusCode: 200,  // 使用200状态码，避免被记录为错误
		LocalError: true, // 标记为本地错误，避免被记录到上游错误统计中
	}
}
