package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "停止守护进程",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🛑 停止 NAM 守护进程")
		fmt.Println("此功能将在后续 Phase 实现")
		// TODO: Phase 7 将实现进程管理功能
	},
}
