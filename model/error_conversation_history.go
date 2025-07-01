package model

import (
	"encoding/json"
	"fmt"
	"one-api/common"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetErrorConversationHistoryTableName 获取错误对话历史表名（支持日期分表）
func GetErrorConversationHistoryTableName(date ...time.Time) string {
	if !common.MESDailyPartition {
		return "error_conversation_histories"
	}

	var targetDate time.Time
	if len(date) > 0 {
		targetDate = date[0]
	} else {
		targetDate = time.Now()
	}

	return fmt.Sprintf("error_conversation_histories_%s", targetDate.Format("2006_01_02"))
}

// ensureErrorConversationHistoryTableExists 确保错误对话历史表存在
func ensureErrorConversationHistoryTableExists(tableName string) error {
	if MES_DB.Migrator().HasTable(tableName) {
		return nil
	}

	return MES_DB.Table(tableName).AutoMigrate(&ErrorConversationHistory{})
}

// getErrorConversationHistoryAllTables 获取所有存在的错误对话历史分表
func getErrorConversationHistoryAllTables() []string {
	var tables []string

	if !common.MESDailyPartition {
		return []string{"error_conversation_histories"}
	}

	// 获取数据库中所有以 error_conversation_histories_ 开头的表
	var tableNames []string
	err := MES_DB.Raw("SHOW TABLES LIKE 'error_conversation_histories_%'").Scan(&tableNames).Error
	if err != nil {
		// 如果查询失败，回退到检查最近一年的表
		now := time.Now()
		for i := 0; i < 365; i++ {
			pastDate := now.AddDate(0, 0, -i)
			tableName := GetErrorConversationHistoryTableName(pastDate)
			if MES_DB.Migrator().HasTable(tableName) {
				tables = append(tables, tableName)
			}
		}
		return tables
	}

	tables = append(tables, tableNames...)

	// 添加基础表（如果存在的话）
	if MES_DB.Migrator().HasTable("error_conversation_histories") {
		tables = append(tables, "error_conversation_histories")
	}

	return tables
}

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

	// 获取当天的表名
	tableName := GetErrorConversationHistoryTableName()

	// 确保表存在
	err := ensureErrorConversationHistoryTableExists(tableName)
	if err != nil {
		common.LogError(c, "failed to ensure error conversation history table exists: "+err.Error())
		return err
	}

	err = MES_DB.Table(tableName).Create(history).Error
	if err != nil {
		common.LogError(c, "failed to create error conversation history: "+err.Error())
		return err
	}

	return nil
}

// GetErrorConversationHistoryById 根据ID获取错误对话历史
func GetErrorConversationHistoryById(id int) (*ErrorConversationHistory, error) {
	var history ErrorConversationHistory

	if !common.MESDailyPartition {
		// 不分表，直接查询
		err := MES_DB.Table("error_conversation_histories").First(&history, id).Error
		if err != nil {
			return nil, err
		}
		return &history, nil
	}

	// 分表模式：查询所有存在的表
	tables := getErrorConversationHistoryAllTables()

	for _, tableName := range tables {
		if !MES_DB.Migrator().HasTable(tableName) {
			continue
		}

		err := MES_DB.Table(tableName).First(&history, id).Error
		if err == nil {
			return &history, nil
		}
	}

	return nil, gorm.ErrRecordNotFound
}

// GetErrorConversationHistoriesByConversationId 根据对话ID获取错误对话历史列表
func GetErrorConversationHistoriesByConversationId(conversationId string, limit int, offset int) ([]*ErrorConversationHistory, int64, error) {
	var histories []*ErrorConversationHistory
	var total int64

	if !common.MESDailyPartition {
		// 不分表，直接查询
		err := MES_DB.Table("error_conversation_histories").Model(&ErrorConversationHistory{}).Where("conversation_id = ?", conversationId).Count(&total).Error
		if err != nil {
			return nil, 0, err
		}

		err = MES_DB.Table("error_conversation_histories").Where("conversation_id = ?", conversationId).
			Order("created_at desc").
			Limit(limit).
			Offset(offset).
			Find(&histories).Error

		if err != nil {
			return nil, 0, err
		}

		return histories, total, nil
	}

	// 分表模式：查询所有存在的表
	tables := getErrorConversationHistoryAllTables()

	// 先获取总数
	total = 0
	for _, tableName := range tables {
		var count int64
		err := MES_DB.Table(tableName).Model(&ErrorConversationHistory{}).Where("conversation_id = ?", conversationId).Count(&count).Error
		if err != nil {
			return nil, 0, err
		}
		total += count
	}

	// 获取分页数据
	histories = make([]*ErrorConversationHistory, 0)
	collected := 0
	skipped := 0

	for _, tableName := range tables {
		if collected >= limit {
			break
		}

		var tableHistories []*ErrorConversationHistory
		tableQuery := MES_DB.Table(tableName).Where("conversation_id = ?", conversationId).Order("created_at desc")

		// 计算这个表需要跳过和获取的记录数
		var tableTotal int64
		err := MES_DB.Table(tableName).Model(&ErrorConversationHistory{}).Where("conversation_id = ?", conversationId).Count(&tableTotal).Error
		if err != nil {
			return nil, 0, err
		}

		if skipped < offset {
			toSkip := offset - skipped
			if toSkip >= int(tableTotal) {
				skipped += int(tableTotal)
				continue
			}
			tableQuery = tableQuery.Offset(toSkip)
			skipped = offset
		}

		remaining := limit - collected
		err = tableQuery.Limit(remaining).Find(&tableHistories).Error
		if err != nil {
			return nil, 0, err
		}

		histories = append(histories, tableHistories...)
		collected += len(tableHistories)
	}

	return histories, total, nil
}

