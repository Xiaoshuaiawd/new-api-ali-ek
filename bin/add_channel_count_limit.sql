-- 添加渠道次数限制相关字段
-- 为channels表添加次数限制功能

-- 添加次数限制开关字段
ALTER TABLE channels ADD COLUMN count_limit_enabled BOOLEAN DEFAULT FALSE;

-- 添加次数限制值字段
ALTER TABLE channels ADD COLUMN count_limit BIGINT DEFAULT 0;

-- 添加已使用次数字段
ALTER TABLE channels ADD COLUMN used_count BIGINT DEFAULT 0;

-- 添加自动重置开关字段
ALTER TABLE channels ADD COLUMN auto_reset_enabled BOOLEAN DEFAULT FALSE;

-- 添加自动重置间隔字段（秒）
ALTER TABLE channels ADD COLUMN auto_reset_interval BIGINT DEFAULT 0;

-- 添加禁用时间字段（用于自动重置计时）
ALTER TABLE channels ADD COLUMN last_reset_time BIGINT DEFAULT 0;

-- 为新字段添加索引以提高查询性能
CREATE INDEX idx_channels_count_limit_enabled ON channels(count_limit_enabled);
CREATE INDEX idx_channels_used_count ON channels(used_count);
CREATE INDEX idx_channels_auto_reset_enabled ON channels(auto_reset_enabled);

-- 更新现有记录，确保字段有默认值
UPDATE channels SET count_limit_enabled = FALSE WHERE count_limit_enabled IS NULL;
UPDATE channels SET count_limit = 0 WHERE count_limit IS NULL;
UPDATE channels SET used_count = 0 WHERE used_count IS NULL;
UPDATE channels SET auto_reset_enabled = FALSE WHERE auto_reset_enabled IS NULL;
UPDATE channels SET auto_reset_interval = 0 WHERE auto_reset_interval IS NULL;
UPDATE channels SET last_reset_time = 0 WHERE last_reset_time IS NULL; 