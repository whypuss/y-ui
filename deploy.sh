#!/bin/bash
# deploy.sh - 舊版 SSH 部署腳本（已廢棄，請改用 install.sh）
#
# 新功能腳本:
#   curl -sL https://raw.githubusercontent.com/whypuss/y-ui/main/install.sh | sudo bash -s -- --full
#
# 保留此腳本僅供參考，不會再維護。
# ============================================================
set -e

VPS_HOST="${VPS_HOST:-}"
VPS_USER="${VPS_USER:-}"
VPS_PASS="${VPS_PASS:-}"
REMOTE_DIR="${REMOTE_DIR:-/opt/singbox-panel}"
PORT="${PANEL_PORT:-19999}"

echo "=== 舊版部署腳本（已廢棄）==="
echo "請改用 install.sh:"
echo "  curl -sL https://raw.githubusercontent.com/whypuss/y-ui/main/install.sh | sudo bash -s -- --full"
echo ""

if [ -z "$VPS_HOST" ]; then
    echo "ERROR: 請設定 VPS_HOST 環境變量"
    echo "  export VPS_HOST=your-vps-ip"
    exit 1
fi

if [ -z "$VPS_PASS" ]; then
    echo "ERROR: 請設定 VPS_PASS 環境變量"
    echo "  export VPS_PASS=your-password"
    exit 1
fi

echo "目標: ${VPS_USER:-$USER}@${VPS_HOST}:${REMOTE_DIR}"
echo "端口: ${PORT}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# 1. 打包（排除自身和敏感文件）
TMP_ARCHIVE="/tmp/singbox-panel.tar.gz"
tar -czf "$TMP_ARCHIVE" -C "$SCRIPT_DIR" \
    --exclude='deploy.sh' --exclude='.env' --exclude='*.key' --exclude='*.pem' \
    --exclude='.git' --exclude='*.zip' \
    . 2>/dev/null

echo "[${TMP_ARCHIVE}] 大小: $(du -h "$TMP_ARCHIVE" | cut -f1)"
echo "傳輸中..."

B64=$(base64 "$TMP_ARCHIVE")
sshpass -p "$VPS_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "${VPS_USER:-$USER}@${VPS_HOST}" \
    "mkdir -p ${REMOTE_DIR} && echo '${B64}' | base64 -d | tar -xzf - -C ${REMOTE_DIR} && echo TRANSMIT_OK"

echo "部署文件列表:"
sshpass -p "$VPS_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "${VPS_USER:-$USER}@${VPS_HOST}" \
    "ls -la ${REMOTE_DIR}"

echo "啟動面板 (端口 ${PORT})..."
sshpass -p "$VPS_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "${VPS_USER:-$USER}@${VPS_HOST}" \
    "export SINGBOX_SUDO_PASS='${VPS_PASS}' && SINGBOX_PANEL_PORT=${PORT} sudo -S nohup python3 ${REMOTE_DIR}/panel.py > ${REMOTE_DIR}/panel.log 2>&1 & echo PID=\$!"

sleep 2

echo "檢查端口..."
sshpass -p "$VPS_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "${VPS_USER:-$USER}@${VPS_HOST}" \
    "ss -tlnp | grep ${PORT} || echo PORT_NOT_LISTENING"

echo ""
echo "=== 部署完成 ==="
echo "訪問: http://${VPS_HOST}:${PORT}/"
echo "日誌: ${REMOTE_DIR}/panel.log"

rm -f "$TMP_ARCHIVE"
