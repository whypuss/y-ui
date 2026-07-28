#!/bin/bash
# ============================================================
# y-ui 一鍵部署腳本
# 用法:
#   curl -o install.sh https://raw.githubusercontent.com/whypuss/y-ui/main/install.sh
#   bash install.sh              # 互動式 (僅部署 y-ui)
#   bash install.sh --full       # 完整部署 (sing-box + y-ui)
#   bash install.sh --panel      # 僅部署 y-ui 到現有 sing-box
#   bash install.sh --uninstall  # 解除安裝
# 非互動參數:
#   --port <端口>       面板端口 (預設 19999)
#   --version <版本>    sing-box 版本，e.g. 1.13.14 (預設 latest)
#   --admin <用戶名>    管理用戶 (預設當前用戶)
# 示例:
#   curl -sL https://raw.githubusercontent.com/whypuss/y-ui/main/install.sh | sudo bash -s -- --full
#   sudo bash install.sh --full --version 1.13.14 --port 19999 --admin root
# ============================================================
set -e

PANEL_DIR="/opt/y-ui"
PANEL_PORT="${PANEL_PORT:-19999}"
SINGBOX_DIR="/etc/sing-box"
SINGBOX_BIN_DIR="${SINGBOX_DIR}/bin"
SINGBOX_CONFIG="${SINGBOX_DIR}/config.json"
GITHUB_REPO="whypuss/y-ui"

# ---- 可選非互動參數 ----
DEPLOY_MODE=""       # full / panel / uninstall
SINGBOX_VERSION=""   # 預設 latest
ADMIN_USER=""        # 預設 $(whoami)
QUIET="0"            # 非互動模式

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}[INFO]${NC}  $*"; }
ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()   { echo -e "${RED}[ERR]${NC}  $*" >&2; }

# ---------- 工具函數 ----------

run() {
    if [ "$(id -u)" -ne 0 ]; then
        sudo "$@"
    else
        "$@"
    fi
}

detect_arch() {
    local m
    m=$(uname -m)
    case "$m" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        armv7l|armv8l) echo "arm" ;;
        *) echo "amd64" ;;
    esac
}

check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        err "請以 root 身份運行（或使用 sudo）"
        err "例: sudo bash install.sh"
        exit 1
    fi
}

