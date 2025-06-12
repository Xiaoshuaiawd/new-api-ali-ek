package test

import (
	"encoding/json"
	"one-api/model"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorConversationHistory(t *testing.T) {
	err := model.InitDB()
	if err != nil {
		t.Fatal(err)
	}

	testConversationId := "test_error_conv_123"
	testModelName := "gpt-3.5-turbo"
	testErrorMessage := "Rate limit exceeded"
	testErrorCode := "rate_limit_exceeded"
	testUserId := 1

	// Test data - raw_json只包含原始对话内容
	conversationData := map[string]interface{}{
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "Hello, world!",
			},
		},
		"model": testModelName,
	}
	jsonData, err := json.Marshal(conversationData)
	assert.NoError(t, err)

	t.Run("CreateErrorConversationHistory", func(t *testing.T) {
		err = model.CreateErrorConversationHistory(nil, testConversationId, testModelName, string(jsonData), testErrorMessage, testErrorCode, testUserId)
		assert.NoError(t, err)
	})

	t.Run("GetErrorConversationHistoriesByConversationId", func(t *testing.T) {
		histories, total, err := model.GetErrorConversationHistoriesByConversationId(testConversationId, 10, 0)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, histories, 1)
		assert.Equal(t, testConversationId, histories[0].ConversationId)
		assert.Equal(t, testModelName, histories[0].ModelName)
		assert.Equal(t, testErrorMessage, histories[0].ErrorMessage)
		assert.Equal(t, testErrorCode, histories[0].ErrorCode)
		assert.Equal(t, testUserId, histories[0].UserId)
	})

	t.Run("GetErrorConversationHistoriesByUserId", func(t *testing.T) {
		histories, total, err := model.GetErrorConversationHistoriesByUserId(testUserId, 10, 0)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, total, int64(1))
		assert.GreaterOrEqual(t, len(histories), 1)

		// Find our test history
		found := false
		for _, history := range histories {
			if history.ConversationId == testConversationId {
				found = true
				assert.Equal(t, testModelName, history.ModelName)
				assert.Equal(t, testErrorMessage, history.ErrorMessage)
				assert.Equal(t, testErrorCode, history.ErrorCode)
				break
			}
		}
		assert.True(t, found, "Test error conversation history not found")
	})

	t.Run("SearchErrorConversationHistories", func(t *testing.T) {
		// Search by keyword
		histories, total, err := model.SearchErrorConversationHistories("Rate limit", 0, "", 10, 0)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, total, int64(1))
		assert.GreaterOrEqual(t, len(histories), 1)

		// Search by user ID
		histories, total, err = model.SearchErrorConversationHistories("", testUserId, "", 10, 0)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, total, int64(1))
		assert.GreaterOrEqual(t, len(histories), 1)

		// Search by model name
		histories, total, err = model.SearchErrorConversationHistories("", 0, testModelName, 10, 0)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, total, int64(1))
		assert.GreaterOrEqual(t, len(histories), 1)
	})

	t.Run("GetRawJsonAsMap", func(t *testing.T) {
		histories, _, err := model.GetErrorConversationHistoriesByConversationId(testConversationId, 1, 0)
		assert.NoError(t, err)
		assert.Len(t, histories, 1)

		dataMap, err := histories[0].GetRawJsonAsMap()
		assert.NoError(t, err)
		assert.Equal(t, testModelName, dataMap["model"])

		// 检查messages是否存在
		messages, exists := dataMap["messages"]
		assert.True(t, exists)
		assert.NotNil(t, messages)
	})

	t.Run("SetRawJsonFromMap", func(t *testing.T) {
		histories, _, err := model.GetErrorConversationHistoriesByConversationId(testConversationId, 1, 0)
		assert.NoError(t, err)
		assert.Len(t, histories, 1)

		newData := map[string]interface{}{
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Updated message",
				},
			},
			"model": "gpt-4",
		}

		err = histories[0].SetRawJsonFromMap(newData)
		assert.NoError(t, err)

		dataMap, err := histories[0].GetRawJsonAsMap()
		assert.NoError(t, err)
		assert.Equal(t, "gpt-4", dataMap["model"])

		// 检查messages是否正确更新
		messages, exists := dataMap["messages"]
		assert.True(t, exists)
		assert.NotNil(t, messages)
	})

	t.Run("CleanupOldErrorConversationHistories", func(t *testing.T) {
		// This should not delete our recent test data
		deletedCount, err := model.CleanupOldErrorConversationHistories(30)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, deletedCount, int64(0))

		// Verify our test data still exists
		histories, total, err := model.GetErrorConversationHistoriesByConversationId(testConversationId, 10, 0)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, histories, 1)
	})

	// Cleanup
	t.Run("DeleteErrorConversationHistory", func(t *testing.T) {
		histories, _, err := model.GetErrorConversationHistoriesByConversationId(testConversationId, 1, 0)
		assert.NoError(t, err)
		assert.Len(t, histories, 1)

		err = model.DeleteErrorConversationHistory(histories[0].Id)
		assert.NoError(t, err)

		// Verify deletion
		histories, total, err := model.GetErrorConversationHistoriesByConversationId(testConversationId, 10, 0)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)
		assert.Len(t, histories, 0)
	})
}
