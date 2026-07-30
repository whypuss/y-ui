#!/bin/bash
# y-ui Stop Services (reverse order)
# Must be run with sudo

set -e

echo "Stopping services in reverse order..."
echo "[1/4] Stopping y-ui.service..."
sudo systemctl stop y-ui.service

echo "[2/4] Stopping sing-box.service..."
sudo systemctl stop sing-box.service

echo "[3/4] Stopping sing-box-us.service..."
sudo systemctl stop sing-box-us.service

echo "[4/4] Stopping sing-box-jp.service..."
sudo systemctl stop sing-box-jp.service

echo "All services stopped."
