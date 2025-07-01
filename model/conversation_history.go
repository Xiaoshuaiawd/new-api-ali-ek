package model

import (
	"encoding/json"
	"fmt"
	"one-api/common"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetConversationHistoryTableName 获取对话历史表名（支持日期分表）
func GetConversationHistoryTableName(date ...time.Time) string {
	if !common.MESDailyPartition {
		return "conversation_histories"
	}

	var targetDate time.Time
	if len(date) > 0 {
		targetDate = date[0]
	} else {
		targetDate = time.Now()
	}

	return fmt.Sprintf("conversation_histories_%s", targetDate.Format("2006_01_02"))
}

// ensureConversationHistoryTableExists 确保对话历史表存在
func ensureConversationHistoryTableExists(tableName string) error {
	if MES_DB.Migrator().HasTable(tableName) {
		return nil
	}

	return MES_DB.Table(tableName).AutoMigrate(&ConversationHistory{})
}

// getConversationHistoryAllTables 获取所有存在的对话历史分表
func getConversationHistoryAllTables() []string {
	var tables []string

	if !common.MESDailyPartition {
		return []string{"conversation_histories"}
	}

	// 获取数据库中所有以 conversation_histories_ 开头的表
	var tableNames []string
	err := MES_DB.Raw("SHOW TABLES LIKE 'conversation_histories_%'").Scan(&tableNames).Error
	if err != nil {
		// 如果查询失败，回退到检查最近一年的表
		now := time.Now()
		for i := 0; i < 365; i++ {
			pastDate := now.AddDate(0, 0, -i)
			tableName := GetConversationHistoryTableName(pastDate)
			if MES_DB.Migrator().HasTable(tableName) {
				tables = append(tables, tableName)
			}
		}
		return tables
	}

	tables = append(tables, tableNames...)

	// 添加基础表（如果存在的话）
	if MES_DB.Migrator().HasTable("conversation_histories") {
		tables = append(tables, "conversation_histories")
	}

	return tables
}

// getDateRange 获取日期范围内的所有表名
func getConversationHistoryDateRange(startTime, endTime time.Time) []string {
	var tables []string
	current := startTime
	for current.Before(endTime) || current.Equal(endTime) {
		tableName := GetConversationHistoryTableName(current)
		tables = append(tables, tableName)
		current = current.AddDate(0, 0, 1)
	}
	return tables
}

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

	// 获取当天的表名
	tableName := GetConversationHistoryTableName()

	// 确保表存在（双重保障）
	err := ensureConversationHistoryTableExists(tableName)
	if err != nil {
		common.LogError(c, "failed to ensure conversation history table exists: "+err.Error())
		return err
	}

	err = MES_DB.Table(tableName).Create(history).Error
	if err != nil {
		common.LogError(c, "failed to create conversation history: "+err.Error())
		return err
	}

	return nil
}

