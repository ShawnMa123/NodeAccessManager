package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nodeaccessmanager/nam/internal/enforcer"
	"github.com/nodeaccessmanager/nam/internal/monitor"
	"github.com/nodeaccessmanager/nam/pkg/utils"
)

// Database SQLite 数据库封装
type Database struct {
	db   *sql.DB
	path string
}

// NewDatabase 创建数据库实例
func NewDatabase(path string) (*Database, error) {
	logger := utils.GetLogger()

	// 确保数据库目录存在
	dbDir := filepath.Dir(path)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %w", err)
	}

	// 打开数据库
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连接测试失败: %w", err)
	}

	// 创建表
	if err := createTables(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("创建表失败: %w", err)
	}

	logger.Infof("数据库初始化成功: %s", path)

	return &Database{
		db:   db,
		path: path,
	}, nil
}

// createTables 创建所有表
func createTables(db *sql.DB) error {
	tables := []string{
		CreateSessionsTable,
		CreateBanHistoryTable,
		CreateStatisticsTable,
	}

	for _, table := range tables {
		if _, err := db.Exec(table); err != nil {
			return err
		}
	}

	return nil
}

// Close 关闭数据库连接
func (d *Database) Close() error {
	logger := utils.GetLogger()
	logger.Info("关闭数据库连接")
	return d.db.Close()
}

// RecordSession 记录会话
func (d *Database) RecordSession(session *monitor.Session) error {
	query := `
INSERT INTO sessions (port, ip, first_seen_at, last_seen_at, connection_num)
VALUES (?, ?, ?, ?, ?)
`
	_, err := d.db.Exec(query,
		session.Port,
		session.IP,
		session.FirstSeenAt,
		session.LastSeenAt,
		session.ConnectionNum,
	)

	return err
}

// RecordBan 记录封禁历史
func (d *Database) RecordBan(record *enforcer.BanRecord) error {
	query := `
INSERT INTO ban_history (port, ip, banned_at, expire_at, duration, strategy, reason)
VALUES (?, ?, ?, ?, ?, ?, ?)
`
	_, err := d.db.Exec(query,
		record.Port,
		record.IP,
		record.BannedAt,
		record.ExpireAt,
		record.Duration,
		record.Strategy,
		record.Reason,
	)

	return err
}

// GetBanHistory 获取封禁历史
func (d *Database) GetBanHistory(port int, limit int) ([]enforcer.BanRecord, error) {
	query := `
SELECT ip, port, banned_at, expire_at, duration, strategy, reason
FROM ban_history
WHERE port = ?
ORDER BY banned_at DESC
LIMIT ?
`
	rows, err := d.db.Query(query, port, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []enforcer.BanRecord
	for rows.Next() {
		var record enforcer.BanRecord
		var reason sql.NullString

		err := rows.Scan(
			&record.IP,
			&record.Port,
			&record.BannedAt,
			&record.ExpireAt,
			&record.Duration,
			&record.Strategy,
			&reason,
		)
		if err != nil {
			return nil, err
		}

		if reason.Valid {
			record.Reason = reason.String
		}

		records = append(records, record)
	}

	return records, nil
}

// RecordStatistics 记录统计数据
func (d *Database) RecordStatistics(port int, stats *PortStatistics) error {
	query := `
INSERT OR REPLACE INTO statistics 
(port, hour, unique_ips, total_bans, avg_sessions, max_sessions)
VALUES (?, ?, ?, ?, ?, ?)
`
	_, err := d.db.Exec(query,
		port,
		stats.Hour,
		stats.UniqueIPs,
		stats.TotalBans,
		stats.AvgSessions,
		stats.MaxSessions,
	)

	return err
}

// GetStatistics 获取统计数据
func (d *Database) GetStatistics(port int, hours int) ([]PortStatistics, error) {
	query := `
SELECT hour, unique_ips, total_bans, avg_sessions, max_sessions
FROM statistics
WHERE port = ? AND hour >= datetime('now', '-' || ? || ' hours')
ORDER BY hour DESC
`
	rows, err := d.db.Query(query, port, hours)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []PortStatistics
	for rows.Next() {
		var stat PortStatistics
		var hourStr string

		err := rows.Scan(
			&hourStr,
			&stat.UniqueIPs,
			&stat.TotalBans,
			&stat.AvgSessions,
			&stat.MaxSessions,
		)
		if err != nil {
			return nil, err
		}

		// 解析时间
		stat.Hour, _ = time.Parse("2006-01-02 15:04:05", hourStr)
		stats = append(stats, stat)
	}

	return stats, nil
}

// PortStatistics 端口统计数据
type PortStatistics struct {
	Hour        time.Time
	UniqueIPs   int
	TotalBans   int
	AvgSessions float64
	MaxSessions int
}

// Cleanup 清理旧数据
func (d *Database) Cleanup(daysToKeep int) error {
	logger := utils.GetLogger()
	logger.Infof("清理 %d 天前的数据", daysToKeep)

	tables := []string{"sessions", "ban_history", "statistics"}

	for _, table := range tables {
		query := fmt.Sprintf(`
DELETE FROM %s 
WHERE created_at < datetime('now', '-'  || ? || ' days')
`, table)

		result, err := d.db.Exec(query, daysToKeep)
		if err != nil {
			return err
		}

		affected, _ := result.RowsAffected()
		logger.Infof("清理 %s 表: %d 条记录", table, affected)
	}

	// 压缩数据库
	if _, err := d.db.Exec("VACUUM"); err != nil {
		logger.Warnf("数据库压缩失败: %v", err)
	}

	return nil
}

// GetDatabaseSize 获取数据库文件大小
func (d *Database) GetDatabaseSize() (int64, error) {
	fileInfo, err := os.Stat(d.path)
	if err != nil {
		return 0, err
	}
	return fileInfo.Size(), nil
}
