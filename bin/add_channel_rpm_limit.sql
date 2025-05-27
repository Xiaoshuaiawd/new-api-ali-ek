-- 添加渠道RPM限制相关字段
-- 为channels表添加RPM限制功能

-- 添加RPM限制开关字段
ALTER TABLE channels ADD COLUMN rpm_limit_enabled BOOLEAN DEFAULT FALSE;

-- 添加RPM限制值字段（每分钟请求次数）
ALTER TABLE channels ADD COLUMN rpm_limit BIGINT DEFAULT 0;

-- 添加上一分钟时间戳字段
ALTER TABLE channels ADD COLUMN last_minute_time BIGINT DEFAULT 0;

-- 添加当前分钟已使用次数字段
ALTER TABLE channels ADD COLUMN current_minute_used BIGINT DEFAULT 0;

-- 为新字段添加索引以提高查询性能
CREATE INDEX idx_channels_rpm_limit_enabled ON channels(rpm_limit_enabled);
CREATE INDEX idx_channels_last_minute_time ON channels(last_minute_time);

-- 更新现有记录，确保字段有默认值
UPDATE channels SET rpm_limit_enabled = FALSE WHERE rpm_limit_enabled IS NULL;
UPDATE channels SET rpm_limit = 0 WHERE rpm_limit IS NULL;
UPDATE channels SET last_minute_time = 0 WHERE last_minute_time IS NULL;
UPDATE channels SET current_minute_used = 0 WHERE current_minute_used IS NULL; 