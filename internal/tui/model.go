package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nodeaccessmanager/nam/internal/config"
	"github.com/nodeaccessmanager/nam/internal/core"
)

// Model TUI 模型
type Model struct {
	app          *core.App
	config       *config.Config
	width        int
	height       int
	activeTab    int
	lastUpdate   time.Time
	err          error
	quitting     bool
	autoRefresh  bool
	refreshTick  time.Duration
	portStats    map[int]PortStat
	banRecords   []BanRecord
	systemStats  SystemStats
}

// PortStat 端口统计信息
type PortStat struct {
	Port       int
	Protocol   string
	MaxIPs     int
	CurrentIPs int
	Status     string
}

// BanRecord 封禁记录（用于展示）
type BanRecord struct {
	IP         string
	Port       int
	BannedAt   time.Time
	ExpireAt   time.Time
	Remaining  time.Duration
	Reason     string
}

// SystemStats 系统统计
type SystemStats struct {
	Uptime       time.Duration
	TotalPorts   int
	TotalBans    int
	TotalSessions int
}

// NewModel 创建 TUI 模型
func NewModel(app *core.App, cfg *config.Config) Model {
	return Model{
		app:         app,
		config:      cfg,
		activeTab:   0,
		autoRefresh: true,
		refreshTick: 2 * time.Second,
		portStats:   make(map[int]PortStat),
		lastUpdate:  time.Now(),
	}
}

// Init 初始化
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(m.refreshTick),
		m.fetchData,
	)
}

// Update 更新
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		if !m.quitting && m.autoRefresh {
			return m, tea.Batch(
				tickCmd(m.refreshTick),
				m.fetchData,
			)
		}
		return m, nil

	case dataMsg:
		m.portStats = msg.portStats
		m.banRecords = msg.banRecords
		m.systemStats = msg.systemStats
		m.lastUpdate = time.Now()
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil
	}

	return m, nil
}

// View 渲染
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	// 标题栏
	header := m.renderHeader()

	// 标签页
	tabs := m.renderTabs()

	// 内容区
	var content string
	switch m.activeTab {
	case 0:
		content = m.renderOverview()
	case 1:
		content = m.renderPortStats()
	case 2:
		content = m.renderBanList()
	case 3:
		content = m.renderHelp()
	}

	// 状态栏
	footer := m.renderFooter()

	// 组合
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		tabs,
		content,
		footer,
	)
}

// handleKeyPress 处理按键
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "tab", "right":
		m.activeTab = (m.activeTab + 1) % 4
		return m, nil

	case "shift+tab", "left":
		m.activeTab = (m.activeTab - 1 + 4) % 4
		return m, nil

	case "1":
		m.activeTab = 0
		return m, nil
	case "2":
		m.activeTab = 1
		return m, nil
	case "3":
		m.activeTab = 2
		return m, nil
	case "4":
		m.activeTab = 3
		return m, nil

	case "r":
		return m, m.fetchData

	case "space":
		m.autoRefresh = !m.autoRefresh
		return m, nil
	}

	return m, nil
}

// fetchData 获取数据
func (m Model) fetchData() tea.Msg {
	// 获取状态
	status := m.app.GetStatus()

	// 构建端口统计
	portStats := make(map[int]PortStat)
	for _, ps := range status.Ports {
		stat := PortStat{
			Port:       ps.Port,
			Protocol:   ps.Protocol,
			MaxIPs:     ps.MaxIPs,
			CurrentIPs: ps.CurrentIPs,
		}

		// 计算状态
		if ps.CurrentIPs > ps.MaxIPs {
			stat.Status = "OVERLIMIT"
		} else if ps.CurrentIPs >= ps.MaxIPs*8/10 {
			stat.Status = "WARNING"
		} else {
			stat.Status = "OK"
		}

		portStats[ps.Port] = stat
	}

	// 获取封禁记录
	banRecords := []BanRecord{}
	activeBans := m.app.GetActiveBans()
	now := time.Now()

	for _, ban := range activeBans {
		remaining := ban.ExpireAt.Sub(now)
		if remaining < 0 {
			remaining = 0
		}

		banRecords = append(banRecords, BanRecord{
			IP:        ban.IP,
			Port:      ban.Port,
			BannedAt:  ban.BannedAt,
			ExpireAt:  ban.ExpireAt,
			Remaining: remaining,
			Reason:    ban.Reason,
		})
	}

	// 系统统计
	systemStats := SystemStats{
		Uptime:       status.Uptime,
		TotalPorts:   len(status.Ports),
		TotalBans:    len(banRecords),
		TotalSessions: 0, // 可以从各端口汇总
	}

	for _, ps := range status.Ports {
		systemStats.TotalSessions += ps.CurrentIPs
	}

	return dataMsg{
		portStats:   portStats,
		banRecords:  banRecords,
		systemStats: systemStats,
	}
}

// 消息类型
type tickMsg time.Time
type dataMsg struct {
	portStats   map[int]PortStat
	banRecords  []BanRecord
	systemStats SystemStats
}
type errMsg struct {
	err error
}

// tickCmd 定时器命令
func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// 格式化时长
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}
