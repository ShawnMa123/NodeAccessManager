package monitor

import (
	"sync"
	"time"
)

// PortTracker 端口会话追踪器
type PortTracker struct {
	Port     int                 `json:"port"`
	Sessions map[string]*Session `json:"sessions"` // key: IP address
	mu       sync.RWMutex
}

// NewPortTracker 创建端口追踪器
func NewPortTracker(port int) *PortTracker {
	return &PortTracker{
		Port:     port,
		Sessions: make(map[string]*Session),
	}
}

// Update 更新会话状态
func (pt *PortTracker) Update(connections []Connection) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	now := time.Now()
	currentIPs := make(map[string]bool)

	// 1. 更新现有会话 + 记录新会话
	for _, conn := range connections {
		ip := conn.RemoteAddr
		currentIPs[ip] = true

		if session, exists := pt.Sessions[ip]; exists {
			// 已存在的会话，只更新 LastSeenAt
			session.LastSeenAt = now
			session.ConnectionNum = countIPConnections(connections, ip)
		} else {
			// 新会话，记录首次连接时间
			pt.Sessions[ip] = &Session{
				IP:            ip,
				Port:          pt.Port,
				FirstSeenAt:   now,
				LastSeenAt:    now,
				ConnectionNum: countIPConnections(connections, ip),
				TotalBytes:    0,
			}
		}
	}

	// 2. 清理已断开的会话
	for ip := range pt.Sessions {
		if !currentIPs[ip] {
			delete(pt.Sessions, ip)
		}
	}
}

// countIPConnections 统计指定 IP 的连接数
func countIPConnections(connections []Connection, ip string) int {
	count := 0
	for _, conn := range connections {
		if conn.RemoteAddr == ip {
			count++
		}
	}
	return count
}

// GetActiveSessions 获取活跃会话列表
func (pt *PortTracker) GetActiveSessions() []*Session {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	sessions := make([]*Session, 0, len(pt.Sessions))
	for _, session := range pt.Sessions {
		// 返回副本，避免并发修改
		sessionCopy := *session
		sessions = append(sessions, &sessionCopy)
	}

	return sessions
}

// GetSessionByIP 根据 IP 获取会话
func (pt *PortTracker) GetSessionByIP(ip string) (*Session, bool) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	session, exists := pt.Sessions[ip]
	if !exists {
		return nil, false
	}

	// 返回副本
	sessionCopy := *session
	return &sessionCopy, true
}

// Count 获取当前会话数
func (pt *PortTracker) Count() int {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return len(pt.Sessions)
}

// GetStats 获取端口统计信息
func (pt *PortTracker) GetStats() PortStats {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	totalConnections := 0
	for _, session := range pt.Sessions {
		totalConnections += session.ConnectionNum
	}

	return PortStats{
		Port:             pt.Port,
		ActiveSessions:   len(pt.Sessions),
		TotalConnections: totalConnections,
		UniqueIPs:        len(pt.Sessions),
		LastUpdated:      time.Now(),
	}
}

// Clear 清空所有会话
func (pt *PortTracker) Clear() {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.Sessions = make(map[string]*Session)
}

// RemoveSession 移除指定会话
func (pt *PortTracker) RemoveSession(ip string) bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if _, exists := pt.Sessions[ip]; exists {
		delete(pt.Sessions, ip)
		return true
	}
	return false
}
