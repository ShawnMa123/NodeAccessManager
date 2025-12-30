package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// 样式定义
var (
	// 颜色
	primaryColor   = lipgloss.Color("#00D9FF")
	successColor   = lipgloss.Color("#00FF00")
	warningColor   = lipgloss.Color("#FFFF00")
	dangerColor    = lipgloss.Color("#FF0000")
	mutedColor     = lipgloss.Color("#666666")
	backgroundColor = lipgloss.Color("#1A1A1A")

	// 标题样式
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Padding(0, 1)

	// 标签页样式
	tabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(mutedColor)

	activeTabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(primaryColor).
			Bold(true).
			Underline(true)

	// 表格样式
	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(primaryColor).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true).
				Padding(0, 1)

	tableCellStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// 状态样式
	statusOKStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	statusWarningStyle = lipgloss.NewStyle().
				Foreground(warningColor).
				Bold(true)

	statusDangerStyle = lipgloss.NewStyle().
				Foreground(dangerColor).
				Bold(true)

	// 面板样式
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2).
			Margin(1, 0)

	// 帮助文本样式
	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Padding(0, 1)

	// 通用样式
	mutedStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	dangerStyle = lipgloss.NewStyle().
			Foreground(dangerColor)
)

// renderHeader 渲染标题栏
func (m Model) renderHeader() string {
	title := titleStyle.Render("NAM - Node Access Manager")

	status := m.app.GetStatus()
	uptime := formatDuration(status.Uptime)

	statusText := fmt.Sprintf("运行时间: %s", uptime)
	if !status.IsRunning {
		statusText = "状态: 未运行"
	}

	statusStyle := lipgloss.NewStyle().
		Foreground(mutedColor).
		Align(lipgloss.Right)

	// 计算填充
	width := m.width
	if width == 0 {
		width = 80
	}

	titleWidth := lipgloss.Width(title)
	statusWidth := lipgloss.Width(statusText)
	padding := width - titleWidth - statusWidth - 2

	if padding < 0 {
		padding = 0
	}

	paddingStr := strings.Repeat(" ", padding)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		title,
		paddingStr,
		statusStyle.Render(statusText),
	)
}

