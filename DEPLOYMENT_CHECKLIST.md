# 对话历史功能部署检查清单

## 📋 部署前检查

### 1. 文件完整性检查
- [x] `model/conversation_history.go` - 数据模型定义
- [x] `controller/conversation_history.go` - API控制器
- [x] `router/api-router.go` - 路由配置已更新
- [x] `relay/relay-text.go` - 业务逻辑集成已更新
- [x] `model/main.go` - 数据库迁移已更新
- [x] `docs/conversation_history_api.md` - API文档
- [x] `test/conversation_history_test.go` - 测试文件

### 2. 代码修复检查
- [x] 修复了 `common.IsAdmin` 为 `model.IsAdmin`
- [x] 删除了冲突的测试文件
- [x] 所有导入包都在 go.mod 中存在

### 3. 数据库检查
- [x] `ConversationHistory` 结构体定义正确
- [x] 数据库迁移已添加到 `migrateDB()` 函数
- [x] 表名设置为 `conversation_histories`
- [x] 索引配置正确

### 4. API路由检查
- [x] 用户路由已添加
- [x] 管理员路由已添加
- [x] 中间件权限控制正确

## 🚀 部署步骤

### 1. 编译检查
```bash
# 检查编译是否成功
go build -o one-api
```

### 2. 数据库迁移
系统启动时会自动执行数据库迁移，创建 `conversation_histories` 表。

### 3. 功能验证
启动系统后，可以通过以下方式验证功能：

#### API测试
```bash
# 获取对话历史列表（需要用户token）
curl -H "Authorization: Bearer YOUR_TOKEN" \
     "http://localhost:3000/api/conversation/history"

# 管理员获取所有对话历史（需要管理员token）
curl -H "Authorization: Bearer ADMIN_TOKEN" \
     "http://localhost:3000/api/conversation/admin/history"
```

#### 聊天测试
```bash
# 发送聊天请求，系统会自动保存对话历史
curl -X POST "http://localhost:3000/v1/chat/completions" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer YOUR_API_KEY" \
     -H "X-Conversation-ID: test-conversation-123" \
     -d '{
       "model": "gpt-3.5-turbo",
       "messages": [
         {
           "role": "user",
           "content": "Hello, how are you?"
         }
       ]
     }'
```

### 4. 日志检查
启动后检查日志中是否有以下信息：
- `database migration started`
- `database migrated`
- 没有关于 `conversation_histories` 表的错误信息

## 🔧 配置选项

### 环境变量
无需额外的环境变量配置，功能会自动启用。

### 可选配置
如果需要禁用对话历史功能，可以在代码中添加配置开关：

```go
// 在 relay/relay-text.go 中的 saveConversationHistory 函数开头添加
if !setting.ConversationHistoryEnabled {
    return
}
```

## 📊 监控指标

### 数据库监控
- 监控 `conversation_histories` 表的大小增长
- 监控查询性能
- 设置存储空间告警

### 性能监控
- 监控对话历史保存的异步处理性能
- 监控API响应时间
- 监控内存使用情况

## 🧹 维护任务

### 定期清理
建议设置定期任务清理旧的对话历史：

```bash
# 清理30天前的对话历史
curl -X POST "http://localhost:3000/api/conversation/admin/cleanup?days=30" \
     -H "Authorization: Bearer ADMIN_TOKEN"
```

### 备份策略
- 定期备份 `conversation_histories` 表
- 考虑数据归档策略
- 制定数据恢复计划

## ⚠️ 注意事项

### 隐私保护
- 对话历史包含用户敏感信息
- 确保符合数据保护法规（GDPR、CCPA等）
- 实施适当的数据加密

### 性能考虑
- 在高并发环境下监控数据库性能
- 考虑分表策略（按时间或用户分表）
- 监控存储空间使用

### 安全考虑
- 确保API权限控制正确
- 定期审计数据访问日志
- 实施数据脱敏策略

## 🐛 故障排除

### 常见问题

1. **编译错误**
   - 检查所有导入包是否正确
   - 确认 go.mod 中包含所需依赖

2. **数据库迁移失败**
   - 检查数据库连接
   - 确认数据库用户权限
   - 查看详细错误日志

3. **API返回404**
   - 确认路由配置正确
   - 检查中间件配置
   - 验证控制器函数名称

4. **权限错误**
   - 确认使用 `model.IsAdmin` 而不是 `common.IsAdmin`
   - 检查用户token有效性
   - 验证用户角色权限

### 日志调试
启用调试模式查看详细日志：
```bash
export DEBUG=true
./one-api
```

## ✅ 部署完成确认

- [ ] 系统启动无错误
- [ ] 数据库表创建成功
- [ ] API接口响应正常
- [ ] 聊天请求自动保存对话历史
- [ ] 用户可以查看自己的对话历史
- [ ] 管理员可以管理所有对话历史
- [ ] 权限控制正常工作
- [ ] 性能监控已设置
- [ ] 清理任务已配置

部署完成后，对话历史功能即可正常使用！ 