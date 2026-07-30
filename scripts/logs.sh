#!/bin/bash
# y-ui Live Logs
# Usage: sudo bash scripts/logs.sh [service]
#   service: sing-box | sing-box-us | sing-box-jp | y-ui (default: y-ui)

SVC="${1:-y-ui}"
echo "=== Live logs for $SVC (Ctrl+C to stop) ==="
sudo journalctl -u "$SVC" -f --no-pager
