package discovery

import "time"

// ProxyProcess 代理进程信息
type ProxyProcess struct {
	PID        int        `json:"pid"`
	Name       string     `json:"name"`        // "xray" or "sing-box"
	ConfigPath string     `json:"config_path"` // 配置文件路径
	Inbounds   []Inbound  `json:"inbounds"`    // 监听端口列表
	ScannedAt  time.Time  `json:"scanned_at"`  // 扫描时间
}

// Inbound 入站配置
type Inbound struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Tag      string `json:"tag"`
	Listen   string `json:"listen"` // 监听地址 (0.0.0.0 / 127.0.0.1 / ::)
}

// ScanResult 扫描结果
type ScanResult struct {
	Processes []ProxyProcess `json:"processes"`
	Total     int            `json:"total"`
	ScannedAt time.Time      `json:"scanned_at"`
}
