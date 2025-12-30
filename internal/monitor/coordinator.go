package monitor

import (
	"fmt"
	"sync"
	"time"

	"github.com/nodeaccessmanager/nam/internal/config"
	"github.com/nodeaccessmanager/nam/pkg/utils"
)

// Coordinator 监控协调器
type Coordinator struct {
	config    *config.Config
	collector *Collector
	trackers  map[int]*PortTracker // key: port number
	stopCh    chan struct{}
	wg        sync.WaitGroup
	mu        sync.RWMutex

	// 可选的回调函数
	onOverlimit func(port int, currentCount int, maxAllowed int)
}

// NewCoordinator 创建监控协调器
func NewCoordinator(cfg *config.Config) *Coordinator {
	return &Coordinator{
		config:    cfg,
		collector: NewCollector(),
		trackers:  make(map[int]*PortTracker),
		stopCh:    make(chan struct{}),
	}
}

// SetOverlimitCallback 设置超限回调函数
func (c *Coordinator) SetOverlimitCallback(callback func(port int, currentCount int, maxAllowed int)) {
	c.onOverlimit = callback
}

// Start 启动监控
func (c *Coordinator) Start() error {
	logger := utils.GetLogger()
	logger.Info("启动监控协调器")

	// 初始化每个端口的追踪器
	c.mu.Lock()
	for _, rule := range c.config.Rules {
		c.trackers[rule.Port] = NewPortTracker(rule.Port)
		logger.Infof("初始化端口 %d 的追踪器（最大 %d IP）", rule.Port, rule.MaxIPs)
	}
	c.mu.Unlock()

	// 为每个端口启动监控 goroutine
	for port := range c.trackers {
		c.wg.Add(1)
		go c.monitorPort(port)
	}

	logger.Infof("监控协调器已启动，监控 %d 个端口", len(c.trackers))
	return nil
}

// Stop 停止监控
func (c *Coordinator) Stop() {
	logger := utils.GetLogger()
	logger.Info("停止监控协调器")

	close(c.stopCh)
	c.wg.Wait()

	logger.Info("监控协调器已停止")
}

// monitorPort 监控单个端口
func (c *Coordinator) monitorPort(port int) {
	defer c.wg.Done()

	logger := utils.GetLogger()
	ticker := time.NewTicker(time.Duration(c.config.Global.CheckInterval) * time.Second)
	defer ticker.Stop()

	tracker := c.getTracker(port)
	if tracker == nil {
		logger.Errorf("端口 %d 的追踪器不存在", port)
		return
	}

	logger.Infof("开始监控端口 %d（检查周期: %ds）", port, c.config.Global.CheckInterval)

	for {
		select {
		case <-ticker.C:
			// 1. 采集连接
			connections, err := c.collector.CollectConnections(port)
			if err != nil {
				logger.Errorf("采集端口 %d 连接失败: %v", port, err)
				continue
			}

			// 2. 更新追踪器
			tracker.Update(connections)

			// 3. 检查是否超限
			rule := c.config.GetRuleByPort(port)
			if rule == nil {
				continue
			}

			currentCount := tracker.Count()
			if currentCount > rule.MaxIPs {
				logger.Warnf("端口 %d 超限: 当前 %d IP > 最大 %d IP",
					port, currentCount, rule.MaxIPs)

				// 触发超限回调
				if c.onOverlimit != nil {
					c.onOverlimit(port, currentCount, rule.MaxIPs)
				}
			} else {
				logger.Debugf("端口 %d 状态正常: %d/%d IP", port, currentCount, rule.MaxIPs)
			}

		case <-c.stopCh:
			logger.Infof("停止监控端口 %d", port)
			return
		}
	}
}

// GetTracker 获取指定端口的追踪器
func (c *Coordinator) GetTracker(port int) *PortTracker {
	return c.getTracker(port)
}

// getTracker 内部获取追踪器（线程安全）
func (c *Coordinator) getTracker(port int) *PortTracker {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.trackers[port]
}

// GetAllStats 获取所有端口的统计信息
func (c *Coordinator) GetAllStats() map[int]PortStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := make(map[int]PortStats)
	for port, tracker := range c.trackers {
		stats[port] = tracker.GetStats()
	}

	return stats
}

// GetPortStats 获取指定端口的统计信息
func (c *Coordinator) GetPortStats(port int) (*PortStats, error) {
	tracker := c.getTracker(port)
	if tracker == nil {
		return nil, fmt.Errorf("端口 %d 未监控", port)
	}

	stats := tracker.GetStats()
	return &stats, nil
}

// Reconfigure 重新配置（热重载支持）
func (c *Coordinator) Reconfigure(newConfig *config.Config) error {
	logger := utils.GetLogger()
	logger.Info("重新配置监控协调器")

	c.mu.Lock()
	defer c.mu.Unlock()

	// 更新配置
	c.config = newConfig

	// 重新初始化追踪器
	// 注意：这里简化处理，直接清空并重建
	// 生产环境可以保留现有会话数据
	oldTrackers := c.trackers
	c.trackers = make(map[int]*PortTracker)

	for _, rule := range newConfig.Rules {
		// 如果端口已存在，保留现有追踪器
		if tracker, exists := oldTrackers[rule.Port]; exists {
			c.trackers[rule.Port] = tracker
			logger.Infof("保留端口 %d 的追踪器", rule.Port)
		} else {
			c.trackers[rule.Port] = NewPortTracker(rule.Port)
			logger.Infof("新增端口 %d 的追踪器", rule.Port)
		}
	}

	logger.Info("监控协调器重新配置完成")
	return nil
}
