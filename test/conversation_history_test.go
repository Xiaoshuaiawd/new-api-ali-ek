package test

import (
	"encoding/json"
	"fmt"
	"log"
	"one-api/model"
	"testing"
	"time"
)

// TestConversationHistory 测试对话历史功能
func TestConversationHistory(t *testing.T) {
	// 初始化数据库连接（这里需要根据实际情况配置）
	err := model.InitDB()
	if err != nil {
		t.Fatal("Failed to initialize database:", err)
	}
	defer model.CloseDB()

	// 测试数据
	testUserId := 1
	testConversationId := "test_conv_123"
	testModelName := "claude-opus-4-20250514"

	// 构建测试的对话数据
	conversationData := map[string]interface{}{
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "如何编写高性能的 CUDA Kernel",
					},
				},
			},
			{
				"role":    "assistant",
				"content": "编写高性能的 CUDA Kernel 是一个复杂的主题，涉及多个方面的优化...",
			},
		},
		"model": testModelName,
	}

	jsonData, err := json.Marshal(conversationData)
	if err != nil {
		t.Fatal("Failed to marshal conversation data:", err)
	}

	fmt.Println("=== 测试对话历史功能 ===")

	// 1. 测试创建对话历史
	t.Run("CreateConversationHistory", func(t *testing.T) {
		err = model.CreateConversationHistory(nil, testConversationId, testModelName, string(jsonData), testUserId)
		if err != nil {
			t.Errorf("创建对话历史失败: %v", err)
		} else {
			fmt.Println("✓ 创建对话历史成功")
		}
	})

	// 2. 测试根据用户ID获取对话历史
	t.Run("GetConversationHistoriesByUserId", func(t *testing.T) {
		histories, total, err := model.GetConversationHistoriesByUserId(testUserId, 10, 0)
		if err != nil {
			t.Errorf("获取对话历史失败: %v", err)
		} else {
			fmt.Printf("✓ 获取到 %d 条对话历史，总计 %d 条\n", len(histories), total)
			for _, history := range histories {
				fmt.Printf("  - ID: %d, ConversationID: %s, Model: %s, CreatedAt: %s\n",
					history.Id, history.ConversationId, history.ModelName, history.CreatedAt.Format(time.RFC3339))
			}
		}
	})

	// 3. 测试根据对话ID获取对话历史
	t.Run("GetConversationHistoriesByConversationId", func(t *testing.T) {
		histories, total, err := model.GetConversationHistoriesByConversationId(testConversationId, 10, 0)
		if err != nil {
			t.Errorf("获取对话历史失败: %v", err)
		} else {
			fmt.Printf("✓ 获取到 %d 条对话历史，总计 %d 条\n", len(histories), total)
		}
	})

	// 4. 测试搜索对话历史
	t.Run("SearchConversationHistories", func(t *testing.T) {
		histories, total, err := model.SearchConversationHistories("CUDA", testUserId, "", 10, 0)
		if err != nil {
			t.Errorf("搜索对话历史失败: %v", err)
		} else {
			fmt.Printf("✓ 搜索到 %d 条对话历史，总计 %d 条\n", len(histories), total)
		}
	})

	// 5. 测试解析JSON数据
	t.Run("ParseJSONData", func(t *testing.T) {
		histories, _, err := model.GetConversationHistoriesByUserId(testUserId, 1, 0)
		if err != nil {
			t.Errorf("获取对话历史失败: %v", err)
			return
		}

		if len(histories) > 0 {
			history := histories[0]
			data, err := history.GetRawJsonAsMap()
			if err != nil {
				t.Errorf("解析JSON数据失败: %v", err)
			} else {
				fmt.Println("✓ JSON数据解析成功")
				if messages, ok := data["messages"].([]interface{}); ok {
					fmt.Printf("  - 包含 %d 条消息\n", len(messages))
				}
				if model, ok := data["model"].(string); ok {
					fmt.Printf("  - 模型: %s\n", model)
				}
			}
		}
	})

	// 6. 测试删除对话历史
	t.Run("DeleteConversationHistory", func(t *testing.T) {
		histories, _, err := model.GetConversationHistoriesByUserId(testUserId, 1, 0)
		if err != nil {
			t.Errorf("获取对话历史失败: %v", err)
			return
		}

		if len(histories) > 0 {
			history := histories[0]
			err = model.DeleteConversationHistory(history.Id)
			if err != nil {
				t.Errorf("删除对话历史失败: %v", err)
			} else {
				fmt.Println("✓ 删除对话历史成功")
			}
		}
	})

	// 7. 测试清理旧的对话历史
	t.Run("CleanupOldConversationHistories", func(t *testing.T) {
		count, err := model.CleanupOldConversationHistories(0) // 清理所有记录
		if err != nil {
			t.Errorf("清理对话历史失败: %v", err)
		} else {
			fmt.Printf("✓ 清理了 %d 条对话历史\n", count)
		}
	})

	fmt.Println("\n=== 测试完成 ===")
}

// 运行测试的示例函数
func ExampleRunTest() {
	// 运行测试: go test ./test -v
	log.Println("运行测试命令: go test ./test -v")
}