detect_systemd() {
    if command -v systemctl >/dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# SERVICE_TYPE: systemd / openrc / none
detect_service_manager() {
    if detect_systemd; then
        echo "systemd"
        return 0
    elif [ -d /etc/init.d ] && [ -f /sbin/openrc-run ]; then
        echo "openrc"
        return 0
    else
        echo "none"
        return 1
    fi
}

# ---------- 安裝依賴 ----------

install_deps() {
    info "檢測並安裝依賴..."
    if command -v apt-get >/dev/null 2>&1; then
        apt-get update -qq
        DEPS="curl wget iptables iproute2 git sudo"
        for d in $DEPS; do
            dpkg -l "$d" >/dev/null 2>&1 || apt-get install -y -qq "$d" 2>/dev/null || true
        done
        ok "依賴安裝完成 (apt)"
    elif command -v yum >/dev/null 2>&1; then
        yum install -y curl wget iptables iproute git sudo 2>/dev/null || \
            dnf install -y curl wget iptables iproute git sudo 2>/dev/null || true
        ok "依賴安裝完成 (yum/dnf)"
    elif command -v apk >/dev/null 2>&1; then
        apk add --no-cache curl wget iptables iproute2 git sudo 2>/dev/null || true
        ok "依賴安裝完成 (apk)"
    else
        warn "無法自動檢測包管理器，請手動安裝 curl wget iptables iproute2 git"
    fi
}

# ---------- sing-box 安裝 ----------

install_singbox() {
    local version="${1:-latest}"
    local arch
    arch=$(detect_arch)

    if [ "$version" = "latest" ]; then
        info "檢測 sing-box 最新穩定版..."
        version=$(curl -s https://api.github.com/repos/SagerNet/sing-box/releases/latest | grep -o '"tag_name": "[^"]*"' | sed 's/"tag_name": "//; s/"//' | sed 's/^v//')
        info "最新穩定版: v${version}"
    fi
    echo -e "  ${GREEN}sing-box 版本:${NC} ${YELLOW}${version}${NC} (linux-${arch})"

    local url="https://github.com/SagerNet/sing-box/releases/download/v${version}/sing-box-v${version}-linux-${arch}.tar.gz"

    info "下載中..."
    local tmpfile="/tmp/sing-box-v${version}-linux-${arch}.tar.gz"
    if ! curl -fsSL -o "$tmpfile" "$url" 2>/dev/null; then
        # 嘗試另一種文件名格式（舊版用）
        url="https://github.com/SagerNet/sing-box/releases/download/v${version}/sing-box-${version}-linux-${arch}.tar.gz"
        tmpfile="/tmp/sing-box-${version}-linux-${arch}.tar.gz"
        curl -fsSL -o "$tmpfile" "$url"
    fi
    ok "下載完成"

    mkdir -p "$SINGBOX_BIN_DIR" "$SINGBOX_DIR/conf"
    # tar 內一般有前綴目錄 sing-box-vX.Y.Z-linux-arch/
    local tar_dir
    tar_dir=$(tar -tzf "$tmpfile" 2>/dev/null | head -1 | cut -d/ -f1)
    if [ -n "$tar_dir" ] && [ "$tar_dir" != "sing-box" ]; then
        tar -xzf "$tmpfile" -C /tmp "$tar_dir/sing-box"
        mv "/tmp/${tar_dir}/sing-box" /tmp/sing-box 2>/dev/null || true
    else
        tar -xzf "$tmpfile" -C /tmp
    fi
    chmod +x /tmp/sing-box
    mv /tmp/sing-box "${SINGBOX_BIN_DIR}/sing-box"
    ln -sf "${SINGBOX_BIN_DIR}/sing-box" /usr/local/bin/sing-box
    rm -f "$tmpfile"

    local ver
    ver=$("${SINGBOX_BIN_DIR}/sing-box" version 2>&1 | head -1)
    ok "sing-box 安裝完成: ${ver}"
}

# ---------- y-ui 安裝 ----------

install_yui() {
    local arch
    arch=$(detect_arch)
    info "準備 y-ui binary..."

    # 1. 嘗試從 GitHub Release 自動下載
    local release_bin
    release_bin=$(curl -s "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" 2>/dev/null | \
        grep -o '"browser_download_url": "[^"]*y-ui-linux-'"${arch}"'[^"]*"' | \
        sed 's/"browser_download_url": "//; s/"$//' | head -1)

    if [ -n "$release_bin" ]; then
        info "從 GitHub Release 下載 y-ui (${arch})..."
        curl -fsSL -o /tmp/y-ui "$release_bin"
        ok "自動下載完成"
    else
        # 2. 自動下載失敗 — 互動式處理
        echo ""
        echo -e "  ${YELLOW}GitHub Release 中未找到 linux-${arch} 的 y-ui binary${NC}"
        echo "  請選擇一種方式獲取:"
        echo "    1) 手動上傳 /tmp/y-ui（推薦）"
        echo "       - 在本地: GOOS=linux GOARCH=${arch} go build -trimpath -o y-ui ./cmd"
        echo "       - scp y-ui user@server:/tmp/y-ui"
        echo "    2) 從源編譯（需先安裝 Go）"
        echo "    3) 貼入 y-ui 的 base64 編碼數據"
        echo ""
        read -rp "請選擇 [1/2/3]: " choice
        echo ""

        case "$choice" in
            1)
                if [ -f /tmp/y-ui ]; then
                    ok "/tmp/y-ui 已存在"
                else
                    read -rp "請將 y-ui 上傳到 /tmp/y-ui，然後按 Enter 繼續..."
                fi
                if [ ! -f /tmp/y-ui ]; then
                    err "/tmp/y-ui 不存在，退出"
                    exit 1
                fi
                ok "使用手動提供的 /tmp/y-ui"
                ;;
            2)
                if ! command -v go >/dev/null 2>&1; then
                    info "安裝 Go..."
                    if command -v apt-get >/dev/null 2>&1; then
                        apt-get install -y -qq golang-go 2>/dev/null || {
                            warn "apt 安裝 Go 失敗，嘗試安裝 golang 最新版..."
                            curl -fsSL https://go.dev/dl/go1.22.0.linux-${arch}.tar.gz | tar -C /usr/local -xzf -
                            export PATH=/usr/local/go/bin:$PATH
                        }
                    elif command -v yum >/dev/null 2>&1; then
                        yum install -y golang 2>/dev/null || true
                    fi
                fi
                info "從 GitHub 主分支編譯 y-ui..."
                rm -rf /tmp/y-ui-src
                git clone --depth=1 https://github.com/${GITHUB_REPO}.git /tmp/y-ui-src
                cd /tmp/y-ui-src
                export PATH=/usr/local/go/bin:$PATH
                GOOS=linux GOARCH="${arch}" go build -trimpath -o /tmp/y-ui ./cmd
                if [ ! -f /tmp/y-ui ]; then
                    err "編譯失敗，請手動上傳 /tmp/y-ui"
                    exit 1
                fi
                rm -rf /tmp/y-ui-src
                ok "編譯完成"
                ;;
            3|*)
                warn "請先手動上傳 y-ui 到 /tmp/y-ui"
                read -rp "上傳完成後按 Enter 繼續..."
                if [ ! -f /tmp/y-ui ]; then
                    err "/tmp/y-ui 不存在，退出"
                    exit 1
                fi
                ok "使用手動提供的 /tmp/y-ui"
                ;;
        esac
    fi

    chmod +x /tmp/y-ui
    mkdir -p "$PANEL_DIR"
    mv /tmp/y-ui "${PANEL_DIR}/y-ui"
    ok "y-ui 安裝至 ${PANEL_DIR}/y-ui"
}

