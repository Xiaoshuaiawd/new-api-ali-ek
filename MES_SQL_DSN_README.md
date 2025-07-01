# MES_SQL_DSN 环境变量说明

## 概述

`MES_SQL_DSN` 是一个新增的环境变量，专门用于配置聊天历史数据的存储数据库。通过这个配置，您可以将聊天历史数据存储到独立的数据库中，实现数据分离。

## 功能特性

- **数据分离**: 聊天历史数据可以存储在独立的数据库中，与主业务数据分离
- **向后兼容**: 如果不设置 `MES_SQL_DSN`，聊天历史数据将继续存储在主数据库中
- **多数据库支持**: 支持 MySQL、PostgreSQL、SQLite 三种数据库类型
- **灵活配置**: 可以与主数据库使用相同或不同的数据库类型
- **日期分表**: 支持按日期自动分表存储，便于数据归档和管理

## 环境变量配置

### 基本格式

```bash
# MySQL 格式
MES_SQL_DSN=username:password@tcp(host:port)/database_name

# PostgreSQL 格式  
MES_SQL_DSN=postgres://username:password@host:port/database_name

# SQLite 格式
MES_SQL_DSN=local
```

### 配置示例

#### 1. 使用独立的 MySQL 数据库
```bash
SQL_DSN=root:123456@tcp(localhost:3306)/oneapi
MES_SQL_DSN=root:123456@tcp(localhost:3306)/oneapi_messages
```

#### 1.1 启用日期分表功能
```bash
SQL_DSN=root:123456@tcp(localhost:3306)/oneapi
MES_SQL_DSN=root:123456@tcp(localhost:3306)/oneapi_messages
MES_DAILY_PARTITION=true  # 启用按日期分表
```

#### 2. 使用独立的 PostgreSQL 数据库
```bash
SQL_DSN=postgres://user:password@localhost:5432/oneapi
MES_SQL_DSN=postgres://user:password@localhost:5432/oneapi_messages
```

#### 3. 主数据库 MySQL，聊天历史使用 PostgreSQL
```bash
SQL_DSN=root:123456@tcp(localhost:3306)/oneapi
MES_SQL_DSN=postgres://user:password@localhost:5432/oneapi_messages
```

#### 4. 不设置 MES_SQL_DSN（向后兼容模式）
```bash
SQL_DSN=root:123456@tcp(localhost:3306)/oneapi
# MES_SQL_DSN 未设置，聊天历史存储在主数据库中
```

## Docker Compose 配置示例

```yaml
version: '3.4'

services:
  new-api:
    image: calciumion/new-api:latest
    container_name: new-api
    restart: always
    ports:
      - "3000:3000"
    environment:
      - SQL_DSN=root:123456@tcp(mysql:3306)/new-api
      - MES_SQL_DSN=root:123456@tcp(mysql:3306)/new-api-mes  # 聊天历史专用数据库
      - REDIS_CONN_STRING=redis://redis
      - TZ=Asia/Shanghai
    depends_on:
      - redis
      - mysql

  mysql:
    image: mysql:8.2
    container_name: mysql
    restart: always
    environment:
      MYSQL_ROOT_PASSWORD: 123456
      MYSQL_DATABASE: new-api
    volumes:
      - mysql_data:/var/lib/mysql
```

## 数据库自动创建

系统支持自动创建数据库（如果不存在）：

### MySQL 数据库
- 系统会自动检测目标数据库是否存在
- 如果数据库不存在，会自动创建，使用 `utf8mb4` 字符集
- 创建成功后会在日志中显示确认信息

### PostgreSQL 数据库  
- 由于PostgreSQL权限管理的复杂性，系统不会自动创建数据库
- 请在启动应用前手动创建目标数据库
- 系统会在日志中提示需要手动创建的数据库名称

### SQLite 数据库
- SQLite 文件会在首次访问时自动创建，无需特殊处理

## 数据库表结构

当使用 `MES_SQL_DSN` 时，系统会在指定的数据库中自动创建以下表：

1. **conversation_histories** - 存储正常的聊天历史记录
2. **error_conversation_histories** - 存储发生错误时的聊天历史记录

这些表会在系统启动时自动创建，无需手动建表。

## 日期分表功能

### 概述

