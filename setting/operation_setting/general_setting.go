package operation_setting

import "one-api/setting/config"

type GeneralSetting struct {
	DocsLink                string `json:"docs_link"`
	PingIntervalEnabled     bool   `json:"ping_interval_enabled"`
	PingIntervalSeconds     int    `json:"ping_interval_seconds"`
	RetryDisableStatusCodes string `json:"retry_disable_status_codes"`
	EmptyResponseRetryEnabled bool `json:"empty_response_retry_enabled"`
}

// 默认配置
var generalSetting = GeneralSetting{
	DocsLink:                  "https://docs.newapi.pro",
	PingIntervalEnabled:       false,
	PingIntervalSeconds:       60,
	RetryDisableStatusCodes:   "401,403,429,500,502,503",
	EmptyResponseRetryEnabled: false,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("general_setting", &generalSetting)
}

func GetGeneralSetting() *GeneralSetting {
	return &generalSetting
}
