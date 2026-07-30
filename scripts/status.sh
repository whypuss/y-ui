#!/bin/bash
# y-ui Service Status
# Shows status of all sing-box + y-ui services

for svc in sing-box sing-box-us sing-box-jp y-ui; do
    if [ -f "/etc/systemd/system/${svc}.service" ]; then
        ST=$(systemctl is-active "$svc" 2>/dev/null || echo "not-installed")
        echo "[$ST] $svc"
        if [ "$ST" = "active" ]; then
            echo "  PID: $(systemctl show -p MainPID "$svc" --value 2>/dev/null)"
        fi
    else
        echo "[missing] $svc.service"
    fi
done
echo ""
echo "--- Listening Ports ---"
ss -tlnp 2>/dev/null | grep -E "sing-box|y-ui" || echo "no sing-box/y-ui listeners"
echo ""
echo "--- IP Rules (TUN) ---"
ip rule show 2>/dev/null || echo "ip command not available"
