package commands

import (
	"fmt"
	"os"

	"github.com/nodeaccessmanager/nam/internal/core"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看运行状态",
	Run:   runStatus,
}

func runStatus(cmd *cobra.Command, args []string) {
	statusPidFile := pidFile
	if statusPidFile == "" {
		statusPidFile = core.DefaultPIDFile
	}

	fmt.Println("📊 NAM 状态信息")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// 检查运行状态
	running, pid, err := core.CheckDaemonStatus(statusPidFile)
	if err != nil {
		fmt.Printf("状态: ❌ 未运行\n")
		fmt.Printf("详情: %v\n", err)
		os.Exit(1)
	}

	if !running {
		fmt.Println("状态: ❌ 未运行")
		os.Exit(1)
	}

	fmt.Printf("状态: ✅ 运行中\n")
	fmt.Printf("PID:  %d\n", pid)
	fmt.Printf("配置: %s\n", cfgFile)

	// TODO: 可以通过 Unix Socket 或 HTTP API 获取更详细的运行时信息
	// 例如：监控端口列表、当前连接数、封禁列表等
	// 这将在 Phase 8 (API/管理接口) 中实现

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("💡 提示: 查看详细日志: tail -f /var/log/nam/nam.log")
}
