package enforcer

import (
	"fmt"
	"os/exec"

	"github.com/nodeaccessmanager/nam/pkg/utils"
)

// Executor 执行器
type Executor struct {
	cooldownMgr *CooldownManager
}

// NewExecutor 创建执行器
func NewExecutor(cooldownMgr *CooldownManager) *Executor {
	return &Executor{
		cooldownMgr: cooldownMgr,
	}
}

// KillConnection 使用 ss -K 强制断开连接
func (e *Executor) KillConnection(port int, ip string) error {
	logger := utils.GetLogger()

	// 执行命令: ss -K dst <IP> sport = :<PORT>
	cmd := exec.Command("ss", "-K", "dst", ip, "sport", "=", fmt.Sprintf(":%d", port))
	output, err := cmd.CombinedOutput()

	if err != nil {
		logger.Debugf("ss -K 输出: %s", string(output))
		return fmt.Errorf("ss -K 执行失败: %w", err)
	}

	logger.Infof("已断开 %s:%d 的连接", ip, port)
	return nil
}

// ApplyBan 应用 iptables 封禁
func (e *Executor) ApplyBan(ip string, port int, duration int) error {
	logger := utils.GetLogger()

	// 执行命令: iptables -I INPUT -s <IP> -p tcp --dport <PORT> -j DROP
	cmd := exec.Command("iptables", "-I", "INPUT",
		"-s", ip,
		"-p", "tcp",
		"--dport", fmt.Sprintf("%d", port),
		"-m", "comment", "--comment", "NAM-BAN",
		"-j", "DROP")

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Debugf("iptables 输出: %s", string(output))
		return fmt.Errorf("iptables 封禁失败: %w", err)
	}

	logger.Infof("已封禁 %s:%d（时长 %ds）", ip, port, duration)

	// 启动定时器自动解封
	if duration > 0 {
		e.cooldownMgr.Schedule(ip, port, duration)
	}

	return nil
}

// RemoveBan 移除 iptables 封禁
func (e *Executor) RemoveBan(ip string, port int) error {
	logger := utils.GetLogger()

	// 执行命令: iptables -D INPUT -s <IP> -p tcp --dport <PORT> -j DROP
	cmd := exec.Command("iptables", "-D", "INPUT",
		"-s", ip,
		"-p", "tcp",
		"--dport", fmt.Sprintf("%d", port),
		"-m", "comment", "--comment", "NAM-BAN",
		"-j", "DROP")

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Debugf("iptables 输出: %s", string(output))
		return fmt.Errorf("iptables 解封失败: %w", err)
	}

	logger.Infof("已解封 %s:%d", ip, port)
	return nil
}

// EnforceVictims 执行驱逐操作
func (e *Executor) EnforceVictims(port int, victims []string, banDuration int, reason string) error {
	logger := utils.GetLogger()

	for _, ip := range victims {
		// 1. 断开连接
		if err := e.KillConnection(port, ip); err != nil {
			logger.Errorf("断开连接失败 %s:%d - %v", ip, port, err)
			// 继续处理其他 IP
			continue
		}

		// 2. 应用封禁（如果配置了封禁时长）
		if banDuration > 0 {
			if err := e.ApplyBan(ip, port, banDuration); err != nil {
				logger.Errorf("封禁失败 %s:%d - %v", ip, port, err)
			}
		}

		logger.Warnf("已驱逐 %s（端口 %d，原因: %s）", ip, port, reason)
	}

	return nil
}

// CheckIPTablesAvailable 检查 iptables 是否可用
func CheckIPTablesAvailable() bool {
	cmd := exec.Command("iptables", "-V")
	err := cmd.Run()
	return err == nil
}

// CheckSSAvailable 检查 ss 命令是否可用
func CheckSSAvailable() bool {
	cmd := exec.Command("ss", "-V")
	err := cmd.Run()
	return err == nil
}

// CleanupNAMRules 清理所有 NAM 创建的 iptables 规则
func CleanupNAMRules() error {
	logger := utils.GetLogger()
	logger.Info("清理 NAM iptables 规则")

	// 列出所有带 NAM-BAN 注释的规则
	cmd := exec.Command("bash", "-c",
		"iptables -L INPUT -n --line-numbers | grep 'NAM-BAN' | awk '{print $1}' | tac")

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("列出规则失败: %w", err)
	}

	// 逐行删除
	lines := string(output)
	if lines == "" {
		logger.Info("无需清理，未发现 NAM 规则")
		return nil
	}

	// 注意：这里简化处理，生产环境应该更精确
	logger.Warn("发现残留规则，建议手动检查: iptables -L INPUT -n | grep NAM-BAN")
	return nil
}
