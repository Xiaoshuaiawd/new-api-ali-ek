# Gemini Token 计算逻辑更新

## 修改概述

根据用户需求，对Gemini模型的token计算逻辑进行了调整，以更好地反映思考内容（reasoning tokens）的使用情况。

## 修改前后对比

### 修改前的逻辑
```go
usage := dto.Usage{
    PromptTokens:     geminiResponse.UsageMetadata.PromptTokenCount,
    CompletionTokens: geminiResponse.UsageMetadata.CandidatesTokenCount,
    TotalTokens:      geminiResponse.UsageMetadata.TotalTokenCount,
}
usage.CompletionTokenDetails.ReasoningTokens = geminiResponse.UsageMetadata.ThoughtsTokenCount
usage.CompletionTokens = usage.TotalTokens - usage.PromptTokens
```

### 修改后的逻辑
```go
// 保存原始的completion_tokens用作text_tokens
originalCompletionTokens := geminiResponse.UsageMetadata.CandidatesTokenCount
reasoningTokens := geminiResponse.UsageMetadata.ThoughtsTokenCount

usage := dto.Usage{
    PromptTokens:     geminiResponse.UsageMetadata.PromptTokenCount,
    CompletionTokens: originalCompletionTokens + reasoningTokens, // completion_tokens = 原completion_tokens + reasoning_tokens
    TotalTokens:      geminiResponse.UsageMetadata.TotalTokenCount,
}

usage.CompletionTokenDetails.ReasoningTokens = reasoningTokens
usage.CompletionTokenDetails.TextTokens = originalCompletionTokens // text_tokens = 原completion_tokens
```

## 新的计算规则

| 字段 | 计算方式 | 说明 |
|------|----------|------|
| `completion_tokens` | `原始completion_tokens + reasoning_tokens` | 包含所有生成的token |
| `text_tokens` | `原始completion_tokens` | 仅包含文本内容的token |
| `reasoning_tokens` | `gemini返回的thoughts_token_count` | 思考过程的token |

## 示例

### 有思考内容的情况
```json
// Gemini原始响应
{
  "usageMetadata": {
    "promptTokenCount": 100,
    "candidatesTokenCount": 50,    // 原始completion tokens
    "thoughtsTokenCount": 30,      // reasoning tokens
    "totalTokenCount": 180
  }
}

// 修改后的usage计算结果
{
  "prompt_tokens": 100,
  "completion_tokens": 80,         // 50 + 30
  "total_tokens": 180,
  "completion_tokens_details": {
    "text_tokens": 50,             // 原始的50
    "reasoning_tokens": 30         // 思考内容的30
  }
}
```

### 无思考内容的情况
```json
// Gemini原始响应
{
  "usageMetadata": {
    "promptTokenCount": 100,
    "candidatesTokenCount": 60,    // 原始completion tokens
    "thoughtsTokenCount": 0,       // 无reasoning tokens
    "totalTokenCount": 160
  }
}

// 修改后的usage计算结果
{
  "prompt_tokens": 100,
  "completion_tokens": 60,         // 60 + 0
  "total_tokens": 160,
  "completion_tokens_details": {
    "text_tokens": 60,             // 原始的60
    "reasoning_tokens": 0          // 无思考内容
  }
}
```

## 修改的文件

1. **`relay/channel/gemini/relay-gemini.go`**
   - `GeminiChatHandler` 函数中的token计算逻辑
   - `GeminiChatStreamHandler` 函数中的token计算逻辑

## 影响范围

- ✅ 非流式响应的token计算
- ✅ 流式响应的token计算
- ✅ 保持与OpenAI格式的兼容性
- ✅ 正确反映思考内容的token使用

## 测试验证

创建了 `test/gemini_token_calculation_test.go` 来验证新的计算逻辑：

```bash
go test ./test -run TestGeminiTokenCalculation -v
```

## 优势

1. **更准确的计费**：`completion_tokens` 包含了所有生成的内容
2. **详细的分解**：通过 `text_tokens` 和 `reasoning_tokens` 可以清楚地看到各部分的token使用
3. **向后兼容**：保持了OpenAI API格式的兼容性
4. **透明度**：用户可以清楚地了解思考过程消耗的token数量

## 注意事项

- 这个修改只影响Gemini模型的token计算
- 其他模型（如OpenAI、Claude等）的token计算逻辑保持不变
- 修改后的计算方式更符合实际的token使用情况 