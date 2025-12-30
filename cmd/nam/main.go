package main

import (
	"github.com/nodeaccessmanager/nam/cmd/nam/commands"
)

var (
	// Version 版本号，编译时注入
	Version = "dev"
	// BuildTime 编译时间，编译时注入
	BuildTime = "unknown"
)

func main() {
	// 将版本信息传递给命令包
	commands.Version = Version
	commands.BuildTime = BuildTime

	// 执行命令
	commands.Execute()
}
