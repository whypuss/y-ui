#!/bin/bash
# tproxy-off.sh - 关闭 TProxy (清除 iptables mangle 规则)
set -e
sudo iptables -t mangle -F
sudo ip6tables -t mangle -F
echo "TProxy disabled - iptables mangle cleared"