# ---------- y-ui service (systemd / OpenRC) ----------

install_yui_service() {
    local sm
    sm=$(detect_service_manager)

    if [ "$sm" = "systemd" ]; then
        cat > /etc/systemd/system/y-ui.service << SVCEOF
[Unit]
Description=y-ui Web Panel
After=network.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${PANEL_DIR}/y-ui -port ${PANEL_PORT}
Restart=on-failure
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
SVCEOF
        systemctl daemon-reload
        systemctl enable y-ui
        ok "y-ui systemd service 已安裝 (端口 ${PANEL_PORT})"
    elif [ "$sm" = "openrc" ]; then
        cat > /etc/init.d/y-ui << INITEOF
#!/sbin/openrc-run
name="y-ui Web Panel"
description="y-ui Sing-Box 控制面板"
command="${PANEL_DIR}/y-ui"
command_args="-port ${PANEL_PORT}"
command_background="yes"
pidfile="/run/y-ui.pid"
command_user="root"
INITEOF
        chmod +x /etc/init.d/y-ui
        rc-update add y-ui default 2>/dev/null || true
        ok "y-ui OpenRC init 已安裝 (端口 ${PANEL_PORT})"
    else
        warn "無 service manager，y-ui 需手動啟動"
        info "手動啟動: ${PANEL_DIR}/y-ui -port ${PANEL_PORT} &"
    fi
}

# ---------- sing-box service (systemd / OpenRC) ----------

install_singbox_service() {
    local sm
    sm=$(detect_service_manager)

    if [ "$sm" = "systemd" ]; then
        cat > /etc/systemd/system/sing-box.service << SVCEOF
[Unit]
Description=sing-box proxy service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
Environment=ENABLE_DEPRECATED_LEGACY_DNS_SERVERS=true
ExecStart=${SINGBOX_BIN_DIR}/sing-box run -c ${SINGBOX_CONFIG} -C ${SINGBOX_DIR}/conf
Restart=always
RestartSec=3
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE
LimitNPROC=10000
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
SVCEOF
        systemctl daemon-reload
        systemctl enable sing-box
        ok "sing-box systemd service 已安裝"
    elif [ "$sm" = "openrc" ]; then
        cat > /etc/init.d/sing-box << INITEOF
#!/sbin/openrc-run
name="sing-box proxy"
description="sing-box proxy service"
command="${SINGBOX_BIN_DIR}/sing-box"
command_args="run -c ${SINGBOX_CONFIG} -C ${SINGBOX_DIR}/conf"
command_background="yes"
pidfile="/run/sing-box.pid"
command_user="root"
environment="ENABLE_DEPRECATED_LEGACY_DNS_SERVERS=true"
INITEOF
        chmod +x /etc/init.d/sing-box
        rc-update add sing-box default 2>/dev/null || true
        ok "sing-box OpenRC init 已安裝"
    else
        warn "無 service manager，sing-box 需手動啟動"
    fi
}

