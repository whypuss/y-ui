#!/bin/bash
# tproxy-on.sh - 启用 TProxy (重跑 tproxy-rules.sh)
set -e
TPROXY_SCRIPT="/etc/tproxy-rules.sh"
if [ -x "$TPROXY_SCRIPT" ]; then
    sudo bash "$TPROXY_SCRIPT"
    echo "TProxy enabled via $TPROXY_SCRIPT"
elif [ -f "$TPROXY_SCRIPT" ]; then
    sudo bash "$TPROXY_SCRIPT"
    echo "TProxy enabled via $TPROXY_SCRIPT"
else
    echo "WARNING: $TPROXY_SCRIPT not found"
    # Fallback: add basic tproxy mangle rules
    sudo iptables -t mangle -A PREROUTING -p tcp -j TPROXY --tproxy-mark 0x1/0x1 --on-port 10808
    echo "TProxy enabled via fallback rules"
fi
