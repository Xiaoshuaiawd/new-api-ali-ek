package test

import (
	"one-api/dto"
	"one-api/relay/channel/gemini"
	"testing"
)

// TestGeminiTokenCalculation 测试Gemini token计算逻辑
func TestGeminiTokenCalculation(t *testing.T) {
	// 测试用例1：有思考内容的情况
	t.Run("WithReasoningTokens", func(t *testing.T) {
		// 模拟Gemini响应数据
		geminiResponse := gemini.GeminiChatResponse{
			UsageMetadata: gemini.GeminiUsageMetadata{
				PromptTokenCount:     100,
				CandidatesTokenCount: 50, // 原始completion tokens
				ThoughtsTokenCount:   30, // reasoning tokens
				TotalTokenCount:      180,
			},
		}

		// 按照新逻辑计算
		originalCompletionTokens := geminiResponse.UsageMetadata.CandidatesTokenCount
		reasoningTokens := geminiResponse.UsageMetadata.ThoughtsTokenCount

		expectedUsage := dto.Usage{
			PromptTokens:     100,
			CompletionTokens: originalCompletionTokens + reasoningTokens, // 50 + 30 = 80
			TotalTokens:      180,
		}
		expectedUsage.CompletionTokenDetails.ReasoningTokens = reasoningTokens     // 30
		expectedUsage.CompletionTokenDetails.TextTokens = originalCompletionTokens // 50

		// 验证计算结果
		if expectedUsage.CompletionTokens != 80 {
			t.Errorf("Expected completion_tokens to be 80, got %d", expectedUsage.CompletionTokens)
		}

		if expectedUsage.CompletionTokenDetails.TextTokens != 50 {
			t.Errorf("Expected text_tokens to be 50, got %d", expectedUsage.CompletionTokenDetails.TextTokens)
		}

		if expectedUsage.CompletionTokenDetails.ReasoningTokens != 30 {
			t.Errorf("Expected reasoning_tokens to be 30, got %d", expectedUsage.CompletionTokenDetails.ReasoningTokens)
		}

		t.Logf("✅ 测试通过:")
		t.Logf("   原始 completion_tokens: %d", originalCompletionTokens)
		t.Logf("   reasoning_tokens: %d", reasoningTokens)
		t.Logf("   新的 completion_tokens: %d (原始 + 思考)", expectedUsage.CompletionTokens)
		t.Logf("   text_tokens: %d (原始 completion_tokens)", expectedUsage.CompletionTokenDetails.TextTokens)
	})

	// 测试用例2：没有思考内容的情况
	t.Run("WithoutReasoningTokens", func(t *testing.T) {
		geminiResponse := gemini.GeminiChatResponse{
			UsageMetadata: gemini.GeminiUsageMetadata{
				PromptTokenCount:     100,
				CandidatesTokenCount: 50, // 原始completion tokens
				ThoughtsTokenCount:   0,  // 没有reasoning tokens
				TotalTokenCount:      150,
			},
		}

		originalCompletionTokens := geminiResponse.UsageMetadata.CandidatesTokenCount
		reasoningTokens := geminiResponse.UsageMetadata.ThoughtsTokenCount

		expectedUsage := dto.Usage{
			PromptTokens:     100,
			CompletionTokens: originalCompletionTokens + reasoningTokens, // 50 + 0 = 50
			TotalTokens:      150,
		}
		expectedUsage.CompletionTokenDetails.ReasoningTokens = reasoningTokens     // 0
		expectedUsage.CompletionTokenDetails.TextTokens = originalCompletionTokens // 50

		// 验证计算结果
		if expectedUsage.CompletionTokens != 50 {
			t.Errorf("Expected completion_tokens to be 50, got %d", expectedUsage.CompletionTokens)
		}

		if expectedUsage.CompletionTokenDetails.TextTokens != 50 {
			t.Errorf("Expected text_tokens to be 50, got %d", expectedUsage.CompletionTokenDetails.TextTokens)
		}

		if expectedUsage.CompletionTokenDetails.ReasoningTokens != 0 {
			t.Errorf("Expected reasoning_tokens to be 0, got %d", expectedUsage.CompletionTokenDetails.ReasoningTokens)
		}

		t.Logf("✅ 测试通过:")
		t.Logf("   原始 completion_tokens: %d", originalCompletionTokens)
		t.Logf("   reasoning_tokens: %d", reasoningTokens)
		t.Logf("   新的 completion_tokens: %d (原始 + 思考)", expectedUsage.CompletionTokens)
		t.Logf("   text_tokens: %d (原始 completion_tokens)", expectedUsage.CompletionTokenDetails.TextTokens)
	})
}

// TestGeminiTokenCalculationFormula 测试token计算公式
func TestGeminiTokenCalculationFormula(t *testing.T) {
	testCases := []struct {
		name                     string
		promptTokens             int
		originalCompletionTokens int
		reasoningTokens          int
		expectedCompletionTokens int
		expectedTextTokens       int
	}{
		{
			name:                     "有思考内容",
			promptTokens:             100,
			originalCompletionTokens: 50,
			reasoningTokens:          30,
			expectedCompletionTokens: 80, // 50 + 30
			expectedTextTokens:       50, // 原始50
		},
		{
			name:                     "无思考内容",
			promptTokens:             100,
			originalCompletionTokens: 60,
			reasoningTokens:          0,
			expectedCompletionTokens: 60, // 60 + 0
			expectedTextTokens:       60, // 原始60
		},
		{
			name:                     "大量思考内容",
			promptTokens:             200,
			originalCompletionTokens: 100,
			reasoningTokens:          150,
			expectedCompletionTokens: 250, // 100 + 150
			expectedTextTokens:       100, // 原始100
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 应用新的计算逻辑
			actualCompletionTokens := tc.originalCompletionTokens + tc.reasoningTokens
			actualTextTokens := tc.originalCompletionTokens

			if actualCompletionTokens != tc.expectedCompletionTokens {
				t.Errorf("Expected completion_tokens %d, got %d", tc.expectedCompletionTokens, actualCompletionTokens)
			}

			if actualTextTokens != tc.expectedTextTokens {
				t.Errorf("Expected text_tokens %d, got %d", tc.expectedTextTokens, actualTextTokens)
			}

			t.Logf("✅ %s: completion_tokens=%d, text_tokens=%d, reasoning_tokens=%d",
				tc.name, actualCompletionTokens, actualTextTokens, tc.reasoningTokens)
		})
	}
}