# ---------- 生成默認 config.json ----------

generate_singbox_config() {
    local panel_port="$1"
    if [ -f "$SINGBOX_CONFIG" ]; then
        warn "${SINGBOX_CONFIG} 已存在，跳過生成"
        return 0
    fi
    info "生成默認 config.json（AnyTLS 端口 ${panel_port}）..."

    # 生成 AnyTLS 證書
    mkdir -p "$SINGBOX_DIR"
    openssl genrsa -out "${SINGBOX_DIR}/anytls.key" 2048 >/dev/null 2>&1
    openssl req -new -x509 -days 365 -key "${SINGBOX_DIR}/anytls.key" \
        -out "${SINGBOX_DIR}/anytls.pem" \
        -subj "/CN=localhost/O=y-ui" >/dev/null 2>&1

    local uuid
    uuid=$(cat /proc/sys/kernel/random/uuid)

    cat > "$SINGBOX_CONFIG" << CFGEOF
{
  "dns": {
    "final": "dns-direct",
    "servers": [
      {
        "address": "8.8.8.8",
        "address_strategy": "ipv4_only",
        "tag": "dns-direct"
      }
    ],
    "strategy": "ipv4_only"
  },
  "inbounds": [
    {
      "listen": "0.0.0.0",
      "listen_port": ${panel_port},
      "tag": "ANYTLS-${panel_port}",
      "type": "anytls",
      "users": [
        {
          "password": "${uuid}"
        }
      ],
      "tls": {
        "enabled": true,
        "server_name": "localhost",
        "certificate_path": "${SINGBOX_DIR}/anytls.pem",
        "key_path": "${SINGBOX_DIR}/anytls.key"
      },
      "detour": "anytls-out",
      "strict_route": true
    },
    {
      "listen": "0.0.0.0",
      "listen_port": 1080,
      "type": "socks",
      "tag": "socks-in"
    },
    {
      "listen": "0.0.0.0",
      "listen_port": 10808,
      "type": "mixed",
      "tproxy": false,
      "tag": "mixed-in"
    }
  ],
  "outbounds": [
    {
      "type": "direct",
      "tag": "direct"
    },
    {
      "type": "direct",
      "tag": "dns-out",
      "domain_strategy": "ipv4_only"
    },
    {
      "type": "anytls",
      "tag": "anytls-out",
      "tfo": true,
      "tls": {
        "enabled": true,
        "server_name": "localhost"
      }
    }
  ]
}
CFGEOF

    info "AnyTLS UUID: ${uuid}"
    ok "config.json 已生成"
}

# ---------- sudoers 權限 ----------

setup_sudoers() {
    local user="${1:-}"
    if [ -z "$user" ]; then
        user=$(whoami)
    fi
    [ "$user" = "root" ] && return 0

    info "為用戶 ${user} 設置 sudoers 權限..."
    local target_dir="/etc/sudoers.d"
    mkdir -p "$target_dir"

    cat > "${target_dir}/y-ui" << SUDOEREOF
# y-ui sudoers - 允許面板管理 sing-box / iptables / kill
${user} ALL=(ALL) NOPASSWD: /bin/systemctl restart sing-box, /bin/systemctl start sing-box, /bin/systemctl stop sing-box, /bin/systemctl restart sing-box-main, /bin/systemctl start sing-box-main, /bin/systemctl stop sing-box-main
${user} ALL=(ALL) NOPASSWD: /usr/sbin/iptables*, /sbin/iptables*
${user} ALL=(ALL) NOPASSWD: /usr/sbin/ip*, /sbin/ip*
${user} ALL=(ALL) NOPASSWD: /usr/bin/kill, /bin/kill
${user} ALL=(ALL) NOPASSWD: /usr/bin/systemctl, /bin/systemctl
${user} ALL=(ALL) NOPASSWD: /usr/sbin/conntrack, /sbin/conntrack
SUDOEREOF

    chmod 440 "${target_dir}/y-ui"
    ok "sudoers 權限已設置 (用戶: ${user})"
}

# ---------- 內核參數 ----------

