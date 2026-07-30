#!/bin/bash
# Restart all services (stop → start in correct order)
set -e
echo "Restarting all services..."
for s in y-ui sing-box sing-box-us sing-box-jp; do
    sudo systemctl stop "$s" 2>/dev/null || true
done
sleep 1
for s in sing-box-us sing-box-jp sing-box y-ui; do
    sudo systemctl start "$s"
done
echo "All services restarted."
for svc in sing-box sing-box-us sing-box-jp y-ui; do
    ST=$(systemctl is-active "$svc" 2>/dev/null || echo "failed")
    echo "  $svc: $ST"
done
