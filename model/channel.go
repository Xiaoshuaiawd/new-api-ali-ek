package model

import (
	"encoding/json"
	"fmt"
	"one-api/common"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

type Channel struct {
	Id                 int     `json:"id"`
	Type               int     `json:"type" gorm:"default:0"`
	Key                string  `json:"key" gorm:"not null"`
	OpenAIOrganization *string `json:"openai_organization"`
	TestModel          *string `json:"test_model"`
	Status             int     `json:"status" gorm:"default:1"`
	Name               string  `json:"name" gorm:"index"`
	Weight             *uint   `json:"weight" gorm:"default:0"`
	CreatedTime        int64   `json:"created_time" gorm:"bigint"`
	TestTime           int64   `json:"test_time" gorm:"bigint"`
	ResponseTime       int     `json:"response_time"` // in milliseconds
	BaseURL            *string `json:"base_url" gorm:"column:base_url;default:''"`
	Other              string  `json:"other"`
	Balance            float64 `json:"balance"` // in USD
	BalanceUpdatedTime int64   `json:"balance_updated_time" gorm:"bigint"`
	Models             string  `json:"models"`
	Group              string  `json:"group" gorm:"type:varchar(64);default:'default'"`
	UsedQuota          int64   `json:"used_quota" gorm:"bigint;default:0"`
	ModelMapping       *string `json:"model_mapping" gorm:"type:text"`
	//MaxInputTokens     *int    `json:"max_input_tokens" gorm:"default:0"`
	StatusCodeMapping *string `json:"status_code_mapping" gorm:"type:varchar(1024);default:''"`
	Priority          *int64  `json:"priority" gorm:"bigint;default:0"`
	AutoBan           *int    `json:"auto_ban" gorm:"default:1"`
	OtherInfo         string  `json:"other_info"`
	Tag               *string `json:"tag" gorm:"index"`
	Setting           *string `json:"setting" gorm:"type:text"`
	ParamOverride     *string `json:"param_override" gorm:"type:text"`
	// 渠道限额相关字段
	QuotaLimitEnabled *bool  `json:"quota_limit_enabled" gorm:"default:false"` // 是否启用限额
	QuotaLimit        *int64 `json:"quota_limit" gorm:"bigint;default:0"`      // 限额值（以500000token为1刀计算）

	// 渠道次数限制相关字段
	CountLimitEnabled *bool  `json:"count_limit_enabled" gorm:"default:false"`    // 是否启用次数限制
	CountLimit        *int64 `json:"count_limit" gorm:"bigint;default:0"`         // 次数限制值
	UsedCount         int64  `json:"used_count" gorm:"bigint;default:0"`          // 已使用次数
	AutoResetEnabled  *bool  `json:"auto_reset_enabled" gorm:"default:false"`     // 是否启用自动重置
	AutoResetInterval *int64 `json:"auto_reset_interval" gorm:"bigint;default:0"` // 自动重置间隔（秒）
	LastResetTime     int64  `json:"last_reset_time" gorm:"bigint;default:0"`     // 禁用时间（用于自动重置计时）

	// 渠道RPM限制相关字段
	RPMLimitEnabled   *bool  `json:"rpm_limit_enabled" gorm:"default:false"`      // 是否启用RPM限制
	RPMLimit          *int64 `json:"rpm_limit" gorm:"bigint;default:0"`           // RPM限制值（每分钟请求次数）
	LastMinuteTime    int64  `json:"last_minute_time" gorm:"bigint;default:0"`    // 上一分钟的时间戳
	CurrentMinuteUsed int64  `json:"current_minute_used" gorm:"bigint;default:0"` // 当前分钟已使用次数
}

func (channel *Channel) GetModels() []string {
	if channel.Models == "" {
		return []string{}
	}
	return strings.Split(strings.Trim(channel.Models, ","), ",")
}

func (channel *Channel) GetGroups() []string {
	if channel.Group == "" {
		return []string{}
	}
	groups := strings.Split(strings.Trim(channel.Group, ","), ",")
	for i, group := range groups {
		groups[i] = strings.TrimSpace(group)
	}
	return groups
}

func (channel *Channel) GetOtherInfo() map[string]interface{} {
	otherInfo := make(map[string]interface{})
	if channel.OtherInfo != "" {
		err := json.Unmarshal([]byte(channel.OtherInfo), &otherInfo)
		if err != nil {
			common.SysError("failed to unmarshal other info: " + err.Error())
		}
	}
	return otherInfo
}

func (channel *Channel) SetOtherInfo(otherInfo map[string]interface{}) {
	otherInfoBytes, err := json.Marshal(otherInfo)
	if err != nil {
		common.SysError("failed to marshal other info: " + err.Error())
		return
	}
	channel.OtherInfo = string(otherInfoBytes)
}

func (channel *Channel) GetTag() string {
	if channel.Tag == nil {
		return ""
	}
	return *channel.Tag
}

func (channel *Channel) SetTag(tag string) {
	channel.Tag = &tag
}

func (channel *Channel) GetAutoBan() bool {
	if channel.AutoBan == nil {
		return false
	}
	return *channel.AutoBan == 1
}

func (channel *Channel) GetQuotaLimitEnabled() bool {
	if channel.QuotaLimitEnabled == nil {
		return false
	}
	return *channel.QuotaLimitEnabled
}

func (channel *Channel) GetQuotaLimit() int64 {
	if channel.QuotaLimit == nil {
		return 0
	}
	return *channel.QuotaLimit
}

func (channel *Channel) SetQuotaLimitEnabled(enabled bool) {
	channel.QuotaLimitEnabled = &enabled
}

func (channel *Channel) SetQuotaLimit(limit int64) {
	channel.QuotaLimit = &limit
}

func (channel *Channel) GetCountLimitEnabled() bool {
	if channel.CountLimitEnabled == nil {
		return false
	}
	return *channel.CountLimitEnabled
}

func (channel *Channel) GetCountLimit() int64 {
	if channel.CountLimit == nil {
		return 0
	}
	return *channel.CountLimit
}

func (channel *Channel) SetCountLimitEnabled(enabled bool) {
	channel.CountLimitEnabled = &enabled
}

func (channel *Channel) SetCountLimit(limit int64) {
	channel.CountLimit = &limit
}

func (channel *Channel) GetAutoResetEnabled() bool {
	if channel.AutoResetEnabled == nil {
		return false
	}
	return *channel.AutoResetEnabled
}

func (channel *Channel) GetAutoResetInterval() int64 {
	if channel.AutoResetInterval == nil {
		return 0
	}
	return *channel.AutoResetInterval
}

func (channel *Channel) SetAutoResetEnabled(enabled bool) {
	channel.AutoResetEnabled = &enabled
}

func (channel *Channel) SetAutoResetInterval(interval int64) {
	channel.AutoResetInterval = &interval
}

// CheckQuotaLimit 检查渠道是否超过限额
func (channel *Channel) CheckQuotaLimit() bool {
	// 检查额度限制
	if channel.GetQuotaLimitEnabled() {
		limit := channel.GetQuotaLimit()
		if limit > 0 && channel.UsedQuota >= limit {
			return true
		}
	}

	// 检查次数限制
	if channel.GetCountLimitEnabled() {
		limit := channel.GetCountLimit()
		if limit > 0 && channel.UsedCount >= limit {
			return true
		}
	}

	return false // 未启用任何限制或未达到限制
}

// CheckCountLimit 检查渠道是否超过次数限制
func (channel *Channel) CheckCountLimit() bool {
	if !channel.GetCountLimitEnabled() {
		return false // 未启用次数限制，不受限制
	}

	limit := channel.GetCountLimit()
	if limit <= 0 {
		return false // 限制为0或负数，不受限制
	}

	// 注意：自动重置逻辑现在由定时任务 CheckAndResetChannels() 处理
	// 这里不再处理自动重置，只检查是否超过限制

	return channel.UsedCount >= limit
}

// CheckRPMLimit 检查渠道是否超过RPM限制
func (channel *Channel) CheckRPMLimit() bool {
	if !channel.GetRPMLimitEnabled() {
		return false // 未启用RPM限制，不受限制
	}

	limit := channel.GetRPMLimit()
	if limit <= 0 {
		return false // 限制为0或负数，不受限制
	}

	currentTime := time.Now().Unix()
	currentMinute := currentTime / 60 // 获取当前分钟的时间戳

	// 如果是新的一分钟，重置计数器
	if channel.LastMinuteTime != currentMinute {
		channel.LastMinuteTime = currentMinute
		channel.CurrentMinuteUsed = 0
	}

	// 检查是否超过RPM限制
	return channel.CurrentMinuteUsed >= limit
}

// IncrementRPMUsage 增加RPM使用计数
func (channel *Channel) IncrementRPMUsage() {
	if !channel.GetRPMLimitEnabled() {
		return
	}

	currentTime := time.Now().Unix()
	currentMinute := currentTime / 60

	// 如果是新的一分钟，重置计数器
	if channel.LastMinuteTime != currentMinute {
		channel.LastMinuteTime = currentMinute
		channel.CurrentMinuteUsed = 0
	}

	// 增加使用计数
	channel.CurrentMinuteUsed++
}

func (channel *Channel) Save() error {
	return DB.Save(channel).Error
}

func GetAllChannels(startIdx int, num int, selectAll bool, idSort bool) ([]*Channel, error) {
	var channels []*Channel
	var err error
	order := "priority desc"
	if idSort {
		order = "id desc"
	}
	if selectAll {
		err = DB.Order(order).Find(&channels).Error
	} else {
		err = DB.Order(order).Limit(num).Offset(startIdx).Omit("key").Find(&channels).Error
	}
	return channels, err
}

func GetChannelsByTag(tag string, idSort bool) ([]*Channel, error) {
	var channels []*Channel
	order := "priority desc"
	if idSort {
		order = "id desc"
	}
	err := DB.Where("tag = ?", tag).Order(order).Find(&channels).Error
	return channels, err
}

func SearchChannels(keyword string, group string, model string, idSort bool) ([]*Channel, error) {
	var channels []*Channel
	modelsCol := "`models`"

	// 如果是 PostgreSQL，使用双引号
	if common.UsingPostgreSQL {
		modelsCol = `"models"`
	}

	baseURLCol := "`base_url`"
	// 如果是 PostgreSQL，使用双引号
	if common.UsingPostgreSQL {
		baseURLCol = `"base_url"`
	}

	order := "priority desc"
	if idSort {
		order = "id desc"
	}

	// 构造基础查询
	baseQuery := DB.Model(&Channel{}).Omit(keyCol)

	// 构造WHERE子句
	var whereClause string
	var args []interface{}
	if group != "" && group != "null" {
		var groupCondition string
		if common.UsingMySQL {
			groupCondition = `CONCAT(',', ` + groupCol + `, ',') LIKE ?`
		} else {
			// sqlite, PostgreSQL
			groupCondition = `(',' || ` + groupCol + ` || ',') LIKE ?`
		}
		whereClause = "(id = ? OR name LIKE ? OR " + keyCol + " = ? OR " + baseURLCol + " LIKE ?) AND " + modelsCol + ` LIKE ? AND ` + groupCondition
		args = append(args, common.String2Int(keyword), "%"+keyword+"%", keyword, "%"+keyword+"%", "%"+model+"%", "%,"+group+",%")
	} else {
		whereClause = "(id = ? OR name LIKE ? OR " + keyCol + " = ? OR " + baseURLCol + " LIKE ?) AND " + modelsCol + " LIKE ?"
		args = append(args, common.String2Int(keyword), "%"+keyword+"%", keyword, "%"+keyword+"%", "%"+model+"%")
	}

	// 执行查询
	err := baseQuery.Where(whereClause, args...).Order(order).Find(&channels).Error
	if err != nil {
		return nil, err
	}
	return channels, nil
}

func GetChannelById(id int, selectAll bool) (*Channel, error) {
	channel := Channel{Id: id}
	var err error = nil
	if selectAll {
		err = DB.First(&channel, "id = ?", id).Error
	} else {
		err = DB.Omit("key").First(&channel, "id = ?", id).Error
	}
	return &channel, err
}

func BatchInsertChannels(channels []Channel) error {
	var err error
	err = DB.Create(&channels).Error
	if err != nil {
		return err
	}
	for _, channel_ := range channels {
		err = channel_.AddAbilities()
		if err != nil {
			return err
		}
	}
	return nil
}

func BatchDeleteChannels(ids []int) error {
	//使用事务 删除channel表和channel_ability表
	tx := DB.Begin()
	err := tx.Where("id in (?)", ids).Delete(&Channel{}).Error
	if err != nil {
		// 回滚事务
		tx.Rollback()
		return err
	}
	err = tx.Where("channel_id in (?)", ids).Delete(&Ability{}).Error
	if err != nil {
		// 回滚事务
		tx.Rollback()
		return err
	}
	// 提交事务
	tx.Commit()
	return err
}

func (channel *Channel) GetPriority() int64 {
	if channel.Priority == nil {
		return 0
	}
	return *channel.Priority
}

func (channel *Channel) GetWeight() int {
	if channel.Weight == nil {
		return 0
	}
	return int(*channel.Weight)
}

func (channel *Channel) GetBaseURL() string {
	if channel.BaseURL == nil {
		return ""
	}
	return *channel.BaseURL
}

func (channel *Channel) GetModelMapping() string {
	if channel.ModelMapping == nil {
		return ""
	}
	return *channel.ModelMapping
}

func (channel *Channel) GetStatusCodeMapping() string {
	if channel.StatusCodeMapping == nil {
		return ""
	}
	return *channel.StatusCodeMapping
}

func (channel *Channel) Insert() error {
	var err error
	err = DB.Create(channel).Error
	if err != nil {
		return err
	}
	err = channel.AddAbilities()
	return err
}

func (channel *Channel) Update() error {
	var err error
	err = DB.Model(channel).Updates(channel).Error
	if err != nil {
		return err
	}
	DB.Model(channel).First(channel, "id = ?", channel.Id)
	err = channel.UpdateAbilities(nil)
	return err
}

func (channel *Channel) UpdateResponseTime(responseTime int64) {
	err := DB.Model(channel).Select("response_time", "test_time").Updates(Channel{
		TestTime:     common.GetTimestamp(),
		ResponseTime: int(responseTime),
	}).Error
	if err != nil {
		common.SysError("failed to update response time: " + err.Error())
	}
}

func (channel *Channel) UpdateBalance(balance float64) {
	err := DB.Model(channel).Select("balance_updated_time", "balance").Updates(Channel{
		BalanceUpdatedTime: common.GetTimestamp(),
		Balance:            balance,
	}).Error
	if err != nil {
		common.SysError("failed to update balance: " + err.Error())
	}
}

func (channel *Channel) Delete() error {
	var err error
	err = DB.Delete(channel).Error
	if err != nil {
		return err
	}
	err = channel.DeleteAbilities()
	return err
}

var channelStatusLock sync.Mutex

func UpdateChannelStatusById(id int, status int, reason string) bool {
	if common.MemoryCacheEnabled {
		channelStatusLock.Lock()
		defer channelStatusLock.Unlock()

		channelCache, _ := CacheGetChannel(id)
		// 如果缓存渠道存在，且状态已是目标状态，直接返回
		if channelCache != nil && channelCache.Status == status {
			return false
		}
		// 如果缓存渠道不存在(说明已经被禁用)，且要设置的状态不为启用，直接返回
		if channelCache == nil && status != common.ChannelStatusEnabled {
			return false
		}
		CacheUpdateChannelStatus(id, status)
	}
	err := UpdateAbilityStatus(id, status == common.ChannelStatusEnabled)
	if err != nil {
		common.SysError("failed to update ability status: " + err.Error())
		return false
	}
	channel, err := GetChannelById(id, true)
	if err != nil {
		// find channel by id error, directly update status
		updateData := map[string]interface{}{"status": status}

		// 如果是自动禁用，记录禁用时间（用于自动重置）
		if status == common.ChannelStatusAutoDisabled {
			updateData["last_reset_time"] = time.Now().Unix()
		}

		result := DB.Model(&Channel{}).Where("id = ?", id).Updates(updateData)
		if result.Error != nil {
			common.SysError("failed to update channel status: " + result.Error.Error())
			return false
		}
		if result.RowsAffected == 0 {
			return false
		}
	} else {
		if channel.Status == status {
			return false
		}
		// find channel by id success, update status and other info
		info := channel.GetOtherInfo()
		info["status_reason"] = reason
		info["status_time"] = common.GetTimestamp()
		channel.SetOtherInfo(info)
		channel.Status = status

		// 如果是自动禁用，且启用了自动重置，记录禁用时间
		if status == common.ChannelStatusAutoDisabled && channel.GetAutoResetEnabled() {
			channel.LastResetTime = time.Now().Unix()
		}

		err = channel.Save()
		if err != nil {
			common.SysError("failed to update channel status: " + err.Error())
			return false
		}
	}
	return true
}

func EnableChannelByTag(tag string) error {
	err := DB.Model(&Channel{}).Where("tag = ?", tag).Update("status", common.ChannelStatusEnabled).Error
	if err != nil {
		return err
	}
	err = UpdateAbilityStatusByTag(tag, true)
	return err
}

func DisableChannelByTag(tag string) error {
	err := DB.Model(&Channel{}).Where("tag = ?", tag).Update("status", common.ChannelStatusManuallyDisabled).Error
	if err != nil {
		return err
	}
	err = UpdateAbilityStatusByTag(tag, false)
	return err
}

func EditChannelByTag(tag string, newTag *string, modelMapping *string, models *string, group *string, priority *int64, weight *uint) error {
	updateData := Channel{}
	shouldReCreateAbilities := false
	updatedTag := tag
	// 如果 newTag 不为空且不等于 tag，则更新 tag
	if newTag != nil && *newTag != tag {
		updateData.Tag = newTag
		updatedTag = *newTag
	}
	if modelMapping != nil && *modelMapping != "" {
		updateData.ModelMapping = modelMapping
	}
	if models != nil && *models != "" {
		shouldReCreateAbilities = true
		updateData.Models = *models
	}
	if group != nil && *group != "" {
		shouldReCreateAbilities = true
		updateData.Group = *group
	}
	if priority != nil {
		updateData.Priority = priority
	}
	if weight != nil {
		updateData.Weight = weight
	}

	err := DB.Model(&Channel{}).Where("tag = ?", tag).Updates(updateData).Error
	if err != nil {
		return err
	}
	if shouldReCreateAbilities {
		channels, err := GetChannelsByTag(updatedTag, false)
		if err == nil {
			for _, channel := range channels {
				err = channel.UpdateAbilities(nil)
				if err != nil {
					common.SysError("failed to update abilities: " + err.Error())
				}
			}
		}
	} else {
		err := UpdateAbilityByTag(tag, newTag, priority, weight)
		if err != nil {
			return err
		}
	}
	return nil
}

func UpdateChannelUsedQuota(id int, quota int) {
	if common.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeChannelUsedQuota, id, quota)
		return
	}
	updateChannelUsedQuota(id, quota)
}

