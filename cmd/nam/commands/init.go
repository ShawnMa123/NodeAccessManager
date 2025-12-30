package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nodeaccessmanager/nam/internal/config"
	"github.com/nodeaccessmanager/nam/internal/discovery"
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
	Run: runInitWizard,
}

func runInitWizard(cmd *cobra.Command, args []string) {
	reader := bufio.NewReader(os.Stdin)

	// 打印欢迎信息
	printWelcome()

	// 扫描系统进程
	fmt.Println("\n🔍 正在扫描系统环境...")
	scanner := discovery.NewScanner()
	result, err := scanner.ScanProcesses()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 扫描失败: %v\n", err)
		os.Exit(1)
	}

	if result.Total == 0 {
		fmt.Println("⚠️  未检测到 Xray 或 Sing-box 进程")
		fmt.Println("   请确保代理程序正在运行")
		os.Exit(1)
	}

	// 显示发现的进程
	fmt.Printf("\n✅ 发现 %d 个代理进程:\n\n", result.Total)
	for i, proc := range result.Processes {
		fmt.Printf("[%d] %s (PID: %d)\n", i+1, proc.Name, proc.PID)
		if proc.ConfigPath != "" {
			fmt.Printf("    配置文件: %s\n", proc.ConfigPath)
			if len(proc.Inbounds) > 0 {
				fmt.Printf("    监听端口: ")
				for j, inbound := range proc.Inbounds {
					if j > 0 {
						fmt.Print(", ")
					}
					fmt.Printf("%d (%s)", inbound.Port, inbound.Protocol)
				}
				fmt.Println()
			}
		} else {
			fmt.Println("    ⚠️  未找到配置文件")
		}
		fmt.Println()
	}

	// 收集所有端口
	var allInbounds []discovery.Inbound
	for _, proc := range result.Processes {
		allInbounds = append(allInbounds, proc.Inbounds...)
	}

	if len(allInbounds) == 0 {
		fmt.Println("❌ 未发现任何监听端口")
		fmt.Println("   请检查配置文件是否正确")
		os.Exit(1)
	}

	// 创建配置
	cfg := config.DefaultConfig()
	cfg.Rules = []config.Rule{}

	// 为每个端口配置规则
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📋 请配置每个端口的访问限制:")
	fmt.Println()

	for _, inbound := range allInbounds {
		fmt.Printf("端口 %d (%s - %s)\n", inbound.Port, inbound.Protocol, inbound.Tag)

		// 输入最大IP数
		maxIPs := promptInt(reader, "  最大并发IP数 [5]: ", 5)

		// 选择策略
		fmt.Println("  驱逐策略:")
		fmt.Println("    1) FIFO - 新用户挤掉旧用户（推荐）")
		fmt.Println("    2) LIFO - 拒绝新用户连接")
		strategy := promptChoice(reader, "  选择 [1]: ", 1, 2)

		var strategyName config.Strategy
		if strategy == 1 {
			strategyName = config.StrategyFIFO
		} else {
			strategyName = config.StrategyLIFO
		}

		// 封禁时长
		banDuration := promptInt(reader, "  封禁时长（秒，0表示不封禁）[60]: ", 60)

		// 添加规则
		cfg.Rules = append(cfg.Rules, config.Rule{
			Port:        inbound.Port,
			Protocol:    inbound.Protocol,
			MaxIPs:      maxIPs,
			Tag:         inbound.Tag,
			Strategy:    strategyName,
			BanDuration: banDuration,
			Whitelist:   []string{},
			Blacklist:   []string{},
		})

		fmt.Println()
	}

	// 保存配置
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("💾 保存配置...")

	// 确保配置目录存在
	configDir := filepath.Dir(cfgFile)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "❌ 创建配置目录失败: %v\n", err)
		os.Exit(1)
	}

	// 保存配置文件
	if err := config.Save(cfg, cfgFile); err != nil {
		fmt.Fprintf(os.Stderr, "❌ 保存配置失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 配置已保存到: %s\n", cfgFile)

	// 打印后续步骤
	printNextSteps()
}

// printWelcome 打印欢迎信息
func printWelcome() {
	fmt.Println("\n┌─────────────────────────────────────────────────────────────┐")
	fmt.Println("│                                                             │")
	fmt.Println("│   ███╗   ██╗ █████╗ ███╗   ███╗                            │")
	fmt.Println("│   ████╗  ██║██╔══██╗████╗ ████║                            │")
	fmt.Println("│   ██╔██╗ ██║███████║██╔████╔██║                            │")
	fmt.Println("│   ██║╚██╗██║██╔══██║██║╚██╔╝██║                            │")
	fmt.Println("│   ██║ ╚████║██║  ██║██║ ╚═╝ ██║                            │")
	fmt.Println("│   ╚═╝  ╚═══╝╚═╝  ╚═╝╚═╝     ╚═╝                            │")
	fmt.Println("│                                                             │")
	fmt.Println("│   NodeAccessManager v" + Version + "                                 │")
	fmt.Println("│   代理节点访问控制工具                                      │")
	fmt.Println("│                                                             │")
	fmt.Println("└─────────────────────────────────────────────────────────────┘")
}

// printNextSteps 打印后续步骤
func printNextSteps() {
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🚀 下一步操作:")
	fmt.Println()
	fmt.Println("  1. 启动守护进程:")
	fmt.Println("     sudo nam start --daemon")
	fmt.Println()
	fmt.Println("  2. 查看实时监控:")
	fmt.Println("     sudo nam monitor")
	fmt.Println()
	fmt.Println("  3. 安装为系统服务（开机自启）:")
	fmt.Println("     sudo nam install")
	fmt.Println("     sudo systemctl enable nam")
	fmt.Println("     sudo systemctl start nam")
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// promptInt 提示输入整数
func promptInt(reader *bufio.Reader, prompt string, defaultValue int) int {
	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(input)
	if err != nil || value <= 0 {
		return defaultValue
	}

	return value
}

// promptChoice 提示选择（1或2）
func promptChoice(reader *bufio.Reader, prompt string, defaultValue, max int) int {
	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(input)
	if err != nil || value < 1 || value > max {
		return defaultValue
	}

	return value
}
