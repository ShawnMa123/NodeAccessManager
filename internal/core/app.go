package core

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nodeaccessmanager/nam/internal/config"
	"github.com/nodeaccessmanager/nam/internal/enforcer"
	"github.com/nodeaccessmanager/nam/internal/monitor"
	"github.com/nodeaccessmanager/nam/internal/storage"
	"github.com/nodeaccessmanager/nam/pkg/utils"
)

// App 核心应用控制器
type App struct {
	config      *config.Config
	coordinator *monitor.Coordinator
	enforcer    *enforcer.Enforcer
	db          *storage.Database
	ruleMap     map[int]*config.Rule // port -> rule

	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.RWMutex
	isRunning  bool
	startTime  time.Time
	configPath string
}

// NewApp 创建应用实例
func NewApp(configPath string) (*App, error) {
	logger := utils.GetLogger()

	// 1. 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}

	// 2. 创建数据库
	dbPath := cfg.Global.DatabasePath
	if dbPath == "" {
		dbPath = "/var/lib/nam/nam.db"
	}
	db, err := storage.NewDatabase(dbPath)
	if err != nil {
		return nil, fmt.Errorf("初始化数据库失败: %w", err)
	}

	// 3. 创建 Enforcer
	enf := enforcer.NewEnforcer(cfg)

	// 4. 创建 Monitor Coordinator
	coord := monitor.NewCoordinator(cfg)

	// 5. 构建 port -> rule 映射
	ruleMap := make(map[int]*config.Rule)
	for i := range cfg.Rules {
		ruleMap[cfg.Rules[i].Port] = &cfg.Rules[i]
	}

	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		config:      cfg,
		coordinator: coord,
		enforcer:    enf,
		db:          db,
		ruleMap:     ruleMap,
		ctx:         ctx,
		cancel:      cancel,
		configPath:  configPath,
	}

	// 6. 设置超限回调
	coord.SetOverlimitCallback(app.handleOverlimit)

	logger.Info("应用实例创建成功")
	return app, nil
}

// Start 启动应用
func (a *App) Start() error {
	a.mu.Lock()
	if a.isRunning {
		a.mu.Unlock()
		return fmt.Errorf("应用已在运行")
	}
	a.isRunning = true
	a.startTime = time.Now()
	a.mu.Unlock()

	logger := utils.GetLogger()
	logger.Info("========== NAM 启动 ==========")

	// 1. 启动监控协调器（会自动初始化所有配置中的端口）
	if err := a.coordinator.Start(); err != nil {
		return fmt.Errorf("启动监控失败: %w", err)
	}

	// 2. 启动数据统计协程
	a.wg.Add(1)
	go a.statisticsWorker()

	// 3. 启动数据库清理协程
	a.wg.Add(1)
	go a.cleanupWorker()

	// 4. 设置信号处理
	a.setupSignalHandler()

	logger.Info("NAM 启动完成，开始监控...")
	return nil
}

// Stop 停止应用
func (a *App) Stop() error {
	a.mu.Lock()
	if !a.isRunning {
		a.mu.Unlock()
		return fmt.Errorf("应用未运行")
	}
	a.mu.Unlock()

	logger := utils.GetLogger()
	logger.Info("========== NAM 停止 ==========")

	// 触发关闭
	a.cancel()

	return nil
}

// Shutdown 优雅关闭
func (a *App) Shutdown() {
	logger := utils.GetLogger()
	logger.Info("开始优雅关闭...")

	// 1. 停止监控
	a.coordinator.Stop()

	// 2. 等待后台协程结束
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("后台协程已停止")
	case <-time.After(10 * time.Second):
		logger.Warn("等待后台协程超时")
	}

	// 3. 记录最终会话数据
	a.recordFinalSessions()

	// 4. 关闭 Enforcer（保留 iptables 规则）
	a.enforcer.Shutdown()

	// 5. 关闭数据库
	if err := a.db.Close(); err != nil {
		logger.Errorf("关闭数据库失败: %v", err)
	}

	a.mu.Lock()
	a.isRunning = false
	a.mu.Unlock()

	logger.Info("NAM 已关闭")
}

