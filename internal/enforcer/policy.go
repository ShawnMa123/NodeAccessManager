package enforcer

import (
	"net"
	"sort"

	"github.com/nodeaccessmanager/nam/internal/config"
	"github.com/nodeaccessmanager/nam/internal/monitor"
)

// PolicyEngine 策略引擎
type PolicyEngine struct {
	config *config.Config
}

// NewPolicyEngine 创建策略引擎
func NewPolicyEngine(cfg *config.Config) *PolicyEngine {
	return &PolicyEngine{
		config: cfg,
	}
}

// SelectVictims 选择需要驱逐的会话
func (pe *PolicyEngine) SelectVictims(
	port int,
	sessions []*monitor.Session,
	overlimit int,
) *VictimSelection {
	rule := pe.config.GetRuleByPort(port)
	if rule == nil {
		return &VictimSelection{
			Victims:   []string{},
			Strategy:  "UNKNOWN",
			Total:     len(sessions),
			Overlimit: overlimit,
		}
	}

	// 获取有效策略
	strategy := rule.GetEffectiveStrategy(pe.config.Global.Strategy)

	// 过滤白名单 IP
	var candidates []*monitor.Session
	for _, session := range sessions {
		if !pe.isWhitelisted(rule, session.IP) {
			candidates = append(candidates, session)
		}
	}

	// 如果过滤后的候选者不足，驱逐所有候选者
	if len(candidates) <= overlimit {
		var victims []string
		for _, session := range candidates {
			victims = append(victims, session.IP)
		}
		return &VictimSelection{
			Victims:   victims,
			Strategy:  string(strategy),
			Total:     len(sessions),
			Overlimit: overlimit,
		}
	}

	// 根据策略排序
	if strategy == config.StrategyFIFO {
		// FIFO: 按首次连接时间升序（最早的在前）
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].FirstSeenAt.Before(candidates[j].FirstSeenAt)
		})
	} else {
		// LIFO: 按首次连接时间降序（最新的在前）
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].FirstSeenAt.After(candidates[j].FirstSeenAt)
		})
	}

	// 取前 N 个作为驱逐对象
	victims := make([]string, overlimit)
	for i := 0; i < overlimit; i++ {
		victims[i] = candidates[i].IP
	}

	return &VictimSelection{
		Victims:   victims,
		Strategy:  string(strategy),
		Total:     len(sessions),
		Overlimit: overlimit,
	}
}

// isWhitelisted 检查 IP 是否在白名单
func (pe *PolicyEngine) isWhitelisted(rule *config.Rule, ip string) bool {
	for _, cidr := range rule.Whitelist {
		if matchCIDR(ip, cidr) {
			return true
		}
	}
	return false
}

// IsBlacklisted 检查 IP 是否在黑名单
func (pe *PolicyEngine) IsBlacklisted(port int, ip string) bool {
	rule := pe.config.GetRuleByPort(port)
	if rule == nil {
		return false
	}

	for _, cidr := range rule.Blacklist {
		if matchCIDR(ip, cidr) {
			return true
		}
	}
	return false
}

// matchCIDR 检查 IP 是否匹配 CIDR
func matchCIDR(ip, cidr string) bool {
	// 尝试解析为单个 IP
	if ip == cidr {
		return true
	}

	// 尝试解析为 CIDR
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		// 不是有效的 CIDR，尝试作为单个 IP 比较
		return ip == cidr
	}

	// 解析目标 IP
	targetIP := net.ParseIP(ip)
	if targetIP == nil {
		return false
	}

	// 检查是否在网段内
	return ipNet.Contains(targetIP)
}
