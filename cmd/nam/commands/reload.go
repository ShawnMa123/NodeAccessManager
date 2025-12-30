package commands

import (
	"fmt"
	"os"

	"github.com/nodeaccessmanager/nam/internal/core"
	"github.com/spf13/cobra"
)

var reloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "重载配置文件",
	Long:  `向运行中的 NAM 守护进程发送 SIGHUP 信号，重新加载配置文件`,
	Run:   runReload,
}

func runReload(cmd *cobra.Command, args []string) {
	reloadPidFile := pidFile
	if reloadPidFile == "" {
		reloadPidFile = core.DefaultPIDFile
	}

	// 检查是否在运行
	running, pid, err := core.CheckDaemonStatus(reloadPidFile)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}

	if !running {
		fmt.Println("❌ NAM 未运行，无法重载配置")
		os.Exit(1)
	}

	fmt.Printf("🔄 重载配置 (PID: %d)...\n", pid)

	// 发送 SIGHUP 信号
	if err := core.ReloadDaemon(reloadPidFile); err != nil {
		fmt.Printf("❌ 重载失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ 配置重载信号已发送")
	fmt.Println("💡 提示: 查看日志确认重载是否成功: tail -f /var/log/nam/nam.log")
}

func init() {
	rootCmd.AddCommand(reloadCmd)
}
