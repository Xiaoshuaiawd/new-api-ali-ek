package model

import (
	"encoding/json"
	"one-api/common"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ConversationHistory struct {
	Id             int            `json:"id" gorm:"primaryKey;autoIncrement"`
	ConversationId string         `json:"conversation_id" gorm:"type:varchar(255);index;not null"`
	ModelName      string         `json:"model_name" gorm:"type:varchar(255);index;not null"`
	RawJson        string         `json:"raw_json" gorm:"type:longtext;not null"`
	UserId         int            `json:"user_id" gorm:"index"`
	CreatedAt      time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt      gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// TableName 指定表名
func (ConversationHistory) TableName() string {
	return "conversation_histories"
}

// CreateConversationHistory 创建对话历史记录
func CreateConversationHistory(c *gin.Context, conversationId string, modelName string, rawJson string, userId int) error {
	history := &ConversationHistory{
		ConversationId: conversationId,
		ModelName:      modelName,
		RawJson:        rawJson,
		UserId:         userId,
	}
	
	err := DB.Create(history).Error
	if err != nil {
		common.LogError(c, "failed to create conversation history: "+err.Error())
		return err
	}
	
	return nil
}

// GetConversationHistoryById 根据ID获取对话历史
func GetConversationHistoryById(id int) (*ConversationHistory, error) {
	var history ConversationHistory
	err := DB.First(&history, id).Error
	if err != nil {
		return nil, err
	}
	return &history, nil
}

// GetConversationHistoriesByConversationId 根据对话ID获取对话历史列表
func GetConversationHistoriesByConversationId(conversationId string, limit int, offset int) ([]*ConversationHistory, int64, error) {
	var histories []*ConversationHistory
	var total int64
	
	// 获取总数
	err := DB.Model(&ConversationHistory{}).Where("conversation_id = ?", conversationId).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	
	// 获取分页数据
	err = DB.Where("conversation_id = ?", conversationId).
		Order("created_at desc").
		Limit(limit).
		Offset(offset).
		Find(&histories).Error
	
	if err != nil {
		return nil, 0, err
	}
	
	return histories, total, nil
}

// GetConversationHistoriesByUserId 根据用户ID获取对话历史列表
func GetConversationHistoriesByUserId(userId int, limit int, offset int) ([]*ConversationHistory, int64, error) {
	var histories []*ConversationHistory
	var total int64
	
	// 获取总数
	err := DB.Model(&ConversationHistory{}).Where("user_id = ?", userId).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	
	// 获取分页数据
	err = DB.Where("user_id = ?", userId).
		Order("created_at desc").
		Limit(limit).
		Offset(offset).
		Find(&histories).Error
	
	if err != nil {
		return nil, 0, err
	}
	
	return histories, total, nil
}

// UpdateConversationHistory 更新对话历史
func UpdateConversationHistory(id int, rawJson string) error {
	return DB.Model(&ConversationHistory{}).Where("id = ?", id).Update("raw_json", rawJson).Error
}

// DeleteConversationHistory 软删除对话历史
func DeleteConversationHistory(id int) error {
	return DB.Delete(&ConversationHistory{}, id).Error
}

// DeleteConversationHistoriesByConversationId 根据对话ID软删除对话历史
func DeleteConversationHistoriesByConversationId(conversationId string) error {
	return DB.Where("conversation_id = ?", conversationId).Delete(&ConversationHistory{}).Error
}

// HardDeleteConversationHistory 硬删除对话历史
func HardDeleteConversationHistory(id int) error {
	return DB.Unscoped().Delete(&ConversationHistory{}, id).Error
}

// SearchConversationHistories 搜索对话历史
func SearchConversationHistories(keyword string, userId int, modelName string, limit int, offset int) ([]*ConversationHistory, int64, error) {
	var histories []*ConversationHistory
	var total int64
	
	query := DB.Model(&ConversationHistory{})
	
	// 构建搜索条件
	if keyword != "" {
		query = query.Where("conversation_id LIKE ? OR raw_json LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	
	if userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	
	if modelName != "" {
		query = query.Where("model_name = ?", modelName)
	}
	
	// 获取总数
	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	
	// 获取分页数据
	err = query.Order("created_at desc").
		Limit(limit).
		Offset(offset).
		Find(&histories).Error
	
	if err != nil {
		return nil, 0, err
	}
	
	return histories, total, nil
}

// GetRawJsonAsMap 将RawJson字段解析为map
func (h *ConversationHistory) GetRawJsonAsMap() (map[string]interface{}, error) {
	var result map[string]interface{}
	err := json.Unmarshal([]byte(h.RawJson), &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// SetRawJsonFromMap 从map设置RawJson字段
func (h *ConversationHistory) SetRawJsonFromMap(data map[string]interface{}) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	h.RawJson = string(jsonBytes)
	return nil
}

// CleanupOldConversationHistories 清理旧的对话历史记录
func CleanupOldConversationHistories(days int) (int64, error) {
	cutoffTime := time.Now().AddDate(0, 0, -days)
	result := DB.Where("created_at < ?", cutoffTime).Delete(&ConversationHistory{})
	return result.RowsAffected, result.Error
} 