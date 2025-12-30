#!/bin/bash

# NAM 卸载脚本
# 用法: sudo ./scripts/uninstall.sh

set -e

echo "========================================="
echo "   NAM (Node Access Manager) 卸载脚本"
echo "========================================="
echo ""

# 检查是否为 root 用户
if [ "$EUID" -ne 0 ]; then
    echo "❌ 请使用 root 权限运行此脚本"
    echo "   sudo ./scripts/uninstall.sh"
    exit 1
fi

# 确认卸载
read -p "⚠️  确定要卸载 NAM 吗？这将删除所有服务和数据 (y/N): " confirm
if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
    echo "❌ 取消卸载"
    exit 0
fi

# 停止服务
echo ""
echo "🛑 停止服务..."
if systemctl is-active --quiet nam; then
    systemctl stop nam
    echo "✅ 服务已停止"
else
    echo "⚠️  服务未运行"
fi

# 禁用服务
if systemctl is-enabled --quiet nam 2>/dev/null; then
    systemctl disable nam
    echo "✅ 已禁用开机自启"
fi

# 删除 systemd 服务文件
echo ""
echo "🗑️  删除服务文件..."
if [ -f "/etc/systemd/system/nam.service" ]; then
    rm -f /etc/systemd/system/nam.service
    systemctl daemon-reload
    echo "✅ systemd 服务已删除"
fi

# 删除可执行文件
echo ""
echo "🗑️  删除可执行文件..."
if [ -f "/usr/local/bin/nam" ]; then
    rm -f /usr/local/bin/nam
    echo "✅ 可执行文件已删除"
fi

# 询问是否删除配置和数据
echo ""
read -p "🗑️  是否删除配置文件和数据库？(y/N): " delete_data
if [ "$delete_data" = "y" ] || [ "$delete_data" = "Y" ]; then
    rm -rf /etc/nam
    rm -rf /var/lib/nam
    rm -rf /var/log/nam
    rm -f /var/run/nam.pid
    echo "✅ 配置和数据已删除"
else
    echo "⚠️  保留配置和数据:"
    echo "   - /etc/nam/"
    echo "   - /var/lib/nam/"
    echo "   - /var/log/nam/"
fi

# 清理 iptables 规则（可选）
echo ""
read -p "🗑️  是否清理 NAM 创建的 iptables 规则？(y/N): " cleanup_rules
if [ "$cleanup_rules" = "y" ] || [ "$cleanup_rules" = "Y" ]; then
    echo "正在清理 iptables 规则..."
    # 列出并删除所有带 NAM-BAN 注释的规则
    iptables-save | grep "NAM-BAN" | while read -r line; do
        # 转换为删除命令
        delete_cmd=$(echo "$line" | sed 's/-A /-D /')
        eval "iptables $delete_cmd" 2>/dev/null || true
    done
    echo "✅ iptables 规则已清理"
fi

echo ""
echo "========================================="
echo "✅ NAM 卸载完成！"
echo "========================================="
echo ""
