package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version 版本号，编译时注入
	Version = "dev"
	// BuildTime 编译时间,编译时注入
	BuildTime = "unknown"

	// 全局配置文件路径
	cfgFile string
	// 调试模式
	debug bool
)

// rootCmd 根命令
var rootCmd = &cobra.Command{
	Use:   "nam",
	Short: "NodeAccessManager - 代理节点访问控制工具",
	Long: `
███╗   ██╗ █████╗ ███╗   ███╗
████╗  ██║██╔══██╗████╗ ████║
██╔██╗ ██║███████║██╔████╔██║
██║╚██╗██║██╔══██║██║╚██╔╝██║
██║ ╚████║██║  ██║██║ ╚═╝ ██║
╚═╝  ╚═══╝╚═╝  ╚═╝╚═╝     ╚═╝

NodeAccessManager (NAM) - 代理节点访问控制工具

专为 Linux VPS 设计，通过内核层面的连接管理，为 Sing-box/Xray 
等代理工具提供基于端口的并发 IP 限制能力。

特性:
  • 单文件部署，零依赖
  • 自动发现代理进程和配置
  • 实时监控连接状态
  • 智能驱逐超限连接
  • TUI 可视化界面
`,
	Version: fmt.Sprintf("%s (Build: %s)", Version, BuildTime),
}

// Execute 执行根命令
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// 全局参数
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "/etc/nam/config.yaml", "配置文件路径")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "启用调试模式")

	// 添加子命令
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(configCmd)
}
