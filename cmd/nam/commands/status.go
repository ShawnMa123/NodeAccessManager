package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看运行状态",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("📊 NAM 状态信息")
		fmt.Println("此功能将在后续 Phase 实现")
		// TODO: Phase 3-7 将实现状态查询功能
	},
}
