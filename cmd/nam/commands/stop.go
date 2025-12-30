package commands

import (
	"fmt"
	"os"
	"time"

	"github.com/nodeaccessmanager/nam/internal/core"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "停止守护进程",
	Run:   runStop,
}

func runStop(cmd *cobra.Command, args []string) {
	stopPidFile := pidFile
	if stopPidFile == "" {
		stopPidFile = core.DefaultPIDFile
	}

	// 检查是否在运行
	running, pid, err := core.CheckDaemonStatus(stopPidFile)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		// 清理可能存在的孤立 PID 文件
		core.RemovePIDFile(stopPidFile)
		os.Exit(1)
	}

	if !running {
		fmt.Println("⚠️  NAM 未运行")
		os.Exit(1)
	}

	fmt.Printf("🛑 停止 NAM 守护进程 (PID: %d)...\n", pid)

	// 发送停止信号
	if err := core.StopDaemon(stopPidFile); err != nil {
		fmt.Printf("❌ 停止失败: %v\n", err)
		os.Exit(1)
	}

	// 等待进程退出
	fmt.Print("   等待进程退出")
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		if !core.IsProcessRunning(pid) {
			break
		}
		fmt.Print(".")
	}
	fmt.Println()

	// 确认进程已停止
	if core.IsProcessRunning(pid) {
		fmt.Println("⚠️  进程未在预期时间内停止，可能需要强制终止")
		os.Exit(1)
	}

	// 清理 PID 文件
	core.RemovePIDFile(stopPidFile)

	fmt.Println("✅ NAM 已停止")
}
