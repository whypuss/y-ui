#!/bin/bash
# ============================================================
# y-ui Step-by-Step Install Script
# Usage:
#   sudo bash scripts/install.sh          # Full install (interactive)
#   sudo bash scripts/install.sh --full   # Same, explicit
#   sudo bash scripts/install.sh --panel  # y-ui only
#   sudo bash scripts/install.sh --uninstall
# Non-interactive:
#   sudo bash scripts/install.sh --version 1.13.14 --port 19999 --admin maxwell
# ============================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

PANEL_DIR="/opt/y-ui"
PANEL_PORT="${PANEL_PORT:-19999}"
SINGBOX_DIR="/etc/sing-box"
SINGBOX_BIN="/etc/sing-box/bin/sing-box"
GITHUB_REPO="whypuss/y-ui"
YUI_BIN_NAME="y-ui-linux"

# Non-interactive params
DEPLOY_MODE=""
SINGBOX_VERSION=""
ADMIN_USER=""
QUIET=0

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'

info() { echo -e "  ${CYAN}[INFO]${NC}  $*"; }
ok()   { echo -e "  ${GREEN}[OK]${NC}    $*"; }
warn() { echo -e "  ${YELLOW}[WARN]${NC}  $*"; }
err()  { echo -e "  ${RED}[ERR]${NC}   $*" >&2; }

run() {
    if [ "$(id -u)" -ne 0 ]; then
        sudo "$@"
    else
        "$@"
    fi
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        armv7l|armv8l) echo "arm" ;;
        *)             echo "amd64" ;;
    esac
}

detect_distro() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        echo "$ID"
    else
        echo "unknown"
    fi
}

detect_svc_mgr() {
    command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ] && echo "systemd"
    command -v rc-service >/dev/null 2>&1 && echo "openrc"
    echo "none"
}

# ============================================================
# Parse args
# ============================================================
while [ $# -gt 0 ]; do
    case "$1" in
        --full)    DEPLOY_MODE="full" ;;
        --panel)   DEPLOY_MODE="panel" ;;
        --uninstall) DEPLOY_MODE="uninstall" ;;
        --version) SINGBOX_VERSION="$2"; shift ;;
        --port)    PANEL_PORT="$2"; shift ;;
        --admin)   ADMIN_USER="$2"; shift ;;
        --quiet)   QUIET=1 ;;
        *)         err "Unknown arg: $1"; exit 1 ;;
    esac
    shift
done

[ -z "$DEPLOY_MODE" ] && DEPLOY_MODE="full"
[ -z "$ADMIN_USER" ] && ADMIN_USER="$(whoami)"

# ============================================================
# Helpers — step display
# ============================================================
STEP=0
step() {
    STEP=$((STEP+1))
    local total=8
    echo ""
    echo "========================================="
    echo "  [${STEP}/${total}] $*"
    echo "========================================="
}

check_result() {
    local msg="$1"
    if [ -z "$2" ]; then
        fail_msg "$msg"
        exit 1
    else
        ok "$msg"
    fi
}

fail_msg() {
    err "$1"
    echo "  → Fix the issue above, then re-run this script."
}

# ============================================================
# Step functions
# ============================================================

step_check_root() {
    step "Checking root access"
    if [ "$(id -u)" -ne 0 ]; then
        fail_msg "Must run as root. Use: sudo bash install.sh"
    fi
    ok "Running as root (user: $USER)"
}

step_check_deps() {
    step "Installing dependencies"
    DISTRO=$(detect_distro)
    ARCH=$(detect_arch)
    info "Detected distro: $DISTRO, arch: $ARCH"

    case "$DISTRO" in
        ubuntu|debian)
            apt update -qq
            DEPS="curl wget iptables iproute2 sudo python3"
            apt install -y $DEPS 2>/dev/null
            ok "Dependencies installed via apt"
            ;;
        alpine)
            apk add --no-cache curl wget iptables iproute2 sudo python3 2>/dev/null
            ok "Dependencies installed via apk"
            ;;
        centos|rhel|fedora)
            dnf install -y curl wget iptables iproute2 sudo python3 2>/dev/null
            ok "Dependencies installed via dnf"
            ;;
        *)
            warn "Unknown distro '$DISTRO'. Installing common deps with apt..."
            apt update -qq 2>/dev/null && apt install -y curl wget iptables iproute2 sudo python3 2>/dev/null || true
            ;;
    esac

    # Verify
    for cmd in curl iptables ip sudo; do
        command -v "$cmd" >/dev/null 2>&1 || warn "$cmd not found"
    done
}

