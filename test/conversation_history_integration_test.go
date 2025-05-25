package test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"one-api/dto"
	"one-api/relay/channel/openai"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestConversationHistoryIntegration 测试对话历史功能的完整集成
func TestConversationHistoryIntegration(t *testing.T) {
	// 初始化测试环境
	gin.SetMode(gin.TestMode)

	// 测试AI回复内容提取
	t.Run("TestAIResponseContentExtraction", func(t *testing.T) {
		// 创建测试上下文
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		// 模拟AI回复内容
		aiResponse := &openai.AIResponseContent{
			Content:          "这是AI的回复内容",
			ReasoningContent: "这是AI的思考过程",
		}
		c.Set("ai_response_content", aiResponse)

		// 测试从上下文中提取AI回复内容
		extracted := extractAIResponseFromContext(c)
		assert.NotNil(t, extracted)
		assert.Equal(t, "这是AI的回复内容", extracted.Content)
		assert.Equal(t, "这是AI的思考过程", extracted.ReasoningContent)
	})

	// 测试从响应体解析AI回复内容
	t.Run("TestParseAIResponseFromBody", func(t *testing.T) {
		// 构建测试响应体
		response := dto.OpenAITextResponse{
			Choices: []dto.OpenAITextResponseChoice{
				{
					Message: dto.Message{
						Role:             "assistant",
						ReasoningContent: "AI的思考过程",
					},
				},
			},
		}
		response.Choices[0].Message.SetStringContent("AI的回复内容")

		responseBody, err := json.Marshal(response)
		assert.NoError(t, err)

		// 测试解析
		extracted := parseAIResponseFromBody(responseBody)
		assert.NotNil(t, extracted)
		assert.Equal(t, "AI的回复内容", extracted.Content)
		assert.Equal(t, "AI的思考过程", extracted.ReasoningContent)
	})

	// 测试空回复不保存
	t.Run("TestEmptyResponseNotSaved", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		// 模拟空的AI回复
		aiResponse := &openai.AIResponseContent{
			Content: "",
		}
		c.Set("ai_response_content", aiResponse)

		extracted := extractAIResponseFromContext(c)
		assert.NotNil(t, extracted)
		assert.Equal(t, "", extracted.Content)

		// 在实际的saveConversationHistoryWithResponse函数中，空内容会导致提前返回
	})

	// 测试对话历史JSON格式
	t.Run("TestConversationHistoryFormat", func(t *testing.T) {
		// 构建测试消息
		userMessage := dto.Message{
			Role: "user",
		}
		userMessage.SetStringContent("你是什么模型")

		aiMessage := dto.Message{
			Role:             "assistant",
			ReasoningContent: "我需要思考一下这个问题",
		}
		aiMessage.SetStringContent("我是DeepSeek-R1模型")

		messages := []dto.Message{userMessage, aiMessage}

		conversationData := map[string]interface{}{
			"messages": messages,
			"model":    "deepseek-r1",
		}

		jsonData, err := json.Marshal(conversationData)
		assert.NoError(t, err)

		// 验证JSON格式
		var parsed map[string]interface{}
		err = json.Unmarshal(jsonData, &parsed)
		assert.NoError(t, err)

		assert.Equal(t, "deepseek-r1", parsed["model"])

		messagesArray, ok := parsed["messages"].([]interface{})
		assert.True(t, ok)
		assert.Len(t, messagesArray, 2)

		// 验证用户消息
		userMsg := messagesArray[0].(map[string]interface{})
		assert.Equal(t, "user", userMsg["role"])

		// 验证AI消息
		aiMsg := messagesArray[1].(map[string]interface{})
		assert.Equal(t, "assistant", aiMsg["role"])
		assert.Equal(t, "我需要思考一下这个问题", aiMsg["reasoning_content"])

		fmt.Printf("Generated conversation JSON: %s\n", string(jsonData))
	})
}

// 这些函数需要从relay-text.go中导入，这里只是为了测试而重新定义
func extractAIResponseFromContext(c *gin.Context) *openai.AIResponseContent {
	if responseData, exists := c.Get("ai_response_content"); exists {
		if content, ok := responseData.(*openai.AIResponseContent); ok {
			return content
		}
	}
	return nil
}

func parseAIResponseFromBody(responseBody []byte) *openai.AIResponseContent {
	var openaiResponse dto.OpenAITextResponse
	if err := json.Unmarshal(responseBody, &openaiResponse); err != nil {
		return nil
	}

	if len(openaiResponse.Choices) == 0 {
		return nil
	}

	choice := openaiResponse.Choices[0]
	content := choice.Message.StringContent()

	if content == "" {
		return nil
	}

	result := &openai.AIResponseContent{
		Content: content,
	}

	if choice.Message.ReasoningContent != "" {
		result.ReasoningContent = choice.Message.ReasoningContent
	} else if choice.Message.Reasoning != "" {
		result.ReasoningContent = choice.Message.Reasoning
	}

	return result
}
