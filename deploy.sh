#!/bin/bash
# deploy.sh - 部署控制面板到 VPS
# 用法: bash deploy.sh
# 環境變量: VPS_HOST VPS_USER VPS_PASS REMOTE_DIR PANEL_PORT
# 默认: VPS_HOST=192.168.31.55 VPS_USER=maxwell REMOTE_DIR=/opt/singbox-panel PANEL_PORT=19999
set -e

VPS_HOST="${VPS_HOST:-192.168.31.55}"
VPS_USER="${VPS_USER:-maxwell}"
VPS_PASS="${VPS_PASS:-}"
REMOTE_DIR="${REMOTE_DIR:-/opt/singbox-panel}"
PORT="${PANEL_PORT:-19999}"

if [ -z "$VPS_PASS" ]; then
    echo "ERROR: VPS_PASS not set. Source .env or export VPS_PASS=xxx"
    echo "Example: source ~/.hermes/profiles/puss_profile/.env"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "=== 部署 Sing-Box 控制面板 ==="
echo "目标: $VPS_USER@$VPS_HOST:$REMOTE_DIR"

# 1. 打包
TMP_ARCHIVE="/tmp/singbox-panel.tar.gz"
tar -czf "$TMP_ARCHIVE" -C "$SCRIPT_DIR" .

# 2. 用 sshpass + base64 传输
echo "[$(basename "$TMP_ARCHIVE")] 大小: $(du -h "$TMP_ARCHIVE" | cut -f1)"
echo "传输中..."

B64=$(base64 "$TMP_ARCHIVE")
sshpass -p "$VPS_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "$VPS_USER@$VPS_HOST" \
    "mkdir -p $REMOTE_DIR && echo '$B64' | base64 -d | tar -xzf - -C $REMOTE_DIR && echo TRANSMIT_OK"

# 3. 确认
echo "部署文件列表:"
sshpass -p "$VPS_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "$VPS_USER@$VPS_HOST" \
    "ls -la $REMOTE_DIR"

# 4. 启动面板
echo "启动面板 (端口 $PORT)..."
sshpass -p "$VPS_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "$VPS_USER@$VPS_HOST" \
    "export SINGBOX_SUDO_PASS='$VPS_PASS' && SINGBOX_PANEL_PORT=$PORT PIPX_HOME=/opt/pipx SUDO_ASKPASS='/opt/singbox-panel/sshpass-cmd' sudo -S nohup python3 $REMOTE_DIR/panel.py > $REMOTE_DIR/panel.log 2>&1 & echo PID=\$!"

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

rm -f "$TMP_ARCHIVE"