step_create_dirs() {
    step "Creating directories"
    mkdir -p "$PANEL_DIR"
    mkdir -p "$SINGBOX_DIR"
    mkdir -p "$SINGBOX_DIR/conf"
    mkdir -p /etc/systemd/system
    mkdir -p /etc/sudoers.d
    ok "Directories created"
}

step_download_singbox() {
    step "Downloading sing-box"
    ARCH=$(detect_arch)

    if [ "$SINGBOX_VERSION" = "latest" ] || [ -z "$SINGBOX_VERSION" ]; then
        RELEASE_URL="https://github.com/SagerNet/sing-box/releases/latest/download/sing-box-linux-${ARCH}.tar.xz"
    else
        RELEASE_URL="https://github.com/SagerNet/sing-box/releases/download/v${SINGBOX_VERSION}/sing-box-linux-${ARCH}.tar.xz"
    fi

    info "Downloading from: $RELEASE_URL"
    curl -sL "$RELEASE_URL" -o /tmp/sing-box.tar.xz
    check_result "sing-box downloaded" "$(file /tmp/sing-box.tar.xz)"

    tar -xf /tmp/sing-box.tar.xz -C /tmp/
    mkdir -p "$SINGBOX_DIR/bin"
    mv /tmp/sing-box "$SINGBOX_DIR/bin/sing-box"
    chmod +x "$SINGBOX_DIR/bin/sing-box"
    rm -f /tmp/sing-box.tar.xz

    ok "sing-box installed at $SINGBOX_DIR/bin/"
    "$SINGBOX_DIR/bin/sing-box" version 2>&1 | head -1 || true
}

step_install_yui() {
    step "Installing y-ui binary"
    ARCH=$(detect_arch)

    # Try to download from GitHub releases first
    YUI_URL="https://github.com/${GITHUB_REPO}/releases/latest/download/${YUI_BIN_NAME}"
    info "Trying GitHub Release: $YUI_URL"

    if curl -sL -o /tmp/y-ui "$YUI_URL" && [ -s /tmp/y-ui ]; then
        file /tmp/y-ui | grep -q "executable" || file /tmp/y-ui | grep -qi "ELF"
        if [ $? -eq 0 ]; then
            cp /tmp/y-ui "$PANEL_DIR/y-ui"
            chmod +x "$PANEL_DIR/y-ui"
            rm -f /tmp/y-ui
            ok "y-ui installed from GitHub Release"
            return
        fi
    fi

    # Fallback: build from source if Go is available
    if command -v go >/dev/null 2>&1; then
        info "GitHub Release not available — building from source"
        cd "$REPO_DIR"
        GOOS=linux GOARCH=$ARCH go build -trimpath -o "$PANEL_DIR/y-ui" ./cmd
        chmod +x "$PANEL_DIR/y-ui"
        ok "y-ui built from source"
        return
    fi

    err "Cannot install y-ui: no GitHub Release and no Go compiler"
    echo "  Options:"
    echo "  1. Build locally: cd y-ui && GOOS=linux GOARCH=$ARCH go build -trimpath -o y-ui ./cmd"
    echo "  2. Upload binary: scp ./y-ui user@host:$PANEL_DIR/y-ui"
    echo "  3. Wait for GitHub Release to be published"
    exit 1
}

step_install_services() {
    step "Installing systemd services"

    if [ ! -d /run/systemd/system ]; then
        warn "systemd not available — skipping systemd install"
        return
    fi

    # Copy service files from repo
    for f in sing-box sing-box-us sing-box-jp y-ui; do
        if [ -f "${REPO_DIR}/systemd/${f}.service" ]; then
            cp "${REPO_DIR}/systemd/${f}.service" "/etc/systemd/system/${f}.service"
            ok "${f}.service installed"
        else
            warn "${f}.service not found in repo"
        fi
    done

    # Patch y-ui.service with correct user/port
    sed -i "s/^User=.*/User=${ADMIN_USER}/" "/etc/systemd/system/y-ui.service"
    sed -i "s/^Group=.*/Group=${ADMIN_USER}/" "/etc/systemd/system/y-ui.service"

    systemctl daemon-reload
    ok "systemd daemon reloaded"
}

