package monitor

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Collector 连接采集器
type Collector struct {
	// 可扩展配置
}

// NewCollector 创建采集器实例
func NewCollector() *Collector {
	return &Collector{}
}

// CollectConnections 采集指定端口的连接信息
func (c *Collector) CollectConnections(port int) ([]Connection, error) {
	// 执行 ss 命令: ss -tn state established sport = :<PORT>
	cmd := exec.Command("ss", "-tn", "state", "established",
		"sport", "=", fmt.Sprintf(":%d", port))

	output, err := cmd.Output()
	if err != nil {
		// ss 命令执行失败，可能是权限问题或命令不存在
		return nil, fmt.Errorf("执行 ss 命令失败: %w", err)
	}

	return c.parseSSOutput(output)
}

// parseSSOutput 解析 ss 命令输出
func (c *Collector) parseSSOutput(output []byte) ([]Connection, error) {
	// ss 输出格式:
	// State   Recv-Q   Send-Q   Local Address:Port   Peer Address:Port   Process
	// ESTAB   0        0        0.0.0.0:443          203.0.113.1:52341

	lines := strings.Split(string(output), "\n")
	var connections []Connection

	now := time.Now()

	for i, line := range lines {
		// 跳过表头和空行
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue // 字段不完整，跳过
		}

		// 解析字段
		state := fields[0]
		recvQ, _ := strconv.Atoi(fields[1])
		sendQ, _ := strconv.Atoi(fields[2])
		localAddr := fields[3]
		peerAddr := fields[4]

		// 解析本地地址
		localIP, localPort, err := parseAddr(localAddr)
		if err != nil {
			continue
		}

		// 解析远程地址
		remoteIP, remotePort, err := parseAddr(peerAddr)
		if err != nil {
			continue
		}

		connections = append(connections, Connection{
			LocalAddr:  localIP,
			LocalPort:  localPort,
			RemoteAddr: remoteIP,
			RemotePort: remotePort,
			State:      state,
			RecvQ:      recvQ,
			SendQ:      sendQ,
			DetectedAt: now,
		})
	}

	return connections, nil
}

// parseAddr 解析地址字符串 "IP:Port" 或 "[IPv6]:Port"
func parseAddr(addr string) (string, int, error) {
	// 处理 IPv6 格式: [2001:db8::1]:8080
	// 处理 IPv4 格式: 192.0.2.1:8080

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, fmt.Errorf("解析地址失败: %w", err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("解析端口失败: %w", err)
	}

	return host, port, nil
}

// CollectAllPorts 批量采集多个端口的连接
func (c *Collector) CollectAllPorts(ports []int) (map[int][]Connection, error) {
	result := make(map[int][]Connection)

	for _, port := range ports {
		connections, err := c.CollectConnections(port)
		if err != nil {
			// 单个端口失败不影响其他端口
			continue
		}
		result[port] = connections
	}

	return result, nil
}

// GetUniqueIPs 从连接列表中提取唯一 IP
func GetUniqueIPs(connections []Connection) []string {
	ipMap := make(map[string]bool)
	for _, conn := range connections {
		ipMap[conn.RemoteAddr] = true
	}

	ips := make([]string, 0, len(ipMap))
	for ip := range ipMap {
		ips = append(ips, ip)
	}

	return ips
}
