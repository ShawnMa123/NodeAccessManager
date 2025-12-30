package main

import (
	"fmt"
	"os"
)

var (
	// Version 版本号，编译时注入
	Version = "dev"
	// BuildTime 编译时间，编译时注入
	BuildTime = "unknown"
)

func main() {
	fmt.Printf("NodeAccessManager (NAM) v%s\n", Version)
	fmt.Printf("Build Time: %s\n", BuildTime)
	fmt.Println("\nA proxy node access control tool for Linux VPS")
	fmt.Println("For help, run: nam --help")
	
	os.Exit(0)
}
