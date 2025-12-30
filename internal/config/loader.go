package config

import (
	"fmt"
	"net"
	"os"

	"gopkg.in/yaml.v3"
)

// Load 从文件加载配置
func Load(path string) (*Config, error) {
	// 读取文件
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("配置文件不存在: %s", path)
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// YAML 解析
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("配置文件格式错误: %w", err)
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return &config, nil
}

// Save 保存配置到文件
func Save(config *Config, path string) error {
	// 验证配置
	if err := config.Validate(); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	// 序列化为 YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// Validate 验证配置的合法性
func (c *Config) Validate() error {
	// 检查必需字段
	if c.Global.CheckInterval <= 0 {
		return fmt.Errorf("check_interval 必须大于 0")
	}

	if c.Global.CheckInterval > 3600 {
		return fmt.Errorf("check_interval 不应超过 3600 秒")
	}

	if c.Global.BanDuration < 0 {
		return fmt.Errorf("ban_duration 不能为负数")
	}

	// 验证策略
	if c.Global.Strategy != StrategyFIFO && c.Global.Strategy != StrategyLIFO {
		return fmt.Errorf("不支持的策略: %s（仅支持 FIFO 或 LIFO）", c.Global.Strategy)
	}

	// 验证日志级别
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[c.Global.LogLevel] {
		return fmt.Errorf("不支持的日志级别: %s", c.Global.LogLevel)
	}

	// 检查端口规则
	if len(c.Rules) == 0 {
		return fmt.Errorf("至少需要配置一个端口规则")
	}

	// 检查端口唯一性
	ports := make(map[int]bool)
	for _, rule := range c.Rules {
		if err := rule.Validate(); err != nil {
			return fmt.Errorf("端口 %d 的规则无效: %w", rule.Port, err)
		}

		if ports[rule.Port] {
			return fmt.Errorf("端口 %d 重复配置", rule.Port)
		}
		ports[rule.Port] = true
	}

	return nil
}

// Validate 验证规则的合法性
func (r *Rule) Validate() error {
	// 验证端口范围
	if r.Port < 1 || r.Port > 65535 {
		return fmt.Errorf("端口号必须在 1-65535 之间")
	}

	// 验证最大 IP 数
	if r.MaxIPs <= 0 {
		return fmt.Errorf("max_ips 必须大于 0")
	}

	// 验证策略（如果设置）
	if r.Strategy != "" && r.Strategy != StrategyFIFO && r.Strategy != StrategyLIFO {
		return fmt.Errorf("不支持的策略: %s", r.Strategy)
	}

	// 验证白名单 CIDR 格式
	for _, cidr := range r.Whitelist {
		if err := validateCIDR(cidr); err != nil {
			return fmt.Errorf("白名单中的 CIDR 无效 (%s): %w", cidr, err)
		}
	}

	// 验证黑名单 CIDR 格式
	for _, cidr := range r.Blacklist {
		if err := validateCIDR(cidr); err != nil {
			return fmt.Errorf("黑名单中的 CIDR 无效 (%s): %w", cidr, err)
		}
	}

	return nil
}

// validateCIDR 验证 CIDR 格式或单个 IP
func validateCIDR(cidr string) error {
	// 尝试解析为单个 IP
	if ip := net.ParseIP(cidr); ip != nil {
		return nil
	}

	// 尝试解析为 CIDR
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("既不是有效的 IP 也不是有效的 CIDR: %w", err)
	}

	return nil
}

// GetRuleByPort 根据端口号获取规则
func (c *Config) GetRuleByPort(port int) *Rule {
	for i := range c.Rules {
		if c.Rules[i].Port == port {
			return &c.Rules[i]
		}
	}
	return nil
}

// GetEffectiveStrategy 获取规则的有效策略（考虑全局默认值）
func (r *Rule) GetEffectiveStrategy(global Strategy) Strategy {
	if r.Strategy != "" {
		return r.Strategy
	}
	return global
}

// GetEffectiveBanDuration 获取规则的有效封禁时长（考虑全局默认值）
func (r *Rule) GetEffectiveBanDuration(global int) int {
	if r.BanDuration > 0 {
		return r.BanDuration
	}
	return global
}