step_install_config() {
    step "Installing configuration templates"

    # Copy config templates (with placeholders)
    if [ -f "${REPO_DIR}/config/sing-box-config.json" ]; then
        cp "${REPO_DIR}/config/sing-box-config.json" "$SINGBOX_DIR/config.json"
        ok "config.json template copied"
    fi

    for f in us-proxy.json jp-proxy.json ip-rules.sh; do
        if [ -f "${REPO_DIR}/config/$f" ]; then
            cp "${REPO_DIR}/config/$f" "$SINGBOX_DIR/$f"
            ok "$f copied"
        fi
    done

    echo ""
    warn "Config templates contain PLACEHOLDER values."
    warn "Edit before starting services:"
    echo "  sudo nano $SINGBOX_DIR/config.json"
    echo "  sudo nano $SINGBOX_DIR/us-proxy.json"
    echo "  sudo nano $SINGBOX_DIR/jp-proxy.json"
    echo ""
    echo "  Generate UUID:  cat /proc/sys/kernel/random/uuid"
    echo "  Generate PW:    openssl rand -base64 24"
}

step_configure_sudoers() {
    step "Configuring sudoers"

    SUDOERS_SRC="${REPO_DIR}/sudoers/y-ui"
    if [ -f "$SUDOERS_SRC" ]; then
        cp "$SUDOERS_SRC" "/etc/sudoers.d/y-ui"
        chmod 440 "/etc/sudoers.d/y-ui"
        # Replace maxwell with actual admin user
        sed -i "s/^maxwell/${ADMIN_USER}/" "/etc/sudoers.d/y-ui"
        ok "sudoers configured for user: $ADMIN_USER"
    else
        warn "sudoers template not found"
    fi
}

step_enable_ip_forward() {
    step "Enabling IP forwarding"
    echo "net.ipv4.ip_forward = 1" > /etc/sysctl.d/99-yui-forward.conf
    sysctl -p /etc/sysctl.d/99-yui-forward.conf 2>/dev/null || true
    ok "IP forwarding enabled"
}

step_verify() {
    step "Verification"
    echo ""
    echo "  === Installed files ==="
    echo "  y-ui:        $(ls -la $PANEL_DIR/y-ui 2>/dev/null | awk '{print $NF}') ($(stat -c%a $PANEL_DIR/y-ui 2>/dev/null) perms)"
    echo "  sing-box:    $SINGBOX_DIR/bin/sing-box"
    echo "  config:      $SINGBOX_DIR/config.json"
    echo "  services:    sing-box sing-box-us sing-box-jp y-ui"
    echo ""
    echo "  === Next steps ==="
    echo "  1. Edit config: sudo nano $SINGBOX_DIR/config.json"
    echo "  2. Start services:"
    echo "     sudo bash ${REPO_DIR}/scripts/start.sh"
    echo "  3. Check status:"
    echo "     sudo bash ${REPO_DIR}/scripts/status.sh"
    echo "  4. Access panel: http://<SERVER_IP>:$PANEL_PORT/"
}

step_uninstall() {
    step "Uninstalling y-ui"
    for svc in y-ui sing-box sing-box-us sing-box-jp; do
        systemctl stop "$svc" 2>/dev/null || true
        systemctl disable "$svc" 2>/dev/null || true
        rm -f "/etc/systemd/system/${svc}.service"
    done
    systemctl daemon-reload 2>/dev/null || true

    rm -f /etc/sudoers.d/y-ui
    rm -f /etc/sysctl.d/99-yui-forward.conf

    rm -rf "$PANEL_DIR"
    # Don't delete sing-box dir — user may want to keep it

    echo ""
    ok "Uninstall complete."
    echo "  sing-box directory ($SINGBOX_DIR) was NOT deleted."
    echo "  To remove it manually: sudo rm -rf $SINGBOX_DIR"
}

# ============================================================
# Main
# ============================================================
echo ""
echo "============================================"
echo "  y-ui Installer"
echo "============================================"
echo "  Repo:      $GITHUB_REPO"
echo "  Mode:      $DEPLOY_MODE"
echo "  Admin:     $ADMIN_USER"
echo "  Panel Port: $PANEL_PORT"
echo "  Sing-Box:  ${SINGBOX_VERSION:-latest}"
echo "============================================"

if [ "$DEPLOY_MODE" = "uninstall" ]; then
    step_check_root
    step_uninstall
    exit 0
fi

step_check_root
step_check_deps
step_create_dirs
step_download_singbox

if [ "$DEPLOY_MODE" != "panel" ]; then
    step_install_config
fi

step_install_yui
step_install_services
step_configure_sudoers
step_enable_ip_forward
step_verify

echo ""
echo "============================================"
echo "  Install complete!"
echo "============================================"
echo "  Edit configs before starting:"
echo "    sudo nano $SINGBOX_DIR/config.json"
echo ""
echo "  Start services:"
echo "    sudo bash ${REPO_DIR}/scripts/start.sh"
echo ""
echo "  Panel URL: http://<SERVER_IP>:$PANEL_PORT/"
echo "============================================"
