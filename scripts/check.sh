#!/bin/bash
# y-ui Environment Check Script
# Usage: sudo bash check.sh
# Checks all prerequisites for y-ui deployment

set -e

PASS=0
FAIL=0
WARN=0

ok()  { echo -e "  [PASS] $1"; PASS=$((PASS+1)); }
fail(){ echo -e "  [FAIL] $1"; FAIL=$((FAIL+1)); }
warn(){ echo -e "  [WARN] $1"; WARN=$((WARN+1)); }

check_root() {
    echo ""
    echo "=== 1. Permissions ==="
    if [ "$(id -u)" -ne 0 ]; then
        fail "Not running as root. Use: sudo bash check.sh"
        exit 1
    fi
    ok "Running as root"
}

check_apt() {
    echo ""
    echo "=== 2. Package Manager ==="
    if command -v apt >/dev/null 2>&1; then
        ok "apt package manager"
    elif command -v apk >/dev/null 2>&1; then
        ok "apk (Alpine) package manager"
    elif command -v dnf >/dev/null 2>&1; then
        ok "dnf package manager"
    else
        fail "No supported package manager found"
    fi
}

check_deps() {
    echo ""
    echo "=== 3. Required Dependencies ==="
    for cmd in curl wget iptables iproute2 bash; do
        case "$cmd" in
            iproute2)
                if command -v ip >/dev/null 2>&1; then
                    ok "iproute2 (ip)"
                else
                    fail "iproute2 not found — run: apt install iproute2 / apk add iproute2"
                fi
                ;;
            *)
                if command -v "$cmd" >/dev/null 2>&1; then
                    ok "$cmd"
                else
                    fail "$cmd not found"
                fi
                ;;
        esac
    done

    if command -v sudo >/dev/null 2>&1; then
        ok "sudo"
    else
        fail "sudo not found"
    fi
}

check_systemd() {
    echo ""
    echo "=== 4. Service Manager ==="
    if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
        ok "systemd"
    elif command -v rc-service >/dev/null 2>&1; then
        ok "OpenRC"
    else
        fail "No systemd or OpenRC found"
    fi
}

check_singbox() {
    echo ""
    echo "=== 5. Sing-Box Binary ==="
    SBIN="/etc/sing-box/bin/sing-box"
    if [ -x "$SBIN" ]; then
        ok "sing-box binary at $SBIN"
        echo "     version: $(sudo "$SBIN" version 2>&1 || echo 'unknown')"
    else
        fail "sing-box binary not found at $SBIN"
        echo "     → Download: curl -sL https://github.com/SagerNet/sing-box/releases/latest/download/sing-box-linux-amd64.tar.xz -o /tmp/sb.tar.xz"
    fi
}

check_config() {
    echo ""
    echo "=== 6. Configuration Files ==="
    CFG="/etc/sing-box/config.json"
    if [ -f "$CFG" ]; then
        if python3 -m json.tool "$CFG" >/dev/null 2>&1; then
            ok "config.json — valid JSON"
        else
            fail "config.json — invalid JSON"
        fi
    else
        fail "config.json not found at $CFG"
        echo "     → Copy template: config/sing-box-config.json → $CFG"
    fi

    US="/etc/sing-box/us-proxy.json"
    if [ -f "$US" ]; then
        ok "us-proxy.json"
    else
        warn "us-proxy.json missing at $US"
    fi

    JP="/etc/sing-box/jp-proxy.json"
    if [ -f "$JP" ]; then
        ok "jp-proxy.json"
    else
        warn "jp-proxy.json missing at $JP"
    fi
}

check_yui() {
    echo ""
    echo "=== 7. y-ui Binary ==="
    YUI="/opt/y-ui/y-ui"
    if [ -x "$YUI" ]; then
        ok "y-ui binary at $YUI"
    else
        fail "y-ui binary not found at $YUI"
        echo "     → From GitHub Release: curl -sL https://github.com/whypuss/y-ui/releases/latest/download/y-ui-linux -o /opt/y-ui/y-ui"
    fi
}

check_services() {
    echo ""
    echo "=== 8. systemd Services ==="
    for svc in sing-box sing-box-us sing-box-jp y-ui; do
        if [ -f "/etc/systemd/system/${svc}.service" ]; then
            st=$(systemctl is-active "$svc" 2>/dev/null || true)
            if [ "$st" = "active" ]; then
                ok "${svc}.service — active"
            else
                warn "${svc}.service — not active (status: ${st})"
            fi
        else
            fail "${svc}.service — not found at /etc/systemd/system/"
        fi
    done
}

check_ports() {
    echo ""
    echo "=== 9. Listening Ports ==="
    for port in 443 8894 1080 10808 17777 20000 20001 10810 10811 19999; do
        if ss -tlnp 2>/dev/null | grep -q ":${port} " || ss -ulnp 2>/dev/null | grep -q ":${port} "; then
            ok "Port ${port}"
        else
            warn "Port ${port} not listening"
        fi
    done
}

check_sudoers() {
    echo ""
    echo "=== 10. sudoers ==="
    if [ -f "/etc/sudoers.d/y-ui" ]; then
        ok "sudoers file exists"
    else
        warn "sudoers file missing at /etc/sudoers.d/y-ui"
        echo "     → Copy from: sudoers/y-ui → /etc/sudoers.d/y-ui"
    fi
}

check_network() {
    echo ""
    echo "=== 11. Network Connectivity ==="
    CODE=$(curl -s --max-time 8 -o /dev/null -w "%{http_code}" https://www.google.com 2>/dev/null || true)
    if [ "$CODE" = "200" ] || [ "$CODE" = "301" ] || [ "$CODE" = "302" ]; then
        ok "Internet access OK (HTTP $CODE)"
    else
        warn "Internet access issue (HTTP $CODE)"
    fi
}

# === Main ===
echo "======================================"
echo " y-ui Deployment Environment Check"
echo "======================================"
check_root
check_apt
check_deps
check_systemd
check_singbox
check_config
check_yui
check_services
check_ports
check_sudoers
check_network

echo ""
echo "======================================"
echo " Summary: ${PASS} passed, ${FAIL} failed, ${WARN} warnings"
echo "======================================"
if [ "$FAIL" -eq 0 ]; then
    echo " ✓ Deployment environment is ready."
else
    echo " ✗ Fix the ${FAIL} failure(s) before deploying."
fi