// GetErrorConversationHistoriesByUserId 根据用户ID获取错误对话历史列表
func GetErrorConversationHistoriesByUserId(userId int, limit int, offset int) ([]*ErrorConversationHistory, int64, error) {
	var histories []*ErrorConversationHistory
	var total int64

	if !common.MESDailyPartition {
		// 不分表，直接查询
		err := MES_DB.Table("error_conversation_histories").Model(&ErrorConversationHistory{}).Where("user_id = ?", userId).Count(&total).Error
		if err != nil {
			return nil, 0, err
		}

		err = MES_DB.Table("error_conversation_histories").Where("user_id = ?", userId).
			Order("created_at desc").
			Limit(limit).
			Offset(offset).
			Find(&histories).Error

		if err != nil {
			return nil, 0, err
		}

		return histories, total, nil
	}

	// 分表模式：查询所有存在的表
	tables := getErrorConversationHistoryAllTables()

	// 先获取总数
	total = 0
	for _, tableName := range tables {
		var count int64
		err := MES_DB.Table(tableName).Model(&ErrorConversationHistory{}).Where("user_id = ?", userId).Count(&count).Error
		if err != nil {
			return nil, 0, err
		}
		total += count
	}

	// 获取分页数据
	histories = make([]*ErrorConversationHistory, 0)
	collected := 0
	skipped := 0

	for _, tableName := range tables {
		if collected >= limit {
			break
		}

		var tableHistories []*ErrorConversationHistory
		tableQuery := MES_DB.Table(tableName).Where("user_id = ?", userId).Order("created_at desc")

		// 计算这个表需要跳过和获取的记录数
		var tableTotal int64
		err := MES_DB.Table(tableName).Model(&ErrorConversationHistory{}).Where("user_id = ?", userId).Count(&tableTotal).Error
		if err != nil {
			return nil, 0, err
		}

		if skipped < offset {
			toSkip := offset - skipped
			if toSkip >= int(tableTotal) {
				skipped += int(tableTotal)
				continue
			}
			tableQuery = tableQuery.Offset(toSkip)
			skipped = offset
		}

		remaining := limit - collected
		err = tableQuery.Limit(remaining).Find(&tableHistories).Error
		if err != nil {
			return nil, 0, err
		}

		histories = append(histories, tableHistories...)
		collected += len(tableHistories)
	}

	return histories, total, nil
}

