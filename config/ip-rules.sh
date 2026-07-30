#!/bin/bash
# ip-rules.sh — Exclude proxy's own traffic from TUN routing
# Exclude LAN host + localhost from TUN policy routing
# Must be run with sudo

# Exclude LAN host IP (replace with your host IP)
sudo ip rule add from 192.168.31.55 lookup main pref 8000
sudo ip route add unreachable 192.168.31.55 src 192.168.31.55 table main 2>/dev/null || true

# Exclude loopback traffic
sudo ip rule add from all iif lo lookup main pref 8001
