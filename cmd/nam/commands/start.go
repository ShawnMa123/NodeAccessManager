package commands

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/nodeaccessmanager/nam/internal/core"
	"github.com/nodeaccessmanager/nam/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	daemon  bool
	pidFile string
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "启动守护进程",
	Long:  `启动 NAM 守护进程，开始监控代理端口的连接状态`,
	Run:   runStart,
}

func runStart(cmd *cobra.Command, args []string) {
	// 检查是否已在运行
	if running, pid, _ := core.CheckDaemonStatus(pidFile); running {
		fmt.Printf("❌ NAM 已在运行 (PID: %d)\n", pid)
		os.Exit(1)
	}

	if daemon {
		// 后台守护模式
		startDaemon()
	} else {
		// 前台运行模式
		startForeground()
	}
}

// startDaemon 以守护进程模式启动
func startDaemon() {
	fmt.Println("🚀 启动 NAM 守护进程...")

	// 获取当前可执行文件路径
	executable, err := os.Executable()
	if err != nil {
		fmt.Printf("❌ 获取可执行文件路径失败: %v\n", err)
		os.Exit(1)
	}

	// 构建启动参数（去掉 --daemon，添加 --foreground 内部标志）
	args := []string{"start", "--config", cfgFile, "--pid-file", pidFile}
	if debug {
		args = append(args, "--debug")
	}

	// 创建子进程
	cmd := exec.Command(executable, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // 创建新会话
	}

	if err := cmd.Start(); err != nil {
		fmt.Printf("❌ 启动守护进程失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ NAM 已启动 (PID: %d)\n", cmd.Process.Pid)
	fmt.Printf("   配置文件: %s\n", cfgFile)
	fmt.Printf("   PID 文件: %s\n", pidFile)
	fmt.Println("   使用 'nam status' 查看状态")
	fmt.Println("   使用 'nam stop' 停止服务")
}

// startForeground 前台运行模式
func startForeground() {
	// 初始化日志
	if err := initLogger(); err != nil {
		fmt.Printf("❌ 初始化日志失败: %v\n", err)
		os.Exit(1)
	}

	logger := utils.GetLogger()

	// 创建应用实例
	app, err := core.NewApp(cfgFile)
	if err != nil {
		logger.Fatalf("创建应用失败: %v", err)
	}

	// 写入 PID 文件
	if err := core.WritePIDFile(pidFile); err != nil {
		logger.Errorf("写入 PID 文件失败: %v", err)
	}
	defer core.RemovePIDFile(pidFile)

	// 启动应用
	if err := app.Start(); err != nil {
		logger.Fatalf("启动应用失败: %v", err)
	}

	// 阻塞等待
	select {}
}

// initLogger 初始化日志系统
func initLogger() error {
	logLevel := "info"
	if debug {
		logLevel = "debug"
	}

	return utils.InitLogger(
		"/var/log/nam/nam.log",
		logLevel,
		100, // maxSize MB
		7,   // maxBackups
		30,  // maxAge days
	)
}

func init() {
	startCmd.Flags().BoolVar(&daemon, "daemon", true, "以守护进程模式运行")
	startCmd.Flags().StringVar(&pidFile, "pid-file", core.DefaultPIDFile, "PID 文件路径")
}
