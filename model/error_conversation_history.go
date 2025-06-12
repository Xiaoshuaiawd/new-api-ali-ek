package model

import (
	"encoding/json"
	"one-api/common"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ErrorConversationHistory struct {
	Id             int            `json:"id" gorm:"primaryKey;autoIncrement"`
	ConversationId string         `json:"conversation_id" gorm:"type:varchar(255);index;not null"`
	ModelName      string         `json:"model_name" gorm:"type:varchar(255);index;not null"`
	RawJson        string         `json:"raw_json" gorm:"type:longtext;not null"`
	ErrorMessage   string         `json:"error_message" gorm:"type:text"`
	ErrorCode      string         `json:"error_code" gorm:"type:varchar(100)"`
	UserId         int            `json:"user_id" gorm:"index"`
	CreatedAt      time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt      gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// TableName 指定表名
func (ErrorConversationHistory) TableName() string {
	return "error_conversation_histories"
}

// CreateErrorConversationHistory 创建错误对话历史记录
func CreateErrorConversationHistory(c *gin.Context, conversationId string, modelName string, rawJson string, errorMessage string, errorCode string, userId int) error {
	history := &ErrorConversationHistory{
		ConversationId: conversationId,
		ModelName:      modelName,
		RawJson:        rawJson,
		ErrorMessage:   errorMessage,
		ErrorCode:      errorCode,
		UserId:         userId,
	}

	err := DB.Create(history).Error
	if err != nil {
		common.LogError(c, "failed to create error conversation history: "+err.Error())
		return err
	}

	return nil
}

// GetErrorConversationHistoryById 根据ID获取错误对话历史
func GetErrorConversationHistoryById(id int) (*ErrorConversationHistory, error) {
	var history ErrorConversationHistory
	err := DB.First(&history, id).Error
	if err != nil {
		return nil, err
	}
	return &history, nil
}

// GetErrorConversationHistoriesByConversationId 根据对话ID获取错误对话历史列表
func GetErrorConversationHistoriesByConversationId(conversationId string, limit int, offset int) ([]*ErrorConversationHistory, int64, error) {
	var histories []*ErrorConversationHistory
	var total int64

	// 获取总数
	err := DB.Model(&ErrorConversationHistory{}).Where("conversation_id = ?", conversationId).Count(&total).Error
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

// GetErrorConversationHistoriesByUserId 根据用户ID获取错误对话历史列表
func GetErrorConversationHistoriesByUserId(userId int, limit int, offset int) ([]*ErrorConversationHistory, int64, error) {
	var histories []*ErrorConversationHistory
	var total int64

	// 获取总数
	err := DB.Model(&ErrorConversationHistory{}).Where("user_id = ?", userId).Count(&total).Error
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

// DeleteErrorConversationHistory 软删除错误对话历史
func DeleteErrorConversationHistory(id int) error {
	return DB.Delete(&ErrorConversationHistory{}, id).Error
}

// DeleteErrorConversationHistoriesByConversationId 根据对话ID软删除错误对话历史
func DeleteErrorConversationHistoriesByConversationId(conversationId string) error {
	return DB.Where("conversation_id = ?", conversationId).Delete(&ErrorConversationHistory{}).Error
}

// HardDeleteErrorConversationHistory 硬删除错误对话历史
func HardDeleteErrorConversationHistory(id int) error {
	return DB.Unscoped().Delete(&ErrorConversationHistory{}, id).Error
}

// SearchErrorConversationHistories 搜索错误对话历史
func SearchErrorConversationHistories(keyword string, userId int, modelName string, limit int, offset int) ([]*ErrorConversationHistory, int64, error) {
	var histories []*ErrorConversationHistory
	var total int64

	query := DB.Model(&ErrorConversationHistory{})

	// 构建搜索条件
	if keyword != "" {
		query = query.Where("conversation_id LIKE ? OR raw_json LIKE ? OR error_message LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
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
func (h *ErrorConversationHistory) GetRawJsonAsMap() (map[string]interface{}, error) {
	var result map[string]interface{}
	err := json.Unmarshal([]byte(h.RawJson), &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// SetRawJsonFromMap 从map设置RawJson字段
func (h *ErrorConversationHistory) SetRawJsonFromMap(data map[string]interface{}) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	h.RawJson = string(jsonBytes)
	return nil
}

// CleanupOldErrorConversationHistories 清理旧的错误对话历史记录
func CleanupOldErrorConversationHistories(days int) (int64, error) {
	cutoffTime := time.Now().AddDate(0, 0, -days)
	result := DB.Where("created_at < ?", cutoffTime).Delete(&ErrorConversationHistory{})
	return result.RowsAffected, result.Error
}