func UpdateChannelUsedCount(id int, count int) {
	if common.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeChannelUsedCount, id, count)
		return
	}
	updateChannelUsedCount(id, count)
}

func updateChannelUsedQuota(id int, quota int) {
	err := DB.Model(&Channel{}).Where("id = ?", id).Update("used_quota", gorm.Expr("used_quota + ?", quota)).Error
	if err != nil {
		common.SysError("failed to update channel used quota: " + err.Error())
	}
}

func updateChannelUsedCount(id int, count int) {
	err := DB.Model(&Channel{}).Where("id = ?", id).Update("used_count", gorm.Expr("used_count + ?", count)).Error
	if err != nil {
		common.SysError("failed to update channel used count: " + err.Error())
	}
}

func DeleteChannelByStatus(status int64) (int64, error) {
	result := DB.Where("status = ?", status).Delete(&Channel{})
	return result.RowsAffected, result.Error
}

func DeleteDisabledChannel() (int64, error) {
	result := DB.Where("status = ? or status = ?", common.ChannelStatusAutoDisabled, common.ChannelStatusManuallyDisabled).Delete(&Channel{})
	return result.RowsAffected, result.Error
}

func GetPaginatedTags(offset int, limit int) ([]*string, error) {
	var tags []*string
	err := DB.Model(&Channel{}).Select("DISTINCT tag").Where("tag != ''").Offset(offset).Limit(limit).Find(&tags).Error
	return tags, err
}

