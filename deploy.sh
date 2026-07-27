#!/bin/bash
# deploy.sh - 部署控制面板到 VPS
# 用法: bash deploy.sh
# 面板端口: 19999
# 访问: http://<VPS_IP>:19999/
set -e

VPS_HOST="192.168.31.55"
VPS_USER="maxwell"
VPS_PASS="qwerty66"
REMOTE_DIR="/opt/singbox-panel"
PORT="19999"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "=== 部署 Sing-Box 控制面板 ==="
echo "目标: $VPS_USER@$VPS_HOST:$REMOTE_DIR"

# 1. 打包
TMP_ARCHIVE="/tmp/singbox-panel.tar.gz"
tar -czf "$TMP_ARCHIVE" -C "$SCRIPT_DIR" .

# 2. 用 Docker alpine 传输
echo "传输文件..."
SSH_AUTH_SOCK="" sshpass -p "$VPS_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "$VPS_USER@$VPS_HOST" \
    "sudo mkdir -p $REMOTE_DIR && docker run --rm -v $REMOTE_DIR:/target alpine sh -c 'cat /dev/stdin > /target/deploy.tar.gz' < /dev/null" 2>/dev/null || true

# 用 sshpass + base64 传输 tarball
echo "[$(basename "$TMP_ARCHIVE")] 大小: $(du -h "$TMP_ARCHIVE" | cut -f1)"
B64=$(base64 "$TMP_ARCHIVE")
echo "传输中..."

sshpass -p "$VPS_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "$VPS_USER@$VPS_HOST" \
    "mkdir -p $REMOTE_DIR && echo $B64 | base64 -d | tar -xzf - -C $REMOTE_DIR && echo TRANSMIT_OK"

# 3. 确认
echo "部署文件列表:"
sshpass -p "$VPS_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "$VPS_USER@$VPS_HOST" \
    "ls -la $REMOTE_DIR"

# 4. 启动面板 (用 sudo 后台运行)
echo "启动面板 (端口 $PORT)..."
sshpass -p "$VPS_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "$VPS_USER@$VPS_HOST" \
    "sudo nohup python3 $REMOTE_DIR/panel.py > $REMOTE_DIR/panel.log 2>&1 & echo PID=\$!"

sleep 2

# 5. 验证
echo "检查端口..."
sshpass -p "$VPS_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "$VPS_USER@$VPS_HOST" \
    "ss -tlnp | grep $PORT || echo PORT_NOT_LISTENING"

echo ""
echo "=== 部署完成 ==="
echo "访问: http://$VPS_HOST:$PORT/"
echo "日志: $REMOTE_DIR/panel.log"
echo "停止: sudo kill \$(cat $REMOTE_DIR/panel.pid)"

# 清理
rm -f "$TMP_ARCHIVE"