setup_kernel_params() {
    info "設置內核參數 (ip_forward)..."
    grep -q "net.ipv4.ip_forward.*1" /etc/sysctl.conf 2>/dev/null || \
        echo "net.ipv4.ip_forward = 1" >> /etc/sysctl.conf
    sysctl -w net.ipv4.ip_forward=1 >/dev/null 2>&1 || true
    ok "ip_forward = 1"
}

# ---------- 防火牆 ----------

setup_firewall() {
    local port="$1"
    info "開放防火牆端口 ${port}..."
    if command -v ufw >/dev/null 2>&1; then
        ufw allow "$port/tcp" 2>/dev/null || true
    elif command -v firewall-cmd >/dev/null 2>&1; then
        firewall-cmd --permanent --add-port="${port}/tcp" 2>/dev/null || true
        firewall-cmd --reload 2>/dev/null || true
    elif command -v iptables >/dev/null 2>&1; then
        iptables -C INPUT -p tcp --dport "$port" -j ACCEPT 2>/dev/null || \
            iptables -I INPUT -p tcp --dport "$port" -j ACCEPT
        iptables -C INPUT -p udp --dport "$port" -j ACCEPT 2>/dev/null || \
            iptables -I INPUT -p udp --dport "$port" -j ACCEPT 2>/dev/null || true
    fi
    ok "防火牆端口 ${port} 已開放"
}

# ---------- 完整部署 ----------

