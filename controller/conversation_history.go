package controller

import (
	"net/http"
	"one-api/model"
	"strconv"

	"github.com/gin-gonic/gin"
)

// GetConversationHistories 获取对话历史列表
func GetConversationHistories(c *gin.Context) {
	userId := c.GetInt("id")

	// 获取查询参数
	conversationId := c.Query("conversation_id")
	modelName := c.Query("model_name")
	keyword := c.Query("keyword")

	// 分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	var histories []*model.ConversationHistory
	var total int64
	var err error

	// 根据不同条件查询
	if conversationId != "" {
		// 根据对话ID查询
		histories, total, err = model.GetConversationHistoriesByConversationId(conversationId, pageSize, offset)
	} else if keyword != "" {
		// 搜索查询
		histories, total, err = model.SearchConversationHistories(keyword, userId, modelName, pageSize, offset)
	} else {
		// 根据用户ID查询
		histories, total, err = model.GetConversationHistoriesByUserId(userId, pageSize, offset)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取对话历史失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"histories": histories,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetConversationHistory 获取单个对话历史
func GetConversationHistory(c *gin.Context) {
	userId := c.GetInt("id")
	historyId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的历史记录ID",
		})
		return
	}

	history, err := model.GetConversationHistoryById(historyId)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "对话历史不存在",
		})
		return
	}

	// 检查权限：只能查看自己的对话历史
	if history.UserId != userId && !model.IsAdmin(userId) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "无权访问此对话历史",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    history,
	})
}

// DeleteConversationHistory 删除对话历史
func DeleteConversationHistory(c *gin.Context) {
	userId := c.GetInt("id")
	historyId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的历史记录ID",
		})
		return
	}

	// 先获取记录检查权限
	history, err := model.GetConversationHistoryById(historyId)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "对话历史不存在",
		})
		return
	}

	// 检查权限：只能删除自己的对话历史
	if history.UserId != userId && !model.IsAdmin(userId) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "无权删除此对话历史",
		})
		return
	}

	err = model.DeleteConversationHistory(historyId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "删除对话历史失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "删除成功",
	})
}

// DeleteConversationHistoriesByConversationId 根据对话ID删除对话历史
func DeleteConversationHistoriesByConversationId(c *gin.Context) {
	userId := c.GetInt("id")
	conversationId := c.Param("conversation_id")

	if conversationId == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "对话ID不能为空",
		})
		return
	}

	// 如果不是管理员，需要验证对话是否属于当前用户
	if !model.IsAdmin(userId) {
		histories, _, err := model.GetConversationHistoriesByConversationId(conversationId, 1, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "查询对话历史失败: " + err.Error(),
			})
			return
		}

		if len(histories) > 0 && histories[0].UserId != userId {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "无权删除此对话历史",
			})
			return
		}
	}

	err := model.DeleteConversationHistoriesByConversationId(conversationId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "删除对话历史失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "删除成功",
	})
}

// AdminGetConversationHistories 管理员获取所有对话历史
func AdminGetConversationHistories(c *gin.Context) {
	// 获取查询参数
	userIdStr := c.Query("user_id")
	conversationId := c.Query("conversation_id")
	modelName := c.Query("model_name")
	keyword := c.Query("keyword")

	// 分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	var histories []*model.ConversationHistory
	var total int64
	var err error

	// 解析用户ID
	var userId int
	if userIdStr != "" {
		userId, _ = strconv.Atoi(userIdStr)
	}

	// 根据不同条件查询
	if conversationId != "" {
		histories, total, err = model.GetConversationHistoriesByConversationId(conversationId, pageSize, offset)
	} else if keyword != "" {
		histories, total, err = model.SearchConversationHistories(keyword, userId, modelName, pageSize, offset)
	} else if userId > 0 {
		histories, total, err = model.GetConversationHistoriesByUserId(userId, pageSize, offset)
	} else {
		// 获取所有对话历史
		histories, total, err = model.SearchConversationHistories("", 0, modelName, pageSize, offset)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取对话历史失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"histories": histories,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// AdminDeleteConversationHistory 管理员删除对话历史
func AdminDeleteConversationHistory(c *gin.Context) {
	historyId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的历史记录ID",
		})
		return
	}

	// 检查是否为硬删除
	hardDelete := c.Query("hard") == "true"

	if hardDelete {
		err = model.HardDeleteConversationHistory(historyId)
	} else {
		err = model.DeleteConversationHistory(historyId)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "删除对话历史失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "删除成功",
	})
}

// CleanupOldConversationHistories 清理旧的对话历史
func CleanupOldConversationHistories(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的天数参数",
		})
		return
	}

	count, err := model.CleanupOldConversationHistories(days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "清理失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "清理完成",
		"data": gin.H{
			"deleted_count": count,
		},
	})
}
