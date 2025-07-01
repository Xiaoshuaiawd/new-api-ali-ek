# 日期分区表自动监控功能

## 功能概述

本系统实现了对话历史记录的日期分区表自动监控和创建功能，确保：

1. **自动检测日期变化**：每分钟检查一次，当检测到日期发生变化时自动创建新的分区表
2. **无缝数据切换**：新的对话历史记录自动写入到当天的分区表
3. **提前准备**：自动预创建明天的分区表，避免跨日时的延迟
4. **故障恢复**：即使监控服务失败，写入时也会检查并创建必要的表

## 工作机制

### 启动时

1. 读取环境变量 `MES_DAILY_PARTITION`（默认：`true`）
2. 创建当天的分区表（如果不存在）
3. 启动后台监控服务

### 运行时

1. **监控服务**：每分钟检查一次当前日期
2. **日期变化检测**：当检测到日期从今天变为明天时：
   - 创建新一天的分区表
   - 预创建下一天的分区表
   - 更新内部日期跟踪
3. **写入保障**：每次写入数据时都会确保目标表存在

### 表命名规则

- 对话历史表：`conversation_histories_YYYY_MM_DD`
- 错误历史表：`error_conversation_histories_YYYY_MM_DD`

示例：
- `conversation_histories_2025_01_15`
- `error_conversation_histories_2025_01_15`

## 配置选项

### 环境变量

```bash
# 启用日期分区（默认：true）
MES_DAILY_PARTITION=true

# 启用对话历史功能（需要同时启用）
CONVERSATION_HISTORY_ENABLED=true

# MES数据库连接（可选，不设置则使用主数据库）
MES_SQL_DSN="user:password@tcp(localhost:3306)/messages_db"
```

### 数据库配置

也可以通过管理后台设置：
- 对话历史启用：`ConversationHistoryEnabled`
- 日期分区启用：`MESDailyPartition`

## 监控和状态

### 日志信息

启动时：
```
Final ConversationHistoryEnabled setting: true
Final MESDailyPartition setting: true
Ensuring today's partition tables exist: conversation_histories_2025_01_15, error_conversation_histories_2025_01_15
Today's partition tables created successfully
Partition table monitor started
```

日期变化时：
```
Date changed from 2025_01_15 to 2025_01_16, creating new partition tables
Successfully created partition tables for 2025_01_16
Successfully pre-created partition tables for 2025_01_17
```

### 状态查询

可以通过程序内部调用 `model.GetPartitionTableStats()` 获取状态：

```go
stats := model.GetPartitionTableStats()
// 返回：
// {
//   "partition_enabled": true,
//   "current_date": "2025_01_15",
//   "monitor_running": true,
//   "existing_tables": ["conversation_histories_2025_01_14", "conversation_histories_2025_01_15"],
//   "total_partitions": 2
// }
```

## 开发和测试

### 模拟日期变化

```go
// 模拟日期变化到明天
tomorrow := time.Now().AddDate(0, 0, 1)
err := model.SimulateDateChange(tomorrow)
if err != nil {
    log.Printf("Failed to simulate date change: %v", err)
}
```

### 手动创建明天的表

```go
err := model.PreCreateTomorrowTables()
if err != nil {
    log.Printf("Failed to pre-create tomorrow's tables: %v", err)
}
```

## 性能考虑

1. **监控频率**：每分钟检查一次，对系统性能影响微乎其微
2. **表创建**：只在必要时创建表，避免重复操作
3. **并发安全**：使用读写锁保护日期状态，确保线程安全
4. **故障恢复**：即使监控服务异常，写入操作仍然会检查并创建必要的表

## 注意事项

1. **时区**：使用服务器本地时间进行日期判断
2. **权限**：确保数据库用户有创建表的权限
3. **存储**：每天的数据存储在独立的表中，便于维护和备份
4. **查询**：系统会自动跨所有相关表进行查询，对应用层透明

## 故障排除

### 监控服务未启动
```
# 检查日志中是否有此信息
Partition table monitor started
```

### 表创建失败
```
# 检查数据库权限
GRANT CREATE ON database_name.* TO 'username'@'host';

# 检查日志中的错误信息
Failed to create partition tables for 2025_01_16: ...
```

### 日期监控不工作
```
# 手动触发测试
model.SimulateDateChange(time.Now().AddDate(0, 0, 1))
``` 