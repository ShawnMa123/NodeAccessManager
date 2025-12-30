package commands

import (
	"fmt"
	"os"

	"github.com/nodeaccessmanager/nam/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "配置管理",
	Long:  `配置文件的查看、验证和编辑`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "显示当前配置",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ 加载配置失败: %v\n", err)
			os.Exit(1)
		}

		// 输出配置
		data, err := yaml.Marshal(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ 序列化配置失败: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("📄 当前配置:")
		fmt.Println(string(data))
	},
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "验证配置文件",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("🔍 验证配置文件: %s\n", cfgFile)

		cfg, err := config.Load(cfgFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ 配置验证失败: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✅ 配置验证通过")
		fmt.Printf("   - 监控端口数: %d\n", len(cfg.Rules))
		fmt.Printf("   - 检查周期: %d 秒\n", cfg.Global.CheckInterval)
		fmt.Printf("   - 默认策略: %s\n", cfg.Global.Strategy)
	},
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "编辑配置文件",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("📝 编辑配置文件")
		fmt.Printf("   请手动编辑: %s\n", cfgFile)
		fmt.Println("   编辑后运行 'nam config validate' 验证配置")
		// TODO: 可选择性地集成编辑器调用
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configValidateCmd)
	configCmd.AddCommand(configEditCmd)
}
