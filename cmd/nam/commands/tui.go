package commands

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nodeaccessmanager/nam/internal/config"
	"github.com/nodeaccessmanager/nam/internal/core"
	"github.com/nodeaccessmanager/nam/internal/tui"
	"github.com/nodeaccessmanager/nam/pkg/utils"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "启动 TUI 监控界面",
	Long:  `启动基于终端的交互式监控界面，实时查看端口状态和封禁信息`,
	Run:   runTUI,
}

func runTUI(cmd *cobra.Command, args []string) {
	// 初始化日志（静默模式，避免干扰 TUI）
	if err := utils.InitLogger("/var/log/nam/tui.log", "error", 10, 3, 7); err != nil {
		fmt.Printf("❌ 初始化日志失败: %v\n", err)
		os.Exit(1)
	}

	// 加载配置
	cfg, err := config.Load(cfgFile)
	if err != nil {
		fmt.Printf("❌ 加载配置失败: %v\n", err)
		fmt.Printf("   配置文件: %s\n", cfgFile)
		os.Exit(1)
	}

	// 检查守护进程是否运行
	tuiPidFile := pidFile
	if tuiPidFile == "" {
		tuiPidFile = core.DefaultPIDFile
	}

	running, pid, _ := core.CheckDaemonStatus(tuiPidFile)
	if !running {
		fmt.Println("❌ NAM 守护进程未运行")
		fmt.Println("   请先启动服务: nam start")
		os.Exit(1)
	}

	fmt.Printf("🔗 连接到 NAM 守护进程 (PID: %d)\n", pid)
	fmt.Println("⏳ 加载监控界面...")

	// 创建应用实例（只读模式，用于获取状态）
	app, err := core.NewApp(cfgFile)
	if err != nil {
		fmt.Printf("❌ 创建应用实例失败: %v\n", err)
		os.Exit(1)
	}

	// 创建 TUI 模型
	model := tui.NewModel(app, cfg)

	// 启动 TUI
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),       // 使用备用屏幕
		tea.WithMouseCellMotion(), // 支持鼠标
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("❌ TUI 运行错误: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
