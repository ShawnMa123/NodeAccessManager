package core

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

const DefaultPIDFile = "/var/run/nam.pid"

// WritePIDFile 写入 PID 文件
func WritePIDFile(pidFile string) error {
	pid := os.Getpid()
	content := fmt.Sprintf("%d\n", pid)

	if err := os.WriteFile(pidFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入 PID 文件失败: %w", err)
	}

	return nil
}

// ReadPIDFile 读取 PID 文件
func ReadPIDFile(pidFile string) (int, error) {
	content, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("PID 文件不存在")
		}
		return 0, fmt.Errorf("读取 PID 文件失败: %w", err)
	}

	pidStr := strings.TrimSpace(string(content))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, fmt.Errorf("解析 PID 失败: %w", err)
	}

	return pid, nil
}

// RemovePIDFile 删除 PID 文件
func RemovePIDFile(pidFile string) error {
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除 PID 文件失败: %w", err)
	}
	return nil
}

// IsProcessRunning 检查进程是否在运行
func IsProcessRunning(pid int) bool {
	// 发送信号 0 检查进程是否存在
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// CheckDaemonStatus 检查守护进程状态
func CheckDaemonStatus(pidFile string) (bool, int, error) {
	pid, err := ReadPIDFile(pidFile)
	if err != nil {
		return false, 0, err
	}

	if IsProcessRunning(pid) {
		return true, pid, nil
	}

	// PID 文件存在但进程不在运行，可能是异常退出
	return false, pid, fmt.Errorf("进程 %d 未运行（可能是异常退出）", pid)
}

// StopDaemon 停止守护进程
func StopDaemon(pidFile string) error {
	pid, err := ReadPIDFile(pidFile)
	if err != nil {
		return err
	}

	if !IsProcessRunning(pid) {
		// 清理孤立的 PID 文件
		RemovePIDFile(pidFile)
		return fmt.Errorf("进程 %d 未运行", pid)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("查找进程失败: %w", err)
	}

	// 发送 SIGTERM 信号
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("发送停止信号失败: %w", err)
	}

	return nil
}

// ReloadDaemon 重载守护进程配置
func ReloadDaemon(pidFile string) error {
	pid, err := ReadPIDFile(pidFile)
	if err != nil {
		return err
	}

	if !IsProcessRunning(pid) {
		return fmt.Errorf("进程 %d 未运行", pid)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("查找进程失败: %w", err)
	}

	// 发送 SIGHUP 信号
	if err := process.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("发送重载信号失败: %w", err)
	}

	return nil
}
