#!/bin/bash
# restart-singbox.sh - 重启 sing-box (支持 sudo password 環境變量)
SUDO_PASS="${SINGBOX_SUDO_PASS:-}"
if [ -n "$SUDO_PASS" ]; then
    echo "$SUDO_PASS" | sudo -S systemctl restart sing-box
else
    sudo systemctl restart sing-box
fi
sleep 2
echo "sing-box status:"
if [ -n "$SUDO_PASS" ]; then
    echo "$SUDO_PASS" | sudo -S systemctl status sing-box --no-pager | head -12
else
    sudo systemctl status sing-box --no-pager | head -12
fi
echo "---"
ps aux | grep "sing-box" | grep -v grep || echo "no sing-box process"
