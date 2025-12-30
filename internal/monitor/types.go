package monitor

import "time"

// Connection TCP 连接信息
type Connection struct {
	LocalAddr  string    `json:"local_addr"`  // 本地地址
	LocalPort  int       `json:"local_port"`  // 本地端口
	RemoteAddr string    `json:"remote_addr"` // 远程地址
	RemotePort int       `json:"remote_port"` // 远程端口
	State      string    `json:"state"`       // 连接状态
	RecvQ      int       `json:"recv_q"`      // 接收队列
	SendQ      int       `json:"send_q"`      // 发送队列
	DetectedAt time.Time `json:"detected_at"` // 检测时间
}

// Session 会话信息
type Session struct {
	IP            string    `json:"ip"`              // 远程 IP
	Port          int       `json:"port"`            // 本地端口
	FirstSeenAt   time.Time `json:"first_seen_at"`   // 首次连接时间
	LastSeenAt    time.Time `json:"last_seen_at"`    // 最后一次检测到的时间
	ConnectionNum int       `json:"connection_num"`  // 当前连接数
	TotalBytes    uint64    `json:"total_bytes"`     // 总字节数（可选）
}

// PortStats 端口统计信息
type PortStats struct {
	Port              int       `json:"port"`
	ActiveSessions    int       `json:"active_sessions"`     // 活跃会话数
	TotalConnections  int       `json:"total_connections"`   // 总连接数
	UniqueIPs         int       `json:"unique_ips"`          // 独立 IP 数
	LastUpdated       time.Time `json:"last_updated"`        // 最后更新时间
}
