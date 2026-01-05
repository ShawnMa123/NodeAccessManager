# NodeAccessManager (NAM)

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[English](#english) | [中文](#chinese)

<a name="english"></a>
## 📖 Overview

**NodeAccessManager (NAM)** is a Linux VPS proxy access control tool that provides **port-based concurrent IP limitation** for proxy tools like Sing-box/Xray through kernel-level connection management.

### Key Features

- ✅ **Single Binary** - 8-10MB single executable, no runtime dependencies.
- 🔍 **Auto Discovery** - Automatically detects proxy processes and parses configurations.
- 📊 **Real-time Monitor** - TUI real-time monitoring interface for connection status.
- 🚫 **Smart Eviction** - FIFO/LIFO strategies, TCP Reset disconnection.
- 🛡️ **Zero Intrusion** - Does not modify proxy core code.
- 🔐 **Reliable** - Comprehensive logging, persistence, and error handling.

## 🚀 Quick Start

### Installation

```bash
# Download latest version
wget https://github.com/ShawnMa123/NodeAccessManager/releases/latest/download/nam-linux-amd64

# Add execution permission
chmod +x nam-linux-amd64
sudo mv nam-linux-amd64 /usr/local/bin/nam

# Initialize configuration
sudo nam init
```

### Usage

```bash
# Start daemon
sudo nam start --daemon

# Real-time monitor
sudo nam monitor

# Check status
nam status

# Install as system service
sudo nam install
sudo systemctl start nam
```

## 📋 Features

| Feature | Description |
|---------|-------------|
| **Auto Discovery** | Scan system processes → Locate config → Extract listening ports |
| **Real-time Monitor** | Collect TCP states → Maintain sessions → Trigger policies |
| **Smart Eviction** | FIFO/LIFO strategies → TCP Reset → iptables cooling bans |
| **Visual Interface** | Real-time TUI panel → Connection list → Event logs → Charts |
| **Access Control** | IP-level whitelist/blacklist → CIDR support |
| **History** | SQLite persistence → Ban history → Traffic stats |

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────┐
│                  User Interface                 │
│    TUI Monitor  │  CLI Command  │  HTTP API     │
└─────────┬───────────────┬─────────────┬─────────┘
          │               │             │
┌─────────┼───────────────┼─────────────┼─────────┐
│         ▼               ▼             ▼         │
│    Discovery  →  Monitor  →  Enforcer           │
│     Module        Module      Module            │
└─────────┬───────────────┬─────────────┬─────────┘
          │               │             │
          ▼               ▼             ▼
    Config Store    SQLite DB     Log Files
```

---

<a name="chinese"></a>
## 📖 项目概述 (Project Overview)

**NodeAccessManager (NAM)** 是一款通过内核层面的连接管理，为 Sing-box/Xray 等代理工具提供**基于端口的并发 IP 限制**能力的工具。

### 核心特性

- ✅ **单文件部署** - 8-10MB 单一可执行文件，无需运行时依赖
- 🔍 **智能识别** - 自动检测代理进程，解析配置文件
- 📊 **实时监控** - TUI 实时监控界面，直观展示连接状态
- 🚫 **智能驱逐** - FIFO/LIFO 策略，TCP Reset 断连
- 🛡️ **零侵入设计** - 不修改代理核心代码
- 🔐 **企业级可靠** - 完善的日志、持久化、错误处理

## 🚀 快速开始

### 安装

```bash
# 下载最新版本
wget https://github.com/ShawnMa123/NodeAccessManager/releases/latest/download/nam-linux-amd64

# 添加执行权限
chmod +x nam-linux-amd64
sudo mv nam-linux-amd64 /usr/local/bin/nam

# 初始化配置
sudo nam init
```

### 基本使用

```bash
# 启动守护进程
sudo nam start --daemon

# 实时监控
sudo nam monitor

# 查看状态
nam status

# 安装为系统服务
sudo nam install
sudo systemctl start nam
```

## 📋 功能特性

| 功能模块 | 功能描述 |
|---------|---------|
| **自动发现** | 扫描系统进程 → 定位配置文件 → 提取监听端口 |
| **实时监控** | 采集 TCP 连接状态 → 维护会话记录 → 触发策略判断 |
| **智能驱逐** | FIFO/LIFO 策略 → TCP Reset 断连 → iptables 冷却封禁 |
| **可视化界面** | 实时 TUI 面板 → 连接列表 → 事件日志 → 统计图表 |
| **访问控制** | IP 级别的白名单/黑名单 → 支持 CIDR 格式 |
| **历史记录** | SQLite 持久化 → 封禁历史 → 流量统计 |

## 🏗️ 架构设计

```
┌─────────────────────────────────────────────────┐
│                  用户接口层                      │
│    TUI Monitor  │  CLI命令行  │  HTTP API       │
└─────────┬───────────────┬─────────────┬─────────┘
          │               │             │
┌─────────┼───────────────┼─────────────┼─────────┐
│         ▼               ▼             ▼         │
│    Discovery  →  Monitor  →  Enforcer          │
│    模块            模块         模块             │
└─────────┬───────────────┬─────────────┬─────────┘
          │               │             │
          ▼               ▼             ▼
    Config Store    SQLite DB     Log Files
```

## 🛠️ 技术栈 (Tech Stack)

- **Language**: Go 1.23+
- **TUI Framework**: Bubble Tea + Lipgloss
- **CLI Framework**: Cobra
- **Database**: SQLite
- **Logging**: Logrus

## 📦 项目结构 (Project Structure)

```
.
├── cmd/nam/              # Main entry point (主程序入口)
├── internal/
│   ├── config/           # Configuration management (配置管理)
│   ├── core/             # Core controller (核心控制器)
│   ├── discovery/        # Auto discovery module (自动发现模块)
│   ├── monitor/          # Monitoring module (监控模块)
│   ├── enforcer/         # Enforcement module (执行模块)
│   ├── storage/          # Data persistence (数据持久化)
│   └── tui/              # TUI Interface (TUI 界面)
├── pkg/utils/            # Utility functions (工具函数)
└── go.mod
```

## 🤝 Contribution (贡献)

Welcome to submit Issues and Pull Requests! (欢迎提交 Issue 和 Pull Request！)

## 📄 License (许可证)

MIT License

## 🙏 Acknowledgements (致谢)

This project uses the following open source projects (本项目使用了以下优秀的开源项目):

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI Framework
- [Cobra](https://github.com/spf13/cobra) - CLI Framework
- [Logrus](https://github.com/sirupsen/logrus) - Logging Library

---

**Note**: This tool requires root privileges to operate iptables and system calls. (本工具需要 root 权限运行，因为需要操作 iptables 和系统调用。)