// Reload 热重载配置
func (a *App) Reload() error {
	logger := utils.GetLogger()
	logger.Info("========== 热重载配置 ==========")

	// 1. 重新加载配置
	newCfg, err := config.Load(a.configPath)
	if err != nil {
		return fmt.Errorf("加载新配置失败: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// 2. 重新配置监控协调器
	if err := a.coordinator.Reconfigure(newCfg); err != nil {
		return fmt.Errorf("重新配置监控器失败: %w", err)
	}

	// 3. 重建 port -> rule 映射
	newRuleMap := make(map[int]*config.Rule)
	for i := range newCfg.Rules {
		newRuleMap[newCfg.Rules[i].Port] = &newCfg.Rules[i]
	}

	// 4. 更新配置
	a.config = newCfg
	a.ruleMap = newRuleMap

	logger.Info("配置热重载完成")
	return nil
}

// handleOverlimit 处理端口超限回调
func (a *App) handleOverlimit(port, current, max int) {
	logger := utils.GetLogger()
	logger.Warnf("端口 %d 超限: 当前 %d IP，最大 %d IP", port, current, max)

	// 获取对应的规则
	a.mu.RLock()
	rule, exists := a.ruleMap[port]
	a.mu.RUnlock()

	if !exists {
		logger.Errorf("未找到端口 %d 的规则", port)
		return
	}

	// 获取端口追踪器
	tracker := a.coordinator.GetTracker(port)
	if tracker == nil {
		logger.Errorf("未找到端口 %d 的追踪器", port)
		return
	}

	// 执行策略
	a.enforcer.Enforce(port, tracker, rule)
}

// statisticsWorker 统计数据后台协程
func (a *App) statisticsWorker() {
	defer a.wg.Done()

	logger := utils.GetLogger()
	logger.Info("启动统计协程")

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			logger.Info("统计协程退出")
			return
		case <-ticker.C:
			a.collectStatistics()
		}
	}
}

// collectStatistics 收集统计数据
func (a *App) collectStatistics() {
	logger := utils.GetLogger()

	a.mu.RLock()
	rules := a.config.Rules
	a.mu.RUnlock()

	for _, rule := range rules {
		tracker := a.coordinator.GetTracker(rule.Port)
		if tracker == nil {
			continue
		}

		stats := &storage.PortStatistics{
			Hour:        time.Now().Truncate(time.Hour),
			UniqueIPs:   tracker.Count(),
			TotalBans:   len(a.enforcer.GetActiveBans()),
			AvgSessions: 0, // 简化处理
			MaxSessions: tracker.Count(),
		}

		if err := a.db.RecordStatistics(rule.Port, stats); err != nil {
			logger.Errorf("记录统计数据失败 (端口 %d): %v", rule.Port, err)
		}
	}
}

// cleanupWorker 数据库清理后台协程
func (a *App) cleanupWorker() {
	defer a.wg.Done()

	logger := utils.GetLogger()
	logger.Info("启动清理协程")

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			logger.Info("清理协程退出")
			return
		case <-ticker.C:
			a.mu.RLock()
			daysToKeep := a.config.Global.HistoryDays
			a.mu.RUnlock()

			if daysToKeep > 0 {
				if err := a.db.Cleanup(daysToKeep); err != nil {
					logger.Errorf("清理数据库失败: %v", err)
				}
			}
		}
	}
}

// recordFinalSessions 记录最终会话数据
func (a *App) recordFinalSessions() {
	logger := utils.GetLogger()
	logger.Info("记录最终会话数据...")

	a.mu.RLock()
	rules := a.config.Rules
	a.mu.RUnlock()

	for _, rule := range rules {
		tracker := a.coordinator.GetTracker(rule.Port)
		if tracker == nil {
			continue
		}

		sessions := tracker.GetActiveSessions()
		for _, session := range sessions {
			if err := a.db.RecordSession(session); err != nil {
				logger.Errorf("记录会话失败: %v", err)
			}
		}
	}
}

// setupSignalHandler 设置信号处理器
func (a *App) setupSignalHandler() {
	logger := utils.GetLogger()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()

		for {
			select {
			case <-a.ctx.Done():
				return
			case sig := <-sigChan:
				switch sig {
				case syscall.SIGTERM, syscall.SIGINT:
					logger.Infof("收到信号 %s，开始关闭...", sig)
					a.cancel()
					a.Shutdown()
					os.Exit(0)
				case syscall.SIGHUP:
					logger.Infof("收到信号 %s，重载配置...", sig)
					if err := a.Reload(); err != nil {
						logger.Errorf("重载配置失败: %v", err)
					}
				}
			}
		}
	}()
}

// GetStatus 获取运行状态
func (a *App) GetStatus() *Status {
	a.mu.RLock()
	defer a.mu.RUnlock()

	status := &Status{
		IsRunning: a.isRunning,
		StartTime: a.startTime,
		Uptime:    time.Since(a.startTime),
		Ports:     make([]PortStatus, 0),
	}

	if !a.isRunning {
		return status
	}

	for _, rule := range a.config.Rules {
		tracker := a.coordinator.GetTracker(rule.Port)
		if tracker == nil {
			continue
		}

		portStatus := PortStatus{
			Port:       rule.Port,
			Protocol:   rule.Protocol,
			MaxIPs:     rule.MaxIPs,
			CurrentIPs: tracker.Count(),
		}

		status.Ports = append(status.Ports, portStatus)
	}

	return status
}

// Status 运行状态
type Status struct {
	IsRunning bool
	StartTime time.Time
	Uptime    time.Duration
	Ports     []PortStatus
}

// PortStatus 端口状态
type PortStatus struct {
	Port       int
	Protocol   string
	MaxIPs     int
	CurrentIPs int
}
