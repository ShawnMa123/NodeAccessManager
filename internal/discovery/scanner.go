package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Scanner 进程扫描器
type Scanner struct {
	// 支持的进程名列表
	supportedProcesses []string
}

// NewScanner 创建扫描器实例
func NewScanner() *Scanner {
	return &Scanner{
		supportedProcesses: []string{"xray", "sing-box"},
	}
}

// ScanProcesses 扫描系统中的代理进程
func (s *Scanner) ScanProcesses() (*ScanResult, error) {
	result := &ScanResult{
		Processes: []ProxyProcess{},
		ScannedAt: time.Now(),
	}

	// 扫描 /proc 目录
	pids, err := s.scanProc()
	if err != nil {
		return nil, fmt.Errorf("扫描进程失败: %w", err)
	}

	// 为每个PID获取详细信息
	for _, pid := range pids {
		process, err := s.getProcessInfo(pid)
		if err != nil {
			// 进程可能已退出，跳过
			continue
		}

		result.Processes = append(result.Processes, *process)
	}

	result.Total = len(result.Processes)

	return result, nil
}

// scanProc 扫描 /proc 目录识别目标进程
func (s *Scanner) scanProc() ([]int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("读取 /proc 目录失败: %w", err)
	}

	var pids []int

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// 尝试将目录名转换为PID
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue // 不是数字目录，跳过
		}

		// 读取进程名
		commPath := filepath.Join("/proc", entry.Name(), "comm")
		data, err := os.ReadFile(commPath)
		if err != nil {
			continue // 进程可能已退出
		}

		procName := strings.TrimSpace(string(data))

		// 检查是否是支持的进程
		if s.isSupported(procName) {
			pids = append(pids, pid)
		}
	}

	return pids, nil
}

// getProcessInfo 获取进程详细信息
func (s *Scanner) getProcessInfo(pid int) (*ProxyProcess, error) {
	// 读取进程名
	commPath := filepath.Join("/proc", strconv.Itoa(pid), "comm")
	data, err := os.ReadFile(commPath)
	if err != nil {
		return nil, fmt.Errorf("读取进程名失败: %w", err)
	}

	procName := strings.TrimSpace(string(data))

	process := &ProxyProcess{
		PID:       pid,
		Name:      procName,
		ScannedAt: time.Now(),
	}

	// 定位配置文件
	configPath, err := s.locateConfigPath(pid, procName)
	if err != nil {
		// 配置文件定位失败不是致命错误，继续
		process.ConfigPath = ""
	} else {
		process.ConfigPath = configPath

		// 解析配置文件
		inbounds, err := s.parseConfig(configPath, procName)
		if err != nil {
			// 解析失败不是致命错误
			process.Inbounds = []Inbound{}
		} else {
			process.Inbounds = inbounds
		}
	}

	return process, nil
}

// locateConfigPath 定位配置文件路径
func (s *Scanner) locateConfigPath(pid int, procName string) (string, error) {
	// 读取 /proc/[PID]/cmdline
	cmdlinePath := filepath.Join("/proc", strconv.Itoa(pid), "cmdline")
	data, err := os.ReadFile(cmdlinePath)
	if err != nil {
		return "", fmt.Errorf("读取 cmdline 失败: %w", err)
	}

	// cmdline 使用 \x00 分隔参数
	args := strings.Split(string(data), "\x00")

	// 解析启动参数
	for i, arg := range args {
		// Xray: -c / -config
		if (arg == "-c" || arg == "-config") && i+1 < len(args) {
			return args[i+1], nil
		}

		// Sing-box: -c / -C / --config
		if (arg == "-c" || arg == "-C" || arg == "--config") && i+1 < len(args) {
			return args[i+1], nil
		}

		// Sing-box: run -c config.json
		if arg == "run" && i+2 < len(args) {
			if args[i+1] == "-c" || args[i+1] == "-C" || args[i+1] == "--config" {
				return args[i+2], nil
			}
		}
	}

	// 尝试默认路径
	defaultPaths := s.getDefaultPaths(procName)
	for _, path := range defaultPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("无法定位配置文件")
}

// getDefaultPaths 获取默认配置文件路径
func (s *Scanner) getDefaultPaths(procName string) []string {
	switch procName {
	case "xray":
		return []string{
			"/etc/xray/config.json",
			"/usr/local/etc/xray/config.json",
			"/etc/xray/config.jsonc",
			"/usr/local/etc/xray/config.jsonc",
		}
	case "sing-box":
		return []string{
			"/etc/sing-box/config.json",
			"/usr/local/etc/sing-box/config.json",
			"/etc/sing-box/config.jsonc",
			"/usr/local/etc/sing-box/config.jsonc",
		}
	default:
		return []string{}
	}
}

// isSupported 检查进程名是否受支持
func (s *Scanner) isSupported(procName string) bool {
	for _, supported := range s.supportedProcesses {
		if procName == supported {
			return true
		}
	}
	return false
}

// parseConfig 解析配置文件
func (s *Scanner) parseConfig(configPath string, proxyType string) ([]Inbound, error) {
	return ParseConfig(configPath, proxyType)
}