// DeleteErrorConversationHistory 软删除错误对话历史
func DeleteErrorConversationHistory(id int) error {
	if !common.MESDailyPartition {
		return MES_DB.Table("error_conversation_histories").Delete(&ErrorConversationHistory{}, id).Error
	}

	// 分表模式：需要在所有表中查找记录
	tables := getErrorConversationHistoryAllTables()
	for _, tableName := range tables {
		if !MES_DB.Migrator().HasTable(tableName) {
			continue
		}

		result := MES_DB.Table(tableName).Delete(&ErrorConversationHistory{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			return nil // 找到并删除了记录
		}
	}

	return gorm.ErrRecordNotFound
}

// DeleteErrorConversationHistoriesByConversationId 根据对话ID软删除错误对话历史
func DeleteErrorConversationHistoriesByConversationId(conversationId string) error {
	if !common.MESDailyPartition {
		return MES_DB.Table("error_conversation_histories").Where("conversation_id = ?", conversationId).Delete(&ErrorConversationHistory{}).Error
	}

	// 分表模式：需要在所有表中删除记录
	tables := getErrorConversationHistoryAllTables()
	var hasError error
	for _, tableName := range tables {
		if !MES_DB.Migrator().HasTable(tableName) {
			continue
		}

		err := MES_DB.Table(tableName).Where("conversation_id = ?", conversationId).Delete(&ErrorConversationHistory{}).Error
		if err != nil {
			hasError = err
		}
	}

	return hasError
}

// HardDeleteErrorConversationHistory 硬删除错误对话历史
func HardDeleteErrorConversationHistory(id int) error {
	if !common.MESDailyPartition {
		return MES_DB.Table("error_conversation_histories").Unscoped().Delete(&ErrorConversationHistory{}, id).Error
	}

	// 分表模式：需要在所有表中查找记录
	tables := getErrorConversationHistoryAllTables()
	for _, tableName := range tables {
		if !MES_DB.Migrator().HasTable(tableName) {
			continue
		}

		result := MES_DB.Table(tableName).Unscoped().Delete(&ErrorConversationHistory{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			return nil // 找到并删除了记录
		}
	}

	return gorm.ErrRecordNotFound
}

// SearchErrorConversationHistories 搜索错误对话历史
func SearchErrorConversationHistories(keyword string, userId int, modelName string, limit int, offset int) ([]*ErrorConversationHistory, int64, error) {
	var histories []*ErrorConversationHistory
	var total int64

	if !common.MESDailyPartition {
		// 不分表，直接查询
		query := MES_DB.Table("error_conversation_histories").Model(&ErrorConversationHistory{})

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

	// 分表模式：查询所有存在的表
	tables := getErrorConversationHistoryAllTables()

	// 先获取总数
	total = 0
	for _, tableName := range tables {
		query := MES_DB.Table(tableName).Model(&ErrorConversationHistory{})

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

		var count int64
		err := query.Count(&count).Error
		if err != nil {
			return nil, 0, err
		}
		total += count
	}

	// 获取分页数据
	histories = make([]*ErrorConversationHistory, 0)
	collected := 0
	skipped := 0

	for _, tableName := range tables {
		if collected >= limit {
			break
		}

		tableQuery := MES_DB.Table(tableName)

		// 构建搜索条件
		if keyword != "" {
			tableQuery = tableQuery.Where("conversation_id LIKE ? OR raw_json LIKE ? OR error_message LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
		}

		if userId > 0 {
			tableQuery = tableQuery.Where("user_id = ?", userId)
		}

		if modelName != "" {
			tableQuery = tableQuery.Where("model_name = ?", modelName)
		}

		tableQuery = tableQuery.Order("created_at desc")

		// 计算这个表需要跳过和获取的记录数
		var tableTotal int64
		countQuery := MES_DB.Table(tableName).Model(&ErrorConversationHistory{})
		if keyword != "" {
			countQuery = countQuery.Where("conversation_id LIKE ? OR raw_json LIKE ? OR error_message LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
		}
		if userId > 0 {
			countQuery = countQuery.Where("user_id = ?", userId)
		}
		if modelName != "" {
			countQuery = countQuery.Where("model_name = ?", modelName)
		}
		err := countQuery.Count(&tableTotal).Error
		if err != nil {
			return nil, 0, err
		}

		if skipped < offset {
			toSkip := offset - skipped
			if toSkip >= int(tableTotal) {
				skipped += int(tableTotal)
				continue
			}
			tableQuery = tableQuery.Offset(toSkip)
			skipped = offset
		}

		var tableHistories []*ErrorConversationHistory
		remaining := limit - collected
		err = tableQuery.Limit(remaining).Find(&tableHistories).Error
		if err != nil {
			return nil, 0, err
		}

		histories = append(histories, tableHistories...)
		collected += len(tableHistories)
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

	if !common.MESDailyPartition {
		result := MES_DB.Table("error_conversation_histories").Where("created_at < ?", cutoffTime).Delete(&ErrorConversationHistory{})
		return result.RowsAffected, result.Error
	}

	// 分表模式：清理旧表或旧记录
	var totalDeleted int64 = 0
	now := time.Now()

	// 检查过去180天的表（比较保守的范围）
	for i := days; i < 180; i++ {
		pastDate := now.AddDate(0, 0, -i)
		tableName := GetErrorConversationHistoryTableName(pastDate)

		if !MES_DB.Migrator().HasTable(tableName) {
			continue
		}

		// 如果整个表都过期了，可以选择删除整个表
		if pastDate.Before(cutoffTime.AddDate(0, 0, -1)) {
			// 删除表中的所有记录
			result := MES_DB.Table(tableName).Where("1 = 1").Delete(&ErrorConversationHistory{})
			if result.Error != nil {
				return totalDeleted, result.Error
			}
			totalDeleted += result.RowsAffected
		} else {
			// 部分记录过期，只删除过期记录
			result := MES_DB.Table(tableName).Where("created_at < ?", cutoffTime).Delete(&ErrorConversationHistory{})
			if result.Error != nil {
				return totalDeleted, result.Error
			}
			totalDeleted += result.RowsAffected
		}
	}

	return totalDeleted, nil
}