// renderTabs 渲染标签页
func (m Model) renderTabs() string {
	tabs := []string{"概览", "端口统计", "封禁列表", "帮助"}
	renderedTabs := make([]string, len(tabs))

	for i, tab := range tabs {
		if i == m.activeTab {
			renderedTabs[i] = activeTabStyle.Render(fmt.Sprintf("[%d] %s", i+1, tab))
		} else {
			renderedTabs[i] = tabStyle.Render(fmt.Sprintf("[%d] %s", i+1, tab))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
}

// renderOverview 渲染概览
func (m Model) renderOverview() string {
	var sections []string

	// 系统统计
	statsPanel := m.renderSystemStats()
	sections = append(sections, statsPanel)

	// 端口摘要
	portPanel := m.renderPortSummary()
	sections = append(sections, portPanel)

	// 最近封禁
	banPanel := m.renderRecentBans(5)
	sections = append(sections, banPanel)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderSystemStats 渲染系统统计
func (m Model) renderSystemStats() string {
	content := fmt.Sprintf(
		"监控端口: %d  │  活跃连接: %d  │  封禁数量: %d",
		m.systemStats.TotalPorts,
		m.systemStats.TotalSessions,
		m.systemStats.TotalBans,
	)

	return panelStyle.Render(titleStyle.Render("系统统计") + "\n\n" + content)
}

// renderPortSummary 渲染端口摘要
func (m Model) renderPortSummary() string {
	if len(m.portStats) == 0 {
		return panelStyle.Render(titleStyle.Render("端口摘要") + "\n\n" + mutedStyle.Render("无监控端口"))
	}

	// 排序端口
	ports := make([]int, 0, len(m.portStats))
	for port := range m.portStats {
		ports = append(ports, port)
	}
	sort.Ints(ports)

	var lines []string
	for _, port := range ports {
		stat := m.portStats[port]
		statusStr := m.formatStatus(stat.Status)

		line := fmt.Sprintf(
			"端口 %-5d  │  %s  │  连接数: %d/%d  │  %s",
			stat.Port,
			stat.Protocol,
			stat.CurrentIPs,
			stat.MaxIPs,
			statusStr,
		)
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")
	return panelStyle.Render(titleStyle.Render("端口摘要") + "\n\n" + content)
}

// renderPortStats 渲染端口统计详情
func (m Model) renderPortStats() string {
	if len(m.portStats) == 0 {
		return panelStyle.Render("无监控端口")
	}

	// 表头
	header := fmt.Sprintf("%-8s %-10s %-12s %-10s %-10s",
		"端口", "协议", "当前/最大", "使用率", "状态")

	headerLine := tableHeaderStyle.Render(header)

	// 排序端口
	ports := make([]int, 0, len(m.portStats))
	for port := range m.portStats {
		ports = append(ports, port)
	}
	sort.Ints(ports)

	// 表格行
	var rows []string
	for _, port := range ports {
		stat := m.portStats[port]

		usage := 0.0
		if stat.MaxIPs > 0 {
			usage = float64(stat.CurrentIPs) / float64(stat.MaxIPs) * 100
		}

		statusStr := m.formatStatus(stat.Status)

		row := fmt.Sprintf("%-8d %-10s %-12s %-10s %s",
			stat.Port,
			stat.Protocol,
			fmt.Sprintf("%d/%d", stat.CurrentIPs, stat.MaxIPs),
			fmt.Sprintf("%.1f%%", usage),
			statusStr,
		)

		rows = append(rows, tableCellStyle.Render(row))
	}

	content := strings.Join(rows, "\n")

	return panelStyle.Render(
		titleStyle.Render("端口统计详情") + "\n\n" +
		headerLine + "\n" +
		content,
	)
}

// renderBanList 渲染封禁列表
func (m Model) renderBanList() string {
	if len(m.banRecords) == 0 {
		return panelStyle.Render(titleStyle.Render("封禁列表") + "\n\n" + mutedStyle.Render("当前无封禁"))
	}

	// 表头
	header := fmt.Sprintf("%-18s %-8s %-12s %-20s %-10s",
		"IP 地址", "端口", "剩余时间", "封禁时间", "原因")

	headerLine := tableHeaderStyle.Render(header)

	// 表格行
	var rows []string
	for _, ban := range m.banRecords {
		bannedTime := ban.BannedAt.Format("01-02 15:04:05")
		remaining := formatDuration(ban.Remaining)

		row := fmt.Sprintf("%-18s %-8d %-12s %-20s %-10s",
			ban.IP,
			ban.Port,
			remaining,
			bannedTime,
			ban.Reason,
		)

		rows = append(rows, tableCellStyle.Render(row))
	}

	content := strings.Join(rows, "\n")

	return panelStyle.Render(
		titleStyle.Render(fmt.Sprintf("封禁列表 (%d)", len(m.banRecords))) + "\n\n" +
		headerLine + "\n" +
		content,
	)
}

// renderRecentBans 渲染最近封禁
func (m Model) renderRecentBans(limit int) string {
	if len(m.banRecords) == 0 {
		return panelStyle.Render(titleStyle.Render("最近封禁") + "\n\n" + mutedStyle.Render("暂无封禁记录"))
	}

	count := len(m.banRecords)
	if count > limit {
		count = limit
	}

	var lines []string
	for i := 0; i < count; i++ {
		ban := m.banRecords[i]
		remaining := formatDuration(ban.Remaining)

		line := fmt.Sprintf(
			"%s:%d  │  剩余: %s  │  原因: %s",
			ban.IP,
			ban.Port,
			remaining,
			ban.Reason,
		)
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")
	return panelStyle.Render(titleStyle.Render("最近封禁") + "\n\n" + content)
}

// renderHelp 渲染帮助
func (m Model) renderHelp() string {
	help := `
快捷键说明：

  Tab / →         切换到下一个标签页
  Shift+Tab / ←   切换到上一个标签页
  1/2/3/4         直接跳转到对应标签页

  r               手动刷新数据
  Space           暂停/恢复自动刷新

  q / Ctrl+C      退出

标签页说明：

  [1] 概览        系统整体状态和摘要信息
  [2] 端口统计    详细的端口监控数据
  [3] 封禁列表    当前所有封禁记录
  [4] 帮助        快捷键和使用说明

状态指示：

  ✓ OK           连接数正常
  ⚠ WARNING      连接数接近上限（≥80%）
  ✗ OVERLIMIT    连接数超限

自动刷新：

  当前状态: %s
  刷新间隔: %v

最后更新: %s
`

	autoRefreshStatus := "开启"
	if !m.autoRefresh {
		autoRefreshStatus = "关闭"
	}

	content := fmt.Sprintf(
		help,
		autoRefreshStatus,
		m.refreshTick,
		m.lastUpdate.Format("15:04:05"),
	)

	return panelStyle.Render(titleStyle.Render("帮助") + content)
}

// renderFooter 渲染状态栏
func (m Model) renderFooter() string {
	var parts []string

	// 自动刷新状态
	refreshStatus := "自动刷新: 开启"
	if !m.autoRefresh {
		refreshStatus = "自动刷新: 关闭"
	}
	parts = append(parts, refreshStatus)

	// 最后更新时间
	updateTime := fmt.Sprintf("更新: %s", m.lastUpdate.Format("15:04:05"))
	parts = append(parts, updateTime)

	// 错误信息
	if m.err != nil {
		parts = append(parts, dangerStyle.Render(fmt.Sprintf("错误: %v", m.err)))
	}

	// 帮助提示
	parts = append(parts, "按 [4] 查看帮助")

	footer := helpStyle.Render(strings.Join(parts, " │ "))

	// 分隔线
	separator := strings.Repeat("─", m.width)
	if m.width == 0 {
		separator = strings.Repeat("─", 80)
	}

	return "\n" + mutedStyle.Render(separator) + "\n" + footer
}

// formatStatus 格式化状态
func (m Model) formatStatus(status string) string {
	switch status {
	case "OK":
		return statusOKStyle.Render("✓ OK")
	case "WARNING":
		return statusWarningStyle.Render("⚠ WARNING")
	case "OVERLIMIT":
		return statusDangerStyle.Render("✗ OVERLIMIT")
	default:
		return status
	}
}