func SearchTags(keyword string, group string, model string, idSort bool) ([]*string, error) {
	var tags []*string
	modelsCol := "`models`"

	// 如果是 PostgreSQL，使用双引号
	if common.UsingPostgreSQL {
		modelsCol = `"models"`
	}

	baseURLCol := "`base_url`"
	// 如果是 PostgreSQL，使用双引号
	if common.UsingPostgreSQL {
		baseURLCol = `"base_url"`
	}

	order := "priority desc"
	if idSort {
		order = "id desc"
	}

	// 构造基础查询
	baseQuery := DB.Model(&Channel{}).Omit(keyCol)

	// 构造WHERE子句
	var whereClause string
	var args []interface{}
	if group != "" && group != "null" {
		var groupCondition string
		if common.UsingMySQL {
			groupCondition = `CONCAT(',', ` + groupCol + `, ',') LIKE ?`
		} else {
			// sqlite, PostgreSQL
			groupCondition = `(',' || ` + groupCol + ` || ',') LIKE ?`
		}
		whereClause = "(id = ? OR name LIKE ? OR " + keyCol + " = ? OR " + baseURLCol + " LIKE ?) AND " + modelsCol + ` LIKE ? AND ` + groupCondition
		args = append(args, common.String2Int(keyword), "%"+keyword+"%", keyword, "%"+keyword+"%", "%"+model+"%", "%,"+group+",%")
	} else {
		whereClause = "(id = ? OR name LIKE ? OR " + keyCol + " = ? OR " + baseURLCol + " LIKE ?) AND " + modelsCol + " LIKE ?"
		args = append(args, common.String2Int(keyword), "%"+keyword+"%", keyword, "%"+keyword+"%", "%"+model+"%")
	}

	subQuery := baseQuery.Where(whereClause, args...).
		Select("tag").
		Where("tag != ''").
		Order(order)

	err := DB.Table("(?) as sub", subQuery).
		Select("DISTINCT tag").
		Find(&tags).Error

	if err != nil {
		return nil, err
	}

	return tags, nil
}