// GetConversationHistoryById 根据ID获取对话历史
func GetConversationHistoryById(id int) (*ConversationHistory, error) {
	var history ConversationHistory

	if !common.MESDailyPartition {
		// 不分表，直接查询
		err := MES_DB.Table("conversation_histories").First(&history, id).Error
		if err != nil {
			return nil, err
		}
		return &history, nil
	}

	// 分表模式：查询所有存在的表
	tables := getConversationHistoryAllTables()

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

// GetConversationHistoriesByConversationId 根据对话ID获取对话历史列表
func GetConversationHistoriesByConversationId(conversationId string, limit int, offset int) ([]*ConversationHistory, int64, error) {
	var histories []*ConversationHistory
	var total int64

	if !common.MESDailyPartition {
		// 不分表，直接查询
		err := MES_DB.Table("conversation_histories").Model(&ConversationHistory{}).Where("conversation_id = ?", conversationId).Count(&total).Error
		if err != nil {
			return nil, 0, err
		}

		err = MES_DB.Table("conversation_histories").Where("conversation_id = ?", conversationId).
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
	tables := getConversationHistoryAllTables()

	// 先获取总数
	total = 0
	for _, tableName := range tables {
		var count int64
		err := MES_DB.Table(tableName).Model(&ConversationHistory{}).Where("conversation_id = ?", conversationId).Count(&count).Error
		if err != nil {
			return nil, 0, err
		}
		total += count
	}

	// 获取分页数据 - 这里需要跨表查询，比较复杂，我们先从最新的表开始查
	histories = make([]*ConversationHistory, 0)
	collected := 0
	skipped := 0

	for _, tableName := range tables {
		if collected >= limit {
			break
		}

		var tableHistories []*ConversationHistory
		tableQuery := MES_DB.Table(tableName).Where("conversation_id = ?", conversationId).Order("created_at desc")

		// 计算这个表需要跳过和获取的记录数
		var tableTotal int64
		err := MES_DB.Table(tableName).Model(&ConversationHistory{}).Where("conversation_id = ?", conversationId).Count(&tableTotal).Error
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

// GetConversationHistoriesByUserId 根据用户ID获取对话历史列表
func GetConversationHistoriesByUserId(userId int, limit int, offset int) ([]*ConversationHistory, int64, error) {
	var histories []*ConversationHistory
	var total int64

	if !common.MESDailyPartition {
		// 不分表，直接查询
		err := MES_DB.Table("conversation_histories").Model(&ConversationHistory{}).Where("user_id = ?", userId).Count(&total).Error
		if err != nil {
			return nil, 0, err
		}

		err = MES_DB.Table("conversation_histories").Where("user_id = ?", userId).
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
	tables := getConversationHistoryAllTables()

	// 先获取总数
	total = 0
	for _, tableName := range tables {
		var count int64
		err := MES_DB.Table(tableName).Model(&ConversationHistory{}).Where("user_id = ?", userId).Count(&count).Error
		if err != nil {
			return nil, 0, err
		}
		total += count
	}

	// 获取分页数据
	histories = make([]*ConversationHistory, 0)
	collected := 0
	skipped := 0

	for _, tableName := range tables {
		if collected >= limit {
			break
		}

		var tableHistories []*ConversationHistory
		tableQuery := MES_DB.Table(tableName).Where("user_id = ?", userId).Order("created_at desc")

		// 计算这个表需要跳过和获取的记录数
		var tableTotal int64
		err := MES_DB.Table(tableName).Model(&ConversationHistory{}).Where("user_id = ?", userId).Count(&tableTotal).Error
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

// UpdateConversationHistory 更新对话历史
func UpdateConversationHistory(id int, rawJson string) error {
	if !common.MESDailyPartition {
		return MES_DB.Table("conversation_histories").Model(&ConversationHistory{}).Where("id = ?", id).Update("raw_json", rawJson).Error
	}

	// 分表模式：需要在所有表中查找记录
	tables := getConversationHistoryAllTables()
	for _, tableName := range tables {
		if !MES_DB.Migrator().HasTable(tableName) {
			continue
		}

		result := MES_DB.Table(tableName).Model(&ConversationHistory{}).Where("id = ?", id).Update("raw_json", rawJson)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			return nil // 找到并更新了记录
		}
	}

	return gorm.ErrRecordNotFound
}

// DeleteConversationHistory 软删除对话历史
func DeleteConversationHistory(id int) error {
	if !common.MESDailyPartition {
		return MES_DB.Table("conversation_histories").Delete(&ConversationHistory{}, id).Error
	}

	// 分表模式：需要在所有表中查找记录
	tables := getConversationHistoryAllTables()
	for _, tableName := range tables {
		if !MES_DB.Migrator().HasTable(tableName) {
			continue
		}

		result := MES_DB.Table(tableName).Delete(&ConversationHistory{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			return nil // 找到并删除了记录
		}
	}

	return gorm.ErrRecordNotFound
}

// DeleteConversationHistoriesByConversationId 根据对话ID软删除对话历史
func DeleteConversationHistoriesByConversationId(conversationId string) error {
	if !common.MESDailyPartition {
		return MES_DB.Table("conversation_histories").Where("conversation_id = ?", conversationId).Delete(&ConversationHistory{}).Error
	}

	// 分表模式：需要在所有表中删除记录
	tables := getConversationHistoryAllTables()
	var hasError error
	for _, tableName := range tables {
		if !MES_DB.Migrator().HasTable(tableName) {
			continue
		}

		err := MES_DB.Table(tableName).Where("conversation_id = ?", conversationId).Delete(&ConversationHistory{}).Error
		if err != nil {
			hasError = err
		}
	}

	return hasError
}

// HardDeleteConversationHistory 硬删除对话历史
func HardDeleteConversationHistory(id int) error {
	if !common.MESDailyPartition {
		return MES_DB.Table("conversation_histories").Unscoped().Delete(&ConversationHistory{}, id).Error
	}

	// 分表模式：需要在所有表中查找记录
	tables := getConversationHistoryAllTables()
	for _, tableName := range tables {
		if !MES_DB.Migrator().HasTable(tableName) {
			continue
		}

		result := MES_DB.Table(tableName).Unscoped().Delete(&ConversationHistory{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			return nil // 找到并删除了记录
		}
	}

	return gorm.ErrRecordNotFound
}

// SearchConversationHistories 搜索对话历史
func SearchConversationHistories(keyword string, userId int, modelName string, limit int, offset int) ([]*ConversationHistory, int64, error) {
	var histories []*ConversationHistory
	var total int64

	if !common.MESDailyPartition {
		// 不分表，直接查询
		query := MES_DB.Table("conversation_histories").Model(&ConversationHistory{})

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

	// 分表模式：查询所有存在的表
	tables := getConversationHistoryAllTables()

	// 先获取总数
	total = 0
	for _, tableName := range tables {
		query := MES_DB.Table(tableName).Model(&ConversationHistory{})

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

		var count int64
		err := query.Count(&count).Error
		if err != nil {
			return nil, 0, err
		}
		total += count
	}

	// 获取分页数据
	histories = make([]*ConversationHistory, 0)
	collected := 0
	skipped := 0

	for _, tableName := range tables {
		if collected >= limit {
			break
		}

		tableQuery := MES_DB.Table(tableName)

		// 构建搜索条件
		if keyword != "" {
			tableQuery = tableQuery.Where("conversation_id LIKE ? OR raw_json LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
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
		countQuery := MES_DB.Table(tableName).Model(&ConversationHistory{})
		if keyword != "" {
			countQuery = countQuery.Where("conversation_id LIKE ? OR raw_json LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
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

		var tableHistories []*ConversationHistory
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

	if !common.MESDailyPartition {
		result := MES_DB.Table("conversation_histories").Where("created_at < ?", cutoffTime).Delete(&ConversationHistory{})
		return result.RowsAffected, result.Error
	}

	// 分表模式：清理旧表或旧记录
	var totalDeleted int64 = 0
	now := time.Now()

	// 检查过去180天的表（比较保守的范围）
	for i := days; i < 180; i++ {
		pastDate := now.AddDate(0, 0, -i)
		tableName := GetConversationHistoryTableName(pastDate)

		if !MES_DB.Migrator().HasTable(tableName) {
			continue
		}

		// 如果整个表都过期了，可以选择删除整个表
		if pastDate.Before(cutoffTime.AddDate(0, 0, -1)) {
			// 删除表中的所有记录
			result := MES_DB.Table(tableName).Where("1 = 1").Delete(&ConversationHistory{})
			if result.Error != nil {
				return totalDeleted, result.Error
			}
			totalDeleted += result.RowsAffected
		} else {
			// 部分记录过期，只删除过期记录
			result := MES_DB.Table(tableName).Where("created_at < ?", cutoffTime).Delete(&ConversationHistory{})
			if result.Error != nil {
				return totalDeleted, result.Error
			}
			totalDeleted += result.RowsAffected
		}
	}

	return totalDeleted, nil
}

// EnsureTodayTablesExist 确保今天的分表存在（系统启动时调用）
func EnsureTodayTablesExist() error {
	if !common.MESDailyPartition {
		return nil
	}

	// 获取今天的表名
	todayTableName := GetConversationHistoryTableName()
	todayErrorTableName := GetErrorConversationHistoryTableName()

	common.SysLog(fmt.Sprintf("Ensuring today's partition tables exist: %s, %s", todayTableName, todayErrorTableName))

	// 创建今天的对话历史表
	err := ensureConversationHistoryTableExists(todayTableName)
	if err != nil {
		return fmt.Errorf("failed to ensure conversation history table %s: %v", todayTableName, err)
	}

	// 创建今天的错误对话历史表
	err = ensureErrorConversationHistoryTableExists(todayErrorTableName)
	if err != nil {
		return fmt.Errorf("failed to ensure error conversation history table %s: %v", todayErrorTableName, err)
	}

	common.SysLog("Today's partition tables created successfully")
	return nil
}

// 用于跟踪当前的日期和表
var (
	currentPartitionDate string
	partitionMutex       sync.RWMutex
)

// StartPartitionTableMonitor 启动分表监控服务
func StartPartitionTableMonitor() {
	if !common.MESDailyPartition {
		return
	}

	// 初始化当前分区日期
	updateCurrentPartitionDate()

	// 启动监控协程
	gopool.Go(func() {
		ticker := time.NewTicker(1 * time.Minute) // 每分钟检查一次
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				checkAndCreateNewPartition()
			}
		}
	})

	common.SysLog("Partition table monitor started")
}

// updateCurrentPartitionDate 更新当前分区日期
func updateCurrentPartitionDate() {
	partitionMutex.Lock()
	defer partitionMutex.Unlock()
	currentPartitionDate = time.Now().Format("2006_01_02")
}

// getCurrentPartitionDate 获取当前分区日期
func getCurrentPartitionDate() string {
	partitionMutex.RLock()
	defer partitionMutex.RUnlock()
	return currentPartitionDate
}

// checkAndCreateNewPartition 检查并创建新的分区表
func checkAndCreateNewPartition() {
	now := time.Now()
	todayString := now.Format("2006_01_02")

	// 检查是否跨日了
	if getCurrentPartitionDate() != todayString {
		common.SysLog(fmt.Sprintf("Date changed from %s to %s, creating new partition tables", getCurrentPartitionDate(), todayString))

		// 更新当前日期
		updateCurrentPartitionDate()

		// 创建新的分区表
		err := createPartitionTablesForDate(now)
		if err != nil {
			common.SysError(fmt.Sprintf("Failed to create partition tables for %s: %v", todayString, err))
		} else {
			common.SysLog(fmt.Sprintf("Successfully created partition tables for %s", todayString))
		}

		// 预创建明天的表（可选，提前准备）
		tomorrow := now.AddDate(0, 0, 1)
		err = createPartitionTablesForDate(tomorrow)
		if err != nil {
			common.SysError(fmt.Sprintf("Failed to pre-create partition tables for %s: %v", tomorrow.Format("2006_01_02"), err))
		} else {
			common.SysLog(fmt.Sprintf("Successfully pre-created partition tables for %s", tomorrow.Format("2006_01_02")))
		}
	}
}

// createPartitionTablesForDate 为指定日期创建分区表
func createPartitionTablesForDate(date time.Time) error {
	tableName := GetConversationHistoryTableName(date)
	errorTableName := GetErrorConversationHistoryTableName(date)

	// 创建对话历史表
	err := ensureConversationHistoryTableExists(tableName)
	if err != nil {
		return fmt.Errorf("failed to create conversation history table %s: %v", tableName, err)
	}

	// 创建错误对话历史表
	err = ensureErrorConversationHistoryTableExists(errorTableName)
	if err != nil {
		return fmt.Errorf("failed to create error conversation history table %s: %v", errorTableName, err)
	}

	return nil
}

// PreCreateTomorrowTables 预创建明天的分区表（可以手动调用）
func PreCreateTomorrowTables() error {
	if !common.MESDailyPartition {
		return nil
	}

	tomorrow := time.Now().AddDate(0, 0, 1)
	err := createPartitionTablesForDate(tomorrow)
	if err != nil {
		return fmt.Errorf("failed to pre-create tomorrow's tables: %v", err)
	}

	common.SysLog(fmt.Sprintf("Pre-created partition tables for %s", tomorrow.Format("2006_01_02")))
	return nil
}

// SimulateDateChange 模拟日期变化（仅用于测试）
func SimulateDateChange(targetDate time.Time) error {
	if !common.MESDailyPartition {
		return fmt.Errorf("daily partition is not enabled")
	}

	common.SysLog(fmt.Sprintf("Simulating date change to %s", targetDate.Format("2006-01-02")))

	// 手动更新当前分区日期
	partitionMutex.Lock()
	currentPartitionDate = targetDate.Format("2006_01_02")
	partitionMutex.Unlock()

	// 创建目标日期的分区表
	err := createPartitionTablesForDate(targetDate)
	if err != nil {
		return fmt.Errorf("failed to create partition tables for %s: %v", targetDate.Format("2006_01_02"), err)
	}

	common.SysLog(fmt.Sprintf("Successfully simulated date change and created tables for %s", targetDate.Format("2006-01-02")))
	return nil
}

// GetPartitionTableStats 获取分区表统计信息
func GetPartitionTableStats() map[string]interface{} {
	if !common.MESDailyPartition {
		return map[string]interface{}{
			"partition_enabled": false,
		}
	}

	stats := map[string]interface{}{
		"partition_enabled": true,
		"current_date":      getCurrentPartitionDate(),
		"monitor_running":   true,
	}

	// 获取所有存在的分区表
	tables := getConversationHistoryAllTables()
	stats["existing_tables"] = tables
	stats["total_partitions"] = len(tables)

	return stats
}
