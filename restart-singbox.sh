#!/bin/bash
# restart-singbox.sh - 重启 sing-box 主进程 (需 sudo)
set -e
sudo systemctl restart sing-box
sleep 2
sudo systemctl status sing-box --no-pager | head -10
echo "---"
ps aux | grep "sing-box run -c /etc/sing-box/config.json" | grep -v grep