do_full_deploy() {
    echo ""
    echo "============================================"
    echo "  y-ui 完整部署 (sing-box + y-ui)"
    echo "============================================"
    echo ""

    check_root
    local sm
    sm=$(detect_service_manager) || { err "系統不支持 systemd / OpenRC，退出"; exit 1; }
    info "Service manager: ${sm}"
    install_deps
    setup_kernel_params

    echo ""
    if [ "$QUIET" != "1" ]; then
        read -rp "請選擇 sing-box 版本 [latest / 如 1.13.14, 直接 Enter 用最新]: " input
        if [ -n "$input" ]; then
            SINGBOX_VERSION="$input"
        fi
    fi
    info "sing-box 版本: ${SINGBOX_VERSION}"
    echo ""

    install_singbox "$SINGBOX_VERSION"
    install_singbox_service

    echo ""
    if [ "$QUIET" != "1" ]; then
        read -rp "y-ui 面板端口 [${PANEL_PORT}]: " input
        if [ -n "$input" ]; then
            PANEL_PORT="$input"
        fi
    fi
    info "面板端口: ${PANEL_PORT}"

    generate_singbox_config "$PANEL_PORT"
    setup_firewall "$PANEL_PORT"

    echo ""
    if [ "$QUIET" != "1" ]; then
        read -rp "管理用戶 (面板運行用戶，用於 sudoers, 直接 Enter 用 ${ADMIN_USER}): " au
        if [ -n "$au" ]; then
            ADMIN_USER="$au"
        fi
    fi
    if id "$ADMIN_USER" >/dev/null 2>&1; then
        setup_sudoers "$ADMIN_USER"
    else
        warn "用戶 ${ADMIN_USER} 不存在，將用當前用戶 $(whoami)"
        setup_sudoers "$(whoami)"
    fi

    install_yui
    install_yui_service

    # 啟動
    echo ""
    info "啟動服務..."
    if [ "$sm" = "openrc" ]; then
        /etc/init.d/sing-box start || warn "sing-box 啟動失敗（請檢查 config.json）"
        /etc/init.d/y-ui start
    else
        systemctl restart sing-box || warn "sing-box 啟動失敗（請檢查 config.json）"
        systemctl restart y-ui
    fi

    sleep 2
    local panel_status
    if [ "$sm" = "openrc" ]; then
        panel_status=$(/sbin/rc-status y-ui 2>/dev/null | grep -q y-ui && echo "active" || echo "inactive")
    else
        panel_status=$(systemctl is-active y-ui 2>/dev/null || echo "inactive")
    fi
    echo ""

    echo "============================================"
    if [ "$panel_status" = "active" ]; then
        echo -e "  ${GREEN}✅ 部署成功!${NC}"
    else
        echo -e "  ${YELLOW}⚠️  部署完成但 y-ui 可能未啟動${NC}"
        echo "     請查看: ${sm:=/etc/init.d/} y-ui status"
    fi
    echo ""
    local pub_ip
    pub_ip=$(curl -s --max-time 5 https://ifconfig.me 2>/dev/null || echo "<本机IP>")
    echo "  面板地址: http://${pub_ip}:${PANEL_PORT}/"
    echo "  sing-box: ${SINGBOX_CONFIG}"
    echo "  Service:  ${sm}"
    echo "============================================"
}

# ---------- 僅部署 y-ui ----------

do_panel_deploy() {
    echo ""
    echo "============================================"
    echo "  y-ui 面板部署 (現有 sing-box)"
    echo "============================================"
    echo ""

    # 檢測 sing-box
    if ! command -v sing-box >/dev/null 2>&1; then
        # 嘗試路徑
        if [ ! -f "${SINGBOX_BIN_DIR}/sing-box" ]; then
            err "未檢測到 sing-box。如用完整部署請使用 --full"
            err "或確保 sing-box 已安裝在 ${SINGBOX_BIN_DIR}/sing-box"
            exit 1
        fi
        ln -sf "${SINGBOX_BIN_DIR}/sing-box" /usr/local/bin/sing-box 2>/dev/null || true
    fi

    local sb_ver
    sb_ver=$(sing-box version 2>&1 | head -1 || echo "unknown")
    ok "檢測到 sing-box: ${sb_ver}"

    local sm
    sm=$(detect_service_manager) || { err "系統不支持 systemd / OpenRC，退出"; exit 1; }
    info "Service manager: ${sm}"

    echo ""
    if [ "$QUIET" != "1" ]; then
        read -rp "y-ui 面板端口 [${PANEL_PORT}]: " input
        if [ -n "$input" ]; then
            PANEL_PORT="$input"
        fi
    fi
    info "面板端口: ${PANEL_PORT}"

    # 檢測 config.json 是否存在
    if [ ! -f "$SINGBOX_CONFIG" ]; then
        warn "未找到 ${SINGBOX_CONFIG}"
        if [ "$QUIET" != "1" ]; then
            read -rp "是否生成默認 config.json? (y/N): " gen
            if [ "$gen" = "y" ] || [ "$gen" = "Y" ]; then
                generate_singbox_config "$PANEL_PORT"
            else
                warn "請確保 sing-box 配置存在"
            fi
        else
            info "自動生成默認 config.json"
            generate_singbox_config "$PANEL_PORT"
        fi
    fi

    setup_firewall "$PANEL_PORT"

    echo ""
    if [ "$QUIET" != "1" ]; then
        read -rp "管理用戶 (用於 sudoers, 直接 Enter 用 ${ADMIN_USER}): " au
        if [ -n "$au" ]; then
            ADMIN_USER="$au"
        fi
    fi
    if id "$ADMIN_USER" >/dev/null 2>&1; then
        setup_sudoers "$ADMIN_USER"
    else
        warn "用戶 ${ADMIN_USER} 不存在，將用當前用戶 $(whoami)"
        setup_sudoers "$(whoami)"
    fi

    install_yui
    install_yui_service

    echo ""
    info "啟動 y-ui..."
    if [ "$sm" = "openrc" ]; then
        /etc/init.d/y-ui start
    else
        systemctl restart y-ui
    fi

    sleep 2
    local panel_status
    if [ "$sm" = "openrc" ]; then
        panel_status=$(/sbin/rc-status y-ui 2>/dev/null | grep -q y-ui && echo "active" || echo "inactive")
    else
        panel_status=$(systemctl is-active y-ui 2>/dev/null || echo "inactive")
    fi
    echo ""

    echo "============================================"
    if [ "$panel_status" = "active" ]; then
        echo -e "  ${GREEN}✅ 部署成功!${NC}"
    else
        echo -e "  ${YELLOW}⚠️  y-ui 未啟動，請查看:${NC}"
        echo "     ${sm:=/etc/init.d/} y-ui status"
    fi
    echo ""
    local pub_ip
    pub_ip=$(curl -s --max-time 5 https://ifconfig.me 2>/dev/null || echo "<本机IP>")
    echo "  面板地址: http://${pub_ip}:${PANEL_PORT}/"
    echo "  sing-box config: ${SINGBOX_CONFIG}"
    echo "  Service: ${sm}"
    echo "============================================"
}

# ---------- 解除安裝 ----------

do_uninstall() {
    echo ""
    echo "============================================"
    echo "  解除安裝 y-ui"
    echo "============================================"
    echo ""
    read -rp "確定解除安裝 y-ui? (y/N): " confirm
    [ "$confirm" = "y" ] || [ "$confirm" = "Y" ] || exit 1

    check_root

    # 停服務
    systemctl stop y-ui 2>/dev/null || true
    systemctl disable y-ui 2>/dev/null || true

    # 刪除文件
    rm -rf "$PANEL_DIR"
    rm -f /etc/systemd/system/y-ui.service
    rm -f /etc/sudoers.d/y-ui
    systemctl daemon-reload

    ok "y-ui 已解除安裝"
    echo "  (sing-box 及其配置未被刪除)"
    echo ""
}

# ---------- 主入口 ----------

main() {
    # ---- 解析標記參數 ----
    while [ $# -gt 0 ]; do
        case "$1" in
            --full|-f)
                DEPLOY_MODE="full"; shift ;;
            --panel|-p)
                DEPLOY_MODE="panel"; shift ;;
            --uninstall|-u)
                DEPLOY_MODE="uninstall"; shift ;;
            --help|-h)
                echo "y-ui 一鍵部署腳本"
                echo ""
                echo "用法:"
                echo "  sudo bash install.sh              # 互動式 (僅部署 y-ui)"
                echo "  sudo bash install.sh --full       # 完整部署 (安裝 sing-box + y-ui)"
                echo "  sudo bash install.sh --panel      # 在現有 sing-box 上部署 y-ui"
                echo "  sudo bash install.sh --uninstall  # 解除安裝 y-ui"
                echo "  sudo bash install.sh --help       # 顯示此幫助"
                echo ""
                echo "非互動參數 (可與 --full/--panel 組合):"
                echo "  --port <端口>       面板端口 (預設 19999)"
                echo "  --version <版本>    sing-box 版本, 如 1.13.14 (預設 latest)"
                echo "  --admin <用戶名>    管理用戶 (預設當前用戶)"
                echo ""
                echo "示例:"
                echo "  # 一條命令完整部署（全部默認值，自動最新版）"
                echo "  curl -sL https://raw.githubusercontent.com/whypuss/y-ui/main/install.sh | sudo bash -s -- --full"
                echo ""
                echo "  # 指定 sing-box 版本 + 面板端口"
                echo "  sudo bash install.sh --full --version 1.13.14 --port 19999 --admin root"
                echo ""
                echo "  # 僅在已有 sing-box 的機子上裝面板"
                echo "  sudo bash install.sh --panel --port 19999"
                exit 0 ;;
            --port)
                PANEL_PORT="$2"; shift 2 ;;
            --version)
                SINGBOX_VERSION="$2"; shift 2 ;;
            --admin)
                ADMIN_USER="$2"; shift 2 ;;
            --quiet)
                QUIET="1"; shift ;;
            *)
                # 舊用法兼容: 位置參數 (第一個非標記參數當做模式, 第二個當做 sing-box 版本)
                if [ -z "$DEPLOY_MODE" ]; then
                    DEPLOY_MODE="$1"; shift
                elif [ "$DEPLOY_MODE" = "full" ] && [ -z "$SINGBOX_VERSION" ]; then
                    SINGBOX_VERSION="$1"; shift
                else
                    err "未知參數: $1 (運行 --help 查看用法)"
                    exit 1
                fi ;;
        esac
    done

    # ---- 默認值 ----
    SINGBOX_VERSION="${SINGBOX_VERSION:-latest}"
    ADMIN_USER="${ADMIN_USER:-$(whoami)}"

    case "$DEPLOY_MODE" in
        full|-f)
            do_full_deploy ;;
        panel|-p)
            do_panel_deploy ;;
        uninstall|-u)
            do_uninstall ;;
        *)
            if [ -z "$DEPLOY_MODE" ]; then
                echo "未指定部署模式, 運行 --help 查看用法。默認: 僅部署 y-ui 面板"
                echo ""
            fi
            do_panel_deploy ;;
    esac
}

main "$@"
