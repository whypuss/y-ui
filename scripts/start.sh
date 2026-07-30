#!/bin/bash
# y-ui Start Services (in correct order)
# Must be run with sudo

set -e

echo "[1/4] Starting sing-box-us.service..."
sudo systemctl enable --now sing-box-us.service

echo "[2/4] Starting sing-box-jp.service..."
sudo systemctl enable --now sing-box-jp.service

sleep 1

echo "[3/4] Starting sing-box.service (main)..."
sudo systemctl enable --now sing-box.service

sleep 2

echo "[4/4] Starting y-ui.service (panel)..."
sudo systemctl enable --now y-ui.service

echo ""
echo "=== All services started ==="
for svc in sing-box sing-box-us sing-box-jp y-ui; do
    ST=$(systemctl is-active "$svc" 2>/dev/null || echo "failed")
    echo "  $svc: $ST"
done
