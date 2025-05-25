# 对话历史功能实现总结

## 功能概述

本次实现为 one-api 项目添加了完整的对话历史存储和管理功能，允许系统自动保存用户与AI的对话记录，并提供完整的CRUD操作API。

## 实现的文件

### 1. 数据模型层 (`model/conversation_history.go`)
- 定义了 `ConversationHistory` 结构体
- 实现了完整的CRUD操作函数
- 支持分页查询、搜索、软删除等功能
- 提供JSON数据的序列化/反序列化方法

### 2. 控制器层 (`controller/conversation_history.go`)
- 实现了用户和管理员的API接口
- 包含权限控制和参数验证
- 支持分页、搜索、删除等操作
- 提供管理员专用的高级功能

### 3. 路由配置 (`router/api-router.go`)
- 添加了对话历史相关的API路由
- 区分用户路由和管理员路由
- 应用了相应的中间件进行权限控制

### 4. 业务逻辑集成 (`relay/relay-text.go`)
- 在聊天完成流程中集成了对话历史保存
- 支持从请求头获取自定义对话ID
- 异步保存，不影响主流程性能
- 仅在聊天完成模式下触发保存

### 5. 数据库迁移 (`model/main.go`)
- 添加了 `ConversationHistory` 表的自动迁移
- 系统启动时自动创建表结构

## 数据库表结构

```sql
CREATE TABLE conversation_histories (
    id INT AUTO_INCREMENT PRIMARY KEY,
    conversation_id VARCHAR(255) NOT NULL,
    model_name VARCHAR(255) NOT NULL,
    raw_json LONGTEXT NOT NULL,
    user_id INT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL,
    INDEX idx_conversation_id (conversation_id),
    INDEX idx_model_name (model_name),
    INDEX idx_user_id (user_id),
    INDEX idx_deleted_at (deleted_at)
);
```

## API 接口列表

### 用户接口
- `GET /api/conversation/history` - 获取对话历史列表
- `GET /api/conversation/history/:id` - 获取单个对话历史
- `DELETE /api/conversation/history/:id` - 删除对话历史
- `DELETE /api/conversation/history/conversation/:conversation_id` - 根据对话ID删除

### 管理员接口
- `GET /api/conversation/admin/history` - 获取所有对话历史
- `DELETE /api/conversation/admin/history/:id` - 管理员删除对话历史
- `POST /api/conversation/admin/cleanup` - 清理旧的对话历史

## 核心功能特性

### 1. 自动保存机制
- 在聊天完成请求成功处理后自动保存
- 异步处理，不影响响应性能
- 支持自定义对话ID（通过 `X-Conversation-ID` 请求头）
- 自动生成对话ID（格式：`conv_{user_id}_{timestamp}`）

### 2. 权限控制
- 用户只能查看和删除自己的对话历史
- 管理员可以查看所有用户的对话历史
- 支持软删除和硬删除（仅管理员）

### 3. 查询功能
- 支持分页查询
- 支持关键词搜索
- 支持按用户ID、对话ID、模型名称筛选
- 支持时间范围查询

### 4. 数据格式
- 使用JSON格式存储完整的对话数据
- 包含用户消息和AI回复
- 保留模型信息和元数据

## 使用示例

### 1. 前端发送聊天请求时指定对话ID
```javascript
fetch('/v1/chat/completions', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${apiKey}`,
    'X-Conversation-ID': 'my-conversation-123'
  },
  body: JSON.stringify({
    model: "claude-opus-4-20250514",
    messages: [
      {
        role: "user",
        content: "你好"
      }
    ]
  })
});
```

### 2. 获取用户对话历史
```javascript
const response = await fetch('/api/conversation/history?page=1&page_size=20', {
  headers: {
    'Authorization': `Bearer ${userToken}`
  }
});
const data = await response.json();
```

### 3. 搜索对话历史
```javascript
const response = await fetch('/api/conversation/history?keyword=CUDA', {
  headers: {
    'Authorization': `Bearer ${userToken}`
  }
});
```

## 性能考虑

1. **异步保存**: 对话历史保存使用协程异步处理，不会阻塞主请求流程
2. **索引优化**: 在关键字段上建立了数据库索引，提高查询性能
3. **分页查询**: 所有列表接口都支持分页，避免大量数据的性能问题
4. **软删除**: 使用软删除机制，避免频繁的物理删除操作

## 安全考虑

1. **权限控制**: 严格的用户权限控制，防止越权访问
2. **数据隐私**: 对话历史包含敏感信息，需要注意数据保护
3. **输入验证**: 对所有用户输入进行验证和过滤
4. **SQL注入防护**: 使用GORM的参数化查询防止SQL注入

## 扩展性

1. **模块化设计**: 各层职责清晰，易于维护和扩展
2. **配置化**: 可以通过配置控制是否启用对话历史功能
3. **清理机制**: 提供了自动清理旧数据的功能，便于数据管理
4. **API兼容性**: 新增功能不影响现有API的兼容性

## 测试

提供了 `test/conversation_history_test.go` 测试文件，可以验证以下功能：
- 创建对话历史
- 查询对话历史
- 搜索功能
- JSON数据解析
- 删除功能
- 清理功能

运行测试命令：
```bash
go test ./test -v
```

## 部署说明

1. 系统启动时会自动创建 `conversation_histories` 表
2. 无需额外的配置，功能会自动启用
3. 建议定期运行清理任务，删除过期的对话历史
4. 监控存储空间使用情况，对话历史可能占用较大存储空间

## 注意事项

1. **存储空间**: 对话历史会占用较大的存储空间，需要定期清理
2. **隐私保护**: 对话内容包含用户隐私信息，需要遵守相关法规
3. **性能监控**: 在高并发场景下需要监控数据库性能
4. **备份策略**: 重要的对话历史数据需要制定备份策略 