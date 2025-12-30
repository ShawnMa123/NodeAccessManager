package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "初始化配置向导（交互式）",
	Long: `初始化配置向导会执行以下操作:
1. 扫描系统中的代理进程（Xray/Sing-box）
2. 解析配置文件，提取监听端口
3. 交互式配置每个端口的访问限制
4. 生成配置文件到 /etc/nam/config.yaml
`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🔍 初始化向导")
		fmt.Println("此功能将在 Phase 2 实现")
		// TODO: Phase 2 将实现完整的初始化向导
	},
}
