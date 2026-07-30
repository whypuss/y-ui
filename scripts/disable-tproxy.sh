#!/bin/bash
# Disable TProxy rules (remove iptables TPROXY + ip rule)
# Use this to clean up after TProxy was enabled

set -e

echo "Disabling TProxy rules..."
sudo iptables -t mangle -F PREROUTING
sudo ip rule del fwmark 0x1/0x1 lookup 100 2>/dev/null || true
sudo ip route del local 0.0.0.0/0 dev lo table 100 2>/dev/null || true

# Also remove tproxy-rules service dependency
sudo systemctl disable tproxy-rules.service 2>/dev/null || true

echo "TProxy disabled."
echo ""
echo "Verify:"
sudo iptables -t mangle -L PREROUTING -n
ip rule show 2>/dev/null | grep fwmark || echo "no fwmark rules"
