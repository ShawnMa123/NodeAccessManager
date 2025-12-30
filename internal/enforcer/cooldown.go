package enforcer

import (
	"fmt"
	"sync"
	"time"

	"github.com/nodeaccessmanager/nam/pkg/utils"
)

// CooldownManager 冷却管理器
type CooldownManager struct {
	records  map[string]*cooldownRecord // key: "IP:PORT"
	mu       sync.Mutex
	executor *Executor // 循环依赖，延迟设置
}

// cooldownRecord 内部冷却记录
type cooldownRecord struct {
	IP       string
	Port     int
	ExpireAt time.Time
	Timer    *time.Timer
}

// NewCooldownManager 创建冷却管理器
func NewCooldownManager() *CooldownManager {
	return &CooldownManager{
		records: make(map[string]*cooldownRecord),
	}
}

// SetExecutor 设置执行器（解决循环依赖）
func (cm *CooldownManager) SetExecutor(executor *Executor) {
	cm.executor = executor
}

// Schedule 安排定时解封
func (cm *CooldownManager) Schedule(ip string, port int, duration int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	logger := utils.GetLogger()
	key := fmt.Sprintf("%s:%d", ip, port)
	expireAt := time.Now().Add(time.Duration(duration) * time.Second)

	// 如果已存在，取消旧的定时器
	if old, exists := cm.records[key]; exists {
		old.Timer.Stop()
		logger.Debugf("取消旧的定时器: %s", key)
	}

	// 创建新的定时器
	timer := time.AfterFunc(time.Duration(duration)*time.Second, func() {
		cm.unban(ip, port)
	})

	cm.records[key] = &cooldownRecord{
		IP:       ip,
		Port:     port,
		ExpireAt: expireAt,
		Timer:    timer,
	}

	logger.Debugf("安排定时解封: %s（%ds 后）", key, duration)
}

// unban 定时器回调，执行解封
func (cm *CooldownManager) unban(ip string, port int) {
	logger := utils.GetLogger()

	// 执行解封
	if cm.executor != nil {
		if err := cm.executor.RemoveBan(ip, port); err != nil {
			logger.Errorf("定时解封失败 %s:%d - %v", ip, port, err)
			// 不删除记录，允许手动重试
			return
		}
	}

	// 从记录中移除
	cm.mu.Lock()
	key := fmt.Sprintf("%s:%d", ip, port)
	delete(cm.records, key)
	cm.mu.Unlock()

	logger.Infof("定时解封成功: %s:%d", ip, port)
}

// Cancel 取消指定的封禁（立即解封）
func (cm *CooldownManager) Cancel(ip string, port int) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	key := fmt.Sprintf("%s:%d", ip, port)
	record, exists := cm.records[key]
	if !exists {
		return fmt.Errorf("未找到封禁记录: %s", key)
	}

	// 停止定时器
	record.Timer.Stop()

	// 执行解封
	if cm.executor != nil {
		if err := cm.executor.RemoveBan(ip, port); err != nil {
			return fmt.Errorf("解封失败: %w", err)
		}
	}

	// 删除记录
	delete(cm.records, key)

	utils.GetLogger().Infof("手动解封成功: %s", key)
	return nil
}

// GetActiveRecords 获取所有活跃的封禁记录
func (cm *CooldownManager) GetActiveRecords() []BanRecord {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	records := make([]BanRecord, 0, len(cm.records))
	now := time.Now()

	for _, record := range cm.records {
		duration := int(record.ExpireAt.Sub(now).Seconds())
		if duration < 0 {
			duration = 0
		}

		records = append(records, BanRecord{
			IP:       record.IP,
			Port:     record.Port,
			BannedAt: record.ExpireAt.Add(-time.Duration(duration) * time.Second),
			ExpireAt: record.ExpireAt,
			Duration: duration,
			Reason:   "Overlimit",
			Strategy: "AUTO",
		})
	}

	return records
}

// Count 获取当前封禁数量
func (cm *CooldownManager) Count() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return len(cm.records)
}

// Clear 清空所有定时器（程序退出时调用）
func (cm *CooldownManager) Clear() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	logger := utils.GetLogger()
	logger.Info("清空冷却管理器")

	for key, record := range cm.records {
		record.Timer.Stop()
		delete(cm.records, key)
	}
}

// IsActive 检查指定 IP 是否正在被封禁
func (cm *CooldownManager) IsActive(ip string, port int) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	key := fmt.Sprintf("%s:%d", ip, port)
	_, exists := cm.records[key]
	return exists
}

// GetExpireTime 获取封禁过期时间
func (cm *CooldownManager) GetExpireTime(ip string, port int) (time.Time, bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	key := fmt.Sprintf("%s:%d", ip, port)
	record, exists := cm.records[key]
	if !exists {
		return time.Time{}, false
	}

	return record.ExpireAt, true
}
