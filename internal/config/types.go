package config

import "time"

// Config 全局配置结构
type Config struct {
	Global    GlobalConfig `yaml:"global"`
	Rules     []Rule       `yaml:"rules"`
	Discovery Discovery    `yaml:"discovery,omitempty"`
}

// GlobalConfig 全局设置
type GlobalConfig struct {
	// 基础设置
	CheckInterval int      `yaml:"check_interval"` // 检查周期（秒）
	BanDuration   int      `yaml:"ban_duration"`   // 默认封禁时长（秒），0 表示不封禁
	Strategy      Strategy `yaml:"strategy"`       // 默认策略: FIFO / LIFO

	// 日志设置
	LogLevel      string `yaml:"log_level"`       // debug / info / warn / error
	LogFile       string `yaml:"log_file"`        // 日志文件路径
	LogMaxSize    int    `yaml:"log_max_size"`    // 日志文件最大大小（MB）
	LogMaxBackups int    `yaml:"log_max_backups"` // 保留备份数
	LogMaxAge     int    `yaml:"log_max_age"`     // 保留天数

	// 数据库设置
	DatabasePath string `yaml:"database_path"` // SQLite 数据库路径
	HistoryDays  int    `yaml:"history_days"`  // 历史数据保留天数

	// 通知设置（可选）
	Notification NotificationConfig `yaml:"notification,omitempty"`
}

// NotificationConfig 通知配置
type NotificationConfig struct {
	Enabled    bool     `yaml:"enabled"`
	WebhookURL string   `yaml:"webhook_url"` // Telegram/Discord Webhook
	Events     []string `yaml:"events"`      // 触发通知的事件
}

// Rule 端口规则
type Rule struct {
	Port        int      `yaml:"port"`
	Protocol    string   `yaml:"protocol"`
	MaxIPs      int      `yaml:"max_ips"`
	Tag         string   `yaml:"tag"`
	Strategy    Strategy `yaml:"strategy,omitempty"`     // 可覆盖全局策略
	BanDuration int      `yaml:"ban_duration,omitempty"` // 可覆盖全局时长
	Whitelist   []string `yaml:"whitelist,omitempty"`    // 白名单（IP 或 CIDR）
	Blacklist   []string `yaml:"blacklist,omitempty"`    // 黑名单
}

// Strategy 驱逐策略
type Strategy string

const (
	StrategyFIFO Strategy = "FIFO" // 先进先出（新挤旧）
	StrategyLIFO Strategy = "LIFO" // 后进先出（拒绝新入）
)

// Discovery 自动发现历史记录
type Discovery struct {
	LastScanAt        time.Time         `yaml:"last_scan_at"`
	DetectedProcesses []DetectedProcess `yaml:"detected_processes,omitempty"`
}

// DetectedProcess 检测到的进程信息
type DetectedProcess struct {
	PID        int    `yaml:"pid"`
	Name       string `yaml:"name"`
	ConfigPath string `yaml:"config_path"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Global: GlobalConfig{
			CheckInterval: 5,
			BanDuration:   60,
			Strategy:      StrategyFIFO,
			LogLevel:      "info",
			LogFile:       "/var/log/nam.log",
			LogMaxSize:    100,
			LogMaxBackups: 5,
			LogMaxAge:     30,
			DatabasePath:  "/var/lib/nam/nam.db",
			HistoryDays:   30,
			Notification: NotificationConfig{
				Enabled: false,
				Events:  []string{"ban", "overlimit"},
			},
		},
		Rules: []Rule{},
	}
}
