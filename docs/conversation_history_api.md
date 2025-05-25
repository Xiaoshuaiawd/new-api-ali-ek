# 对话历史 API 文档

## 概述

对话历史功能允许系统自动保存用户与AI的对话记录，并提供查询、管理等功能。

## 数据库表结构

表名：`conversation_histories`

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | int | 主键，自增 |
| conversation_id | varchar(255) | 对话ID，用于标识同一个对话会话 |
| model_name | varchar(255) | 使用的模型名称 |
| raw_json | longtext | 原始JSON数据，包含用户消息和AI回复 |
| user_id | int | 用户ID |
| created_at | timestamp | 创建时间 |
| updated_at | timestamp | 更新时间 |
| deleted_at | timestamp | 软删除时间 |

## raw_json 数据格式

```json
{
  "messages": [
    {
      "content": [
        {
          "text": "如何编写高性能的 CUDA Kernel",
          "type": "text"
        }
      ],
      "role": "user"
    },
    {
      "content": "编写高性能的 CUDA Kernel 是一个复杂的主题...",
      "role": "assistant"
    }
  ],
  "model": "claude-opus-4-20250514"
}
```

## API 接口

### 用户接口

#### 1. 获取对话历史列表

**GET** `/api/conversation/history`

**参数：**
- `conversation_id` (可选): 根据对话ID筛选
- `model_name` (可选): 根据模型名称筛选
- `keyword` (可选): 关键词搜索
- `page` (可选): 页码，默认1
- `page_size` (可选): 每页数量，默认20，最大100

**响应：**
```json
{
  "success": true,
  "message": "",
  "data": {
    "histories": [
      {
        "id": 1,
        "conversation_id": "conv_123_1704067200",
        "model_name": "claude-opus-4-20250514",
        "raw_json": "{\"messages\":[...],\"model\":\"claude-opus-4-20250514\"}",
        "user_id": 123,
        "created_at": "2024-01-01T00:00:00Z",
        "updated_at": "2024-01-01T00:00:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 20
  }
}
```

#### 2. 获取单个对话历史

**GET** `/api/conversation/history/:id`

**响应：**
```json
{
  "success": true,
  "message": "",
  "data": {
    "id": 1,
    "conversation_id": "conv_123_1704067200",
    "model_name": "claude-opus-4-20250514",
    "raw_json": "{\"messages\":[...],\"model\":\"claude-opus-4-20250514\"}",
    "user_id": 123,
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

#### 3. 删除对话历史

**DELETE** `/api/conversation/history/:id`

**响应：**
```json
{
  "success": true,
  "message": "删除成功"
}
```

#### 4. 根据对话ID删除对话历史

**DELETE** `/api/conversation/history/conversation/:conversation_id`

**响应：**
```json
{
  "success": true,
  "message": "删除成功"
}
```

### 管理员接口

#### 1. 获取所有对话历史

**GET** `/api/conversation/admin/history`

**参数：**
- `user_id` (可选): 根据用户ID筛选
- `conversation_id` (可选): 根据对话ID筛选
- `model_name` (可选): 根据模型名称筛选
- `keyword` (可选): 关键词搜索
- `page` (可选): 页码，默认1
- `page_size` (可选): 每页数量，默认20，最大100

#### 2. 管理员删除对话历史

**DELETE** `/api/conversation/admin/history/:id`

**参数：**
- `hard` (可选): 是否硬删除，值为 "true" 时进行硬删除

#### 3. 清理旧的对话历史

**POST** `/api/conversation/admin/cleanup`

**参数：**
- `days` (可选): 清理多少天前的记录，默认30天

**响应：**
```json
{
  "success": true,
  "message": "清理完成",
  "data": {
    "deleted_count": 100
  }
}
```

## 自动保存机制

系统会在以下情况下自动保存对话历史：

1. **触发条件**: 仅在聊天完成模式 (`RelayModeChatCompletions`) 下保存
2. **保存时机**: 在成功处理请求并完成计费后保存
3. **异步处理**: 使用协程异步保存，不影响主流程性能
4. **对话ID生成**: 如果请求中没有提供 `conversation_id`，系统会自动生成格式为 `conv_{user_id}_{timestamp}` 的ID

## 使用示例

### 前端集成示例

```javascript
// 获取用户的对话历史
async function getConversationHistories(page = 1, pageSize = 20) {
  const response = await fetch(`/api/conversation/history?page=${page}&page_size=${pageSize}`, {
    headers: {
      'Authorization': `Bearer ${userToken}`
    }
  });
  return await response.json();
}

// 搜索对话历史
async function searchConversationHistories(keyword) {
  const response = await fetch(`/api/conversation/history?keyword=${encodeURIComponent(keyword)}`, {
    headers: {
      'Authorization': `Bearer ${userToken}`
    }
  });
  return await response.json();
}

// 删除对话历史
async function deleteConversationHistory(historyId) {
  const response = await fetch(`/api/conversation/history/${historyId}`, {
    method: 'DELETE',
    headers: {
      'Authorization': `Bearer ${userToken}`
    }
  });
  return await response.json();
}
```

### 在聊天请求中指定对话ID

```javascript
// 在发送聊天请求时，可以在请求头中指定对话ID
const chatRequest = {
  model: "claude-opus-4-20250514",
  messages: [
    {
      role: "user",
      content: "你好"
    }
  ]
};

fetch('/v1/chat/completions', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${apiKey}`,
    'X-Conversation-ID': 'my-conversation-123'  // 自定义对话ID
  },
  body: JSON.stringify(chatRequest)
});
```

## 注意事项

1. **权限控制**: 用户只能查看和删除自己的对话历史，管理员可以查看所有用户的对话历史
2. **软删除**: 普通删除操作使用软删除，数据仍保留在数据库中，只有管理员可以进行硬删除
3. **性能考虑**: 对话历史保存是异步进行的，不会影响聊天请求的响应时间
4. **存储空间**: 建议定期清理旧的对话历史以节省存储空间
5. **隐私保护**: 对话历史包含用户的完整对话内容，需要注意数据安全和隐私保护

## 数据迁移

系统会在启动时自动创建 `conversation_histories` 表，无需手动创建。如果需要手动创建，可以使用以下SQL：

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