#!/bin/bash
# iptables-clear.sh - 清除所有 iptables 规则
set -e
sudo iptables -F
sudo iptables -X
sudo iptables -t nat -F
sudo iptables -t nat -X
sudo iptables -t mangle -F
sudo iptables -t mangle -X
sudo iptables -t raw -F
sudo iptables -t raw -X
sudo ip6tables -F
sudo ip6tables -X
echo "All iptables/ip6tables rules cleared"
