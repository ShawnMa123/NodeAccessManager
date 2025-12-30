#!/bin/bash

# NAM 安装脚本
# 用法: sudo ./scripts/install.sh

set -e

echo "========================================="
echo "   NAM (Node Access Manager) 安装脚本"
echo "========================================="
echo ""

# 检查是否为 root 用户
if [ "$EUID" -ne 0 ]; then
    echo "❌ 请使用 root 权限运行此脚本"
    echo "   sudo ./scripts/install.sh"
    exit 1
fi

# 检查系统依赖
echo "🔍 检查系统依赖..."
if ! command -v iptables &> /dev/null; then
    echo "❌ 未找到 iptables，请先安装"
    exit 1
fi

if ! command -v ss &> /dev/null; then
    echo "❌ 未找到 ss 命令，请安装 iproute2"
    exit 1
fi

echo "✅ 系统依赖检查通过"

# 创建必要的目录
echo ""
echo "📁 创建目录..."
mkdir -p /etc/nam
mkdir -p /var/lib/nam
mkdir -p /var/log/nam
echo "✅ 目录创建完成"

# 复制可执行文件
echo ""
echo "📦 安装可执行文件..."
if [ ! -f "./bin/nam" ]; then
    echo "❌ 找不到编译后的可执行文件 ./bin/nam"
    echo "   请先运行: make build"
    exit 1
fi

cp ./bin/nam /usr/local/bin/nam
chmod +x /usr/local/bin/nam
echo "✅ 可执行文件已安装到 /usr/local/bin/nam"

# 复制示例配置（如果不存在）
echo ""
echo "⚙️  安装配置文件..."
if [ ! -f "/etc/nam/config.yaml" ]; then
    if [ -f "./config/example.yaml" ]; then
        cp ./config/example.yaml /etc/nam/config.yaml
        echo "✅ 示例配置已复制到 /etc/nam/config.yaml"
        echo "   请根据实际需求编辑配置文件"
    else
        echo "⚠️  未找到示例配置文件，请手动创建 /etc/nam/config.yaml"
    fi
else
    echo "⚠️  配置文件已存在，跳过复制"
fi

# 安装 systemd 服务
echo ""
echo "🔧 安装 systemd 服务..."
if [ -f "./systemd/nam.service" ]; then
    cp ./systemd/nam.service /etc/systemd/system/nam.service
    systemctl daemon-reload
    echo "✅ systemd 服务已安装"
else
    echo "⚠️  未找到 systemd 服务文件，跳过"
fi

echo ""
echo "========================================="
echo "✅ NAM 安装完成！"
echo "========================================="
echo ""
echo "📝 下一步操作："
echo "1. 编辑配置文件: vim /etc/nam/config.yaml"
echo "2. 或运行初始化向导: nam init"
echo "3. 启动服务: systemctl start nam"
echo "4. 设置开机自启: systemctl enable nam"
echo "5. 查看状态: nam status 或 systemctl status nam"
echo ""
