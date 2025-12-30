package enforcer

import (
	"time"

	"github.com/nodeaccessmanager/nam/internal/config"
	"github.com/nodeaccessmanager/nam/internal/monitor"
	"github.com/nodeaccessmanager/nam/pkg/utils"
)

// Enforcer 策略执行器主控制器
type Enforcer struct {
	config      *config.Config
	policyEngine *PolicyEngine
	executor    *Executor
	cooldownMgr *CooldownManager
}

// NewEnforcer 创建执行器实例
func NewEnforcer(cfg *config.Config) *Enforcer {
	cooldownMgr := NewCooldownManager()
	executor := NewExecutor(cooldownMgr)
	cooldownMgr.SetExecutor(executor) // 解决循环依赖

	return &Enforcer{
		config:       cfg,
		policyEngine: NewPolicyEngine(cfg),
		executor:     executor,
		cooldownMgr:  cooldownMgr,
	}
}

// Enforce 执行策略（当端口超限时调用）
func (e *Enforcer) Enforce(port int, tracker *monitor.PortTracker, rule *config.Rule) {
	logger := utils.GetLogger()

	// 1. 获取当前会话
	sessions := tracker.GetActiveSessions()
	currentCount := len(sessions)

	if currentCount <= rule.MaxIPs {
		// 未超限，无需处理
		return
	}

	overlimit := currentCount - rule.MaxIPs
	logger.Warnf("端口 %d 超限: 当前 %d IP，最大 %d IP，需驱逐 %d 个",
		port, currentCount, rule.MaxIPs, overlimit)

	// 2. 选择驱逐对象
	selection := e.policyEngine.SelectVictims(port, sessions, overlimit)

	if len(selection.Victims) == 0 {
		logger.Warn("未选出驱逐对象（可能都在白名单）")
		return
	}

	logger.Infof("选出 %d 个驱逐对象（策略: %s）", len(selection.Victims), selection.Strategy)

	// 3. 执行驱逐
	banDuration := rule.GetEffectiveBanDuration(e.config.Global.BanDuration)
	reason := "Overlimit"

	if err := e.executor.EnforceVictims(port, selection.Victims, banDuration, reason); err != nil {
		logger.Errorf("驱逐执行失败: %v", err)
	}
}

// ManualBan 手动封禁 IP
func (e *Enforcer) ManualBan(ip string, port int, duration int, reason string) error {
	logger := utils.GetLogger()
	logger.Infof("手动封禁: %s:%d（时长 %ds，原因: %s）", ip, port, duration, reason)

	// 1. 断开连接
	if err := e.executor.KillConnection(port, ip); err != nil {
		logger.Warnf("断开连接失败（可能未连接）: %v", err)
	}

	// 2. 应用封禁
	if err := e.executor.ApplyBan(ip, port, duration); err != nil {
		return err
	}

	return nil
}

// ManualUnban 手动解封 IP
func (e *Enforcer) ManualUnban(ip string, port int) error {
	logger := utils.GetLogger()
	logger.Infof("手动解封: %s:%d", ip, port)

	// 取消定时器并解封
	if err := e.cooldownMgr.Cancel(ip, port); err != nil {
		// 可能不在定时器中，直接尝试解封
		return e.executor.RemoveBan(ip, port)
	}

	return nil
}

// GetActiveBans 获取活跃的封禁列表
func (e *Enforcer) GetActiveBans() []BanRecord {
	return e.cooldownMgr.GetActiveRecords()
}

// IsBanned 检查 IP 是否被封禁
func (e *Enforcer) IsBanned(ip string, port int) bool {
	return e.cooldownMgr.IsActive(ip, port)
}

// GetBanExpireTime 获取封禁过期时间
func (e *Enforcer) GetBanExpireTime(ip string, port int) (time.Time, bool) {
	return e.cooldownMgr.GetExpireTime(ip, port)
}

// CheckBlacklist 检查 IP 是否在黑名单
func (e *Enforcer) CheckBlacklist(ip string, port int) bool {
	return e.policyEngine.IsBlacklisted(port, ip)
}

// Shutdown 关闭执行器
func (e *Enforcer) Shutdown() {
	logger := utils.GetLogger()
	logger.Info("关闭 Enforcer")

	// 清空定时器（不解封，保留封禁状态）
	e.cooldownMgr.Clear()

	logger.Info("Enforcer 已关闭")
}
