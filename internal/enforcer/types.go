package enforcer

import "time"

// BanRecord 封禁记录
type BanRecord struct {
	IP       string    `json:"ip"`
	Port     int       `json:"port"`
	BannedAt time.Time `json:"banned_at"`
	ExpireAt time.Time `json:"expire_at"`
	Duration int       `json:"duration"` // 秒
	Reason   string    `json:"reason"`   // 封禁原因
	Strategy string    `json:"strategy"` // FIFO/LIFO/MANUAL
}

// VictimSelection 驱逐选择结果
type VictimSelection struct {
	Victims  []string `json:"victims"`  // 被驱逐的 IP 列表
	Strategy string   `json:"strategy"` // 使用的策略
	Total    int      `json:"total"`    // 总会话数
	Overlimit int     `json:"overlimit"` // 超限数量
}