func (channel *Channel) GetSetting() map[string]interface{} {
	setting := make(map[string]interface{})
	if channel.Setting != nil && *channel.Setting != "" {
		err := json.Unmarshal([]byte(*channel.Setting), &setting)
		if err != nil {
			common.SysError("failed to unmarshal setting: " + err.Error())
		}
	}
	return setting
}

func (channel *Channel) SetSetting(setting map[string]interface{}) {
	settingBytes, err := json.Marshal(setting)
	if err != nil {
		common.SysError("failed to marshal setting: " + err.Error())
		return
	}
	channel.Setting = common.GetPointer[string](string(settingBytes))
}

func (channel *Channel) GetParamOverride() map[string]interface{} {
	paramOverride := make(map[string]interface{})
	if channel.ParamOverride != nil && *channel.ParamOverride != "" {
		err := json.Unmarshal([]byte(*channel.ParamOverride), &paramOverride)
		if err != nil {
			common.SysError("failed to unmarshal param override: " + err.Error())
		}
	}
	return paramOverride
}

func GetChannelsByIds(ids []int) ([]*Channel, error) {
	var channels []*Channel
	err := DB.Where("id in (?)", ids).Find(&channels).Error
	return channels, err
}

func BatchSetChannelTag(ids []int, tag *string) error {
	// 开启事务
	tx := DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// 更新标签
	err := tx.Model(&Channel{}).Where("id in (?)", ids).Update("tag", tag).Error
	if err != nil {
		tx.Rollback()
		return err
	}

	// update ability status
	channels, err := GetChannelsByIds(ids)
	if err != nil {
		tx.Rollback()
		return err
	}

	for _, channel := range channels {
		err = channel.UpdateAbilities(tx)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	// 提交事务
	return tx.Commit().Error
}

// GetChannelsWithQuotaLimitEnabled 获取所有启用限额的渠道
func GetChannelsWithQuotaLimitEnabled() ([]*Channel, error) {
	var channels []*Channel
	err := DB.Where("quota_limit_enabled = ? AND status = ?", true, common.ChannelStatusEnabled).Find(&channels).Error
	return channels, err
}

// GetChannelsWithCountLimitEnabled 获取启用次数限制的渠道
func GetChannelsWithCountLimitEnabled() ([]*Channel, error) {
	var channels []*Channel
	err := DB.Where("count_limit_enabled = ? AND status = ?", true, common.ChannelStatusEnabled).Find(&channels).Error
	return channels, err
}

// GetChannelsWithAnyLimitEnabled 获取启用任何限制的渠道
func GetChannelsWithAnyLimitEnabled() ([]*Channel, error) {
	var channels []*Channel
	err := DB.Where("(quota_limit_enabled = ? OR count_limit_enabled = ?) AND status = ?", true, true, common.ChannelStatusEnabled).Find(&channels).Error
	return channels, err
}

// CheckAndResetChannels 检查并重置需要自动重置的渠道
func CheckAndResetChannels() (int, error) {
	// 获取所有启用了自动重置且被自动禁用的渠道
	var channels []*Channel
	err := DB.Where("auto_reset_enabled = ? AND status = ? AND last_reset_time > 0", true, common.ChannelStatusAutoDisabled).Find(&channels).Error
	if err != nil {
		return 0, err
	}

	resetCount := 0
	currentTime := time.Now().Unix()

	for _, channel := range channels {
		// 检查是否到了重置时间（从禁用时间开始计算）
		if channel.GetAutoResetInterval() > 0 && currentTime-channel.LastResetTime >= channel.GetAutoResetInterval() {
			// 计算禁用时长
			disabledDuration := currentTime - channel.LastResetTime

			// 重置次数、清除禁用原因并重新启用渠道
			channel.UsedCount = 0
			channel.LastResetTime = 0 // 重置后清空禁用时间
			channel.Status = common.ChannelStatusEnabled

			// 清除状态原因和时间信息
			info := channel.GetOtherInfo()
			delete(info, "status_reason")
			delete(info, "status_time")
			info["auto_reset_time"] = common.GetTimestamp()
			channel.SetOtherInfo(info)

			err = DB.Save(channel).Error
			if err != nil {
				common.SysError(fmt.Sprintf("failed to reset channel %d: %s", channel.Id, err.Error()))
				continue
			}

			// 更新缓存中的渠道状态
			if common.MemoryCacheEnabled {
				CacheUpdateChannelStatus(channel.Id, common.ChannelStatusEnabled)
			}

			// 更新能力状态
			err = UpdateAbilityStatus(channel.Id, true)
			if err != nil {
				common.SysError(fmt.Sprintf("failed to update ability status for channel %d: %s", channel.Id, err.Error()))
			}

			resetCount++
			common.SysLog(fmt.Sprintf("渠道 %s (ID: %d) 自动重置并重新启用，禁用时长: %d秒", channel.Name, channel.Id, disabledDuration))
		}
	}

	return resetCount, nil
}

// CheckAndDisableOverQuotaChannels 检查并禁用超过限额的渠道
func CheckAndDisableOverQuotaChannels() (int, error) {
	channels, err := GetChannelsWithAnyLimitEnabled()
	if err != nil {
		return 0, err
	}

	disabledCount := 0
	for _, channel := range channels {
		if channel.CheckQuotaLimit() {
			// 禁用渠道
			var reason string
			if channel.GetQuotaLimitEnabled() && channel.UsedQuota >= channel.GetQuotaLimit() {
				reason = fmt.Sprintf("渠道额度已达到限制，限额: %d，已用: %d", channel.GetQuotaLimit(), channel.UsedQuota)
			} else if channel.GetCountLimitEnabled() && channel.UsedCount >= channel.GetCountLimit() {
				reason = fmt.Sprintf("渠道次数已达到限制，限制: %d，已用: %d", channel.GetCountLimit(), channel.UsedCount)
			}

			success := UpdateChannelStatusById(channel.Id, common.ChannelStatusAutoDisabled, reason)
			if !success {
				common.SysError("failed to disable channel due to limit")
				continue
			}
			disabledCount++
			common.SysLog(fmt.Sprintf("渠道 %s (ID: %d) 被自动禁用：%s", channel.Name, channel.Id, reason))
		}
	}

	return disabledCount, nil
}

// ChannelQuotaCheckTask 渠道限额检查定时任务
func ChannelQuotaCheckTask() {
	var lastInterval int // 记录上次的检查间隔
	lastInterval = common.ChannelQuotaCheckInterval

	for {
		// 检查间隔是否发生变化
		currentInterval := common.ChannelQuotaCheckInterval
		if currentInterval != lastInterval {
			common.SysLog(fmt.Sprintf("渠道限额检查间隔已更新：从 %d 秒变更为 %d 秒", lastInterval, currentInterval))
			lastInterval = currentInterval
		}

		// 等待指定的检查间隔
		time.Sleep(time.Duration(currentInterval) * time.Second)

		common.SysLog("开始检查渠道限额和自动重置...")

		// 检查并禁用超过限额的渠道
		disabledCount, err := CheckAndDisableOverQuotaChannels()
		if err != nil {
			common.SysError("渠道限额检查失败: " + err.Error())
		} else {
			if disabledCount > 0 {
				common.SysLog(fmt.Sprintf("渠道限额检查完成，共禁用 %d 个渠道", disabledCount))
			}
		}

		// 检查并重置需要自动重置的渠道
		resetCount, err := CheckAndResetChannels()
		if err != nil {
			common.SysError("渠道自动重置检查失败: " + err.Error())
		} else {
			if resetCount > 0 {
				common.SysLog(fmt.Sprintf("渠道自动重置检查完成，共重置 %d 个渠道", resetCount))
			}
		}

		if disabledCount == 0 && resetCount == 0 {
			common.SysLog("渠道检查完成，无渠道需要禁用或重置")
		}
	}
}

func (channel *Channel) GetRPMLimitEnabled() bool {
	if channel.RPMLimitEnabled == nil {
		return false
	}
	return *channel.RPMLimitEnabled
}

func (channel *Channel) GetRPMLimit() int64 {
	if channel.RPMLimit == nil {
		return 0
	}
	return *channel.RPMLimit
}

func (channel *Channel) SetRPMLimitEnabled(enabled bool) {
	channel.RPMLimitEnabled = &enabled
}

func (channel *Channel) SetRPMLimit(limit int64) {
	channel.RPMLimit = &limit
}
