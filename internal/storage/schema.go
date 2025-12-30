package storage

// SQL 表结构定义

const (
	// CreateSessionsTable 会话历史表
	CreateSessionsTable = `
CREATE TABLE IF NOT EXISTS sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    port INTEGER NOT NULL,
    ip TEXT NOT NULL,
    first_seen_at DATETIME NOT NULL,
    last_seen_at DATETIME NOT NULL,
    connection_num INTEGER DEFAULT 1,
    total_bytes INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sessions_port_ip ON sessions(port, ip);
CREATE INDEX IF NOT EXISTS idx_sessions_first_seen ON sessions(first_seen_at);
`

	// CreateBanHistoryTable 封禁历史表
	CreateBanHistoryTable = `
CREATE TABLE IF NOT EXISTS ban_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    port INTEGER NOT NULL,
    ip TEXT NOT NULL,
    banned_at DATETIME NOT NULL,
    expire_at DATETIME NOT NULL,
    duration INTEGER NOT NULL,
    strategy TEXT NOT NULL,
    reason TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_ban_port_ip ON ban_history(port, ip);
CREATE INDEX IF NOT EXISTS idx_ban_time ON ban_history(banned_at);
`

	// CreateStatisticsTable 统计表
	CreateStatisticsTable = `
CREATE TABLE IF NOT EXISTS statistics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    port INTEGER NOT NULL,
    hour DATETIME NOT NULL,
    unique_ips INTEGER NOT NULL,
    total_bans INTEGER NOT NULL,
    avg_sessions REAL NOT NULL,
    max_sessions INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_stats_port_hour ON statistics(port, hour);
`
)