通过设置 `MES_DAILY_PARTITION=true` 环境变量，可以启用按日期分表的功能。启用后，系统会根据当前日期自动创建表，表名格式如下：

- `conversation_histories_2025_01_15` - 2025年1月15日的聊天历史
- `error_conversation_histories_2025_01_15` - 2025年1月15日的错误聊天历史

### 配置方法

```bash
# 基本配置
MES_SQL_DSN=root:123456@tcp(localhost:3306)/oneapi_messages

# 启用日期分表
MES_DAILY_PARTITION=true
```

### 功能特点

1. **自动创建表**: 系统会在需要时自动创建当天的表，无需手动干预
2. **向后兼容**: 未启用分表时使用原表名，启用后新数据使用分表
3. **跨表查询**: 系统自动处理跨多个日期表的查询操作
4. **数据归档**: 便于按日期进行数据归档和清理

### 查询范围与数据保留

**查询范围：**
- 系统会自动发现并查询所有存在的分表，无时间限制
- 支持跨表分页、搜索和所有CRUD操作
- 查询性能会随着分表数量增加而降低

**数据保留策略：**
- **系统不会自动清理任何历史数据**
- 所有分表数据都会被永久保留，除非手动删除
- 如需清理旧数据，请手动删除对应的数据库表

**性能考虑：**
- 建议定期将非常老的数据归档到其他存储系统
- 可以通过手动删除旧的分表来释放存储空间

### 使用场景

1. **大数据量场景**: 当聊天历史数据量很大时，分表可以提高查询性能
2. **数据归档**: 便于按日期归档历史数据
3. **存储管理**: 可以针对不同日期的表制定不同的存储策略
4. **性能优化**: 减少单表数据量，提高查询效率

### 注意事项

1. **数据迁移**: 启用分表后，新数据会存储在日期表中，旧数据仍在原表中
2. **查询性能**: 跨表查询可能比单表查询稍慢，建议合理设置查询范围
3. **存储空间**: 每天会创建新表，注意监控存储空间使用情况

## 使用场景

### 1. 数据分离
- 将聊天历史数据与业务数据分离，便于数据管理和备份
- 聊天历史数据通常占用较大存储空间，分离后可以独立扩容

### 2. 性能优化
- 减少主数据库的负载，特别是在高并发聊天场景下
- 可以为聊天历史数据库配置不同的性能参数

### 3. 合规要求
- 某些场景下需要将聊天数据存储在特定的数据库或地域
- 便于实现数据保留策略和清理规则

### 4. 灾备策略
- 可以为聊天历史数据制定独立的备份和恢复策略
- 降低单点故障的风险

## 注意事项

1. **数据迁移**: 如果您已有聊天历史数据在主数据库中，设置 `MES_SQL_DSN` 后，新的聊天记录会存储在新数据库中，旧数据仍在主数据库中

2. **权限配置**: 确保 `MES_SQL_DSN` 指定的数据库用户具有创建表和读写权限

3. **网络连接**: 如果使用远程数据库，确保网络连接稳定可靠

4. **存储空间**: 聊天历史数据可能占用大量存储空间，请确保目标数据库有足够的存储容量

5. **性能考虑**: 如果聊天历史数据库与主数据库在不同的服务器上，可能会有网络延迟影响

## 故障排除

### 常见错误

1. **连接失败**
   ```
   failed to initialize MES database: dial tcp: connection refused
   ```
   - 检查数据库服务是否启动
   - 验证主机名和端口号是否正确

2. **认证失败**
   ```
   failed to initialize MES database: Access denied for user
   ```
   - 检查用户名和密码是否正确
   - 验证用户是否有访问该数据库的权限

3. **数据库不存在**
   ```
   failed to initialize MES database: Unknown database
   ```
   - MySQL: 系统会尝试自动创建数据库，如果失败请检查用户权限
   - PostgreSQL: 请手动创建目标数据库
   - 确保数据库用户具有适当的权限

### 调试方法

1. 查看系统日志，确认 MES 数据库初始化状态
2. 检查环境变量设置是否正确
3. 使用数据库客户端工具验证连接参数

## 版本兼容性

- 此功能从 v1.x.x 版本开始支持
- 完全向后兼容，不影响现有部署
- 未设置 `MES_SQL_DSN` 时，行为与之前版本完全相同 