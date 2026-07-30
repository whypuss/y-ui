# y-ui — Manual Installation Guide / 手動安裝指南

> **What y-ui is:** A lightweight Web control panel for sing-box, written in Go.
> It is a **single static binary** — no Docker, no Node.js, no Python, no database needed.
>
> **What y-ui does:** Provides a web UI (port 19999) to manage sing-box traffic routing
> (TUN / TProxy / iptables), generate node share links, and restart services.
> It does **not** route traffic itself — that is sing-box's job.
>
> **Who this guide is for:** Anyone with basic Linux CLI skills.
> Target systems: Ubuntu 20+, Debian 11+, CentOS 8+, Alpine Linux, OpenWRT.

---

## Table of Contents / 目錄

1. [Requirements](#1-requirements)
2. [Pre-flight Check](#2-pre-flight-check)
3. [Quick Install (one command)](#3-quick-install-one-command)
4. [Manual Install (step by step)](#4-manual-install-step-by-step)
5. [Configure Sing-Box](#5-configure-sing-box)
6. [Configure sudoers](#6-configure-sudoers)
7. [Start Services](#7-start-services)
8. [Verify Deployment](#8-verify-deployment)
9. [Access the Panel](#9-access-the-panel)

---

## 1. Requirements / 系統要求

### Hardware

| Item | Minimum | Recommended |
|------|---------|-------------|
| CPU | 1 core | 2+ cores |
| RAM | 256 MB | 512 MB+ |
| Disk | 100 MB free | 500 MB+ |
| Arch | x86_64 / arm64 / arm | — |

### Software

| Item | Required | How to check |
|------|----------|--------------|
| Linux kernel | 4.x+ | `uname -r` |
| systemd or OpenRC | one of | `systemctl --version` |
| iptables / ip6tables | yes | `iptables --version` |
| iproute2 (ip command) | yes | `ip -v` |
| curl or wget | yes | `curl --version` |
| sudo | yes | `sudo -V` |
| bash | yes | `bash --version` |
| python3 (for config tools) | optional | `python3 --version` |

**y-ui itself does NOT need Go, Node.js, or Docker to run.**
Only the person building from source needs Go 1.21+.

---

## 2. Pre-flight Check / 安裝前檢查

Run this script to verify your system before installing:

```bash
curl -sL https://raw.githubusercontent.com/whypuss/y-ui/main/scripts/check.sh | sudo bash
```

Or check manually:

```bash
# Check 1: root access
id -u

# Check 2: iptables
iptables -V

# Check 3: iproute2
ip rule show

# Check 4: curl / wget
curl -s --connect-timeout 5 https://github.com >/dev/null && echo "Network OK"

# Check 5: systemd
systemctl --version 2>/dev/null || echo "No systemd"

# Check 6: sudo NOPASSWD (for current user)
sudo -n true && echo "sudo OK"
```

---

## 3. Quick Install (one command) / 一鍵安裝

The fastest way to get running. Installs both sing-box and y-ui.

```bash
# Option A: Full install (sing-box + y-ui)
curl -sL https://raw.githubusercontent.com/whypuss/y-ui/main/scripts/install.sh | sudo bash -s -- --full

# Option B: y-ui only (sing-box already installed)
curl -sL https://raw.githubusercontent.com/whypuss/y-ui/main/scripts/install.sh | sudo bash -s -- --panel

# Option C: Specify sing-box version
sudo bash scripts/install.sh --full --version 1.13.14
```

**What the script does automatically:**

1. Detects Linux distro (Ubuntu/Debian/CentOS/Alpine/OpenWRT)
2. Detects CPU architecture (amd64/arm64/arm)
3. Installs system dependencies
4. Downloads sing-box binary
5. Generates default `config.json`
6. Installs systemd service files
7. Configures sudoers
8. Starts services
9. Shows the web panel URL

**Uninstall:**

```bash
sudo bash scripts/install.sh --uninstall
```

---

## 4. Manual Install (step by step) / 手動安裝

If you prefer full control, follow these steps.

### Step 1: Install system dependencies

**Ubuntu / Debian:**

```bash
sudo apt update
sudo apt install -y curl wget iptables iproute2 sudo python3
```

**CentOS / RHEL:**

```bash
sudo dnf install -y curl wget iptables iproute2 sudo python3
```

**Alpine Linux:**

```bash
sudo apk add curl wget iptables iproute2 sudo python3
```

### Step 2: Create directories

```bash
sudo mkdir -p /opt/y-ui
sudo mkdir -p /etc/sing-box
sudo mkdir -p /etc/sing-box/conf
sudo mkdir -p /etc/systemd/system
```

### Step 3: Install sing-box binary

Download the latest sing-box for your architecture:

```bash
# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)  BINARCH="amd64" ;;
    aarch64|arm64) BINARCH="arm64" ;;
    armv7l)        BINARCH="arm" ;;
    *)             BINARCH="amd64" ;;
esac

# Download sing-box
cd /tmp
curl -sL "https://github.com/SagerNet/sing-box/releases/latest/download/sing-box-linux-${BINARCH}.tar.xz" -o sing-box.tar.xz
tar -xf sing-box.tar.xz
sudo mkdir -p /etc/sing-box/bin
sudo mv sing-box /etc/sing-box/bin/
```

### Step 4: Place y-ui binary

**If using GitHub Release** (recommended):

```bash
# Download the y-ui binary from releases
curl -sL "https://github.com/whypuss/y-ui/releases/latest/download/y-ui-linux" -o /opt/y-ui/y-ui
sudo chmod +x /opt/y-ui/y-ui
```

**If building from source** (need Go 1.21+):

```bash
git clone https://github.com/whypuss/y-ui.git
cd y-ui
GOOS=linux GOARCH=amd64 go build -trimpath -o y-ui ./cmd
sudo cp y-ui /opt/y-ui/
sudo chmod +x /opt/y-ui/y-ui
```

### Step 5: Copy config template

```bash
# Copy the example configs (provided in this repo)
sudo cp config/sing-box-config.json   /etc/sing-box/config.json
sudo cp config/us-proxy.json          /etc/sing-box/us-proxy.json
sudo cp config/jp-proxy.json          /etc/sing-box/jp-proxy.json
sudo cp config/ip-rules.sh            /etc/sing-box/ip-rules.sh

# Edit config.json — change UUIDs, passwords, ports to your needs
sudo nano /etc/sing-box/config.json
```

> ⚠️ **Important:** Do not use the example UUIDs/passwords in production.
> Generate your own UUID: `cat /proc/sys/kernel/random/uuid`
> Generate a random password: `openssl rand -base64 24`

### Step 6: Configure secrets (separate from config)

Create a file for sensitive values that should never be committed to git:

```bash
sudo mkdir -p /opt/y-ui/secrets
sudo nano /opt/y-ui/secrets/sudo-pass
# Write your sudo password inside (for panel to execute sudo commands)
```

### Step 7: Install systemd services

```bash
# Copy service files
sudo cp systemd/sing-box.service      /etc/systemd/system/
sudo cp systemd/sing-box-us.service   /etc/systemd/system/
sudo cp systemd/sing-box-jp.service   /etc/systemd/system/
sudo cp systemd/y-ui.service          /etc/systemd/system/

# Reload systemd
sudo systemctl daemon-reload
```

### Step 8: Configure sudoers

```bash
sudo cp sudoers/y-ui /etc/sudoers.d/y-ui
sudo chmod 440 /etc/sudoers.d/y-ui
```

This gives the y-ui panel permission to run `systemctl`, `iptables`, `ip`,
`pkill`, and `kill` without prompting for a password.

### Step 9: Enable IP forwarding (required for routing)

```bash
echo "net.ipv4.ip_forward = 1" | sudo tee /etc/sysctl.d/99-yui-forward.conf
sudo sysctl -p /etc/sysctl.d/99-yui-forward.conf
```

---

## 5. Configure Sing-Box / 配置 Sing-Box

### Basic config structure

```json
{
  "log": { "level": "info" },
  "inbounds": [
    { "type": "vless",   "listen_port": 443,   "tag": "VLESS-REALITY-443" },
    { "type": "vless",   "listen_port": 8894,  "tag": "VLESS-WS-8894" },
    { "type": "mixed",   "listen_port": 10808, "tag": "TProxy-Mixed-10808" },
    { "type": "socks",   "listen_port": 1080,  "tag": "SOCKS5-1080" },
    { "type": "anytls",  "listen_port": 17777, "tag": "ANYTLS-17777" },
    { "type": "hysteria2", "listen_port": 20000, "tag": "HY2-20000" },
    { "type": "shadowsocks", "listen_port": 20001, "tag": "SS-20001" },
    { "type": "tun",     "auto_route": true,   "tag": "TUN" }
  ],
  "outbounds": [
    { "type": "socks", "server": "127.0.0.1", "server_port": 10810, "tag": "us-proxy" },
    { "type": "socks", "server": "127.0.0.1", "server_port": 10811, "tag": "proxy-chain" }
  ],
  "route": { "final": "direct" }
}
```

### Key rule: Do not modify the TUN inbound

The `type: "tun"` inbound in `config.json` must **never be deleted or replaced**.
Removing it causes traffic to fail silently. y-ui's panel now blocks this
operation automatically (see `TUN-RULE.md`).

---

## 6. Configure sudoers / 配置 sudoers

The file `/etc/sudoers.d/y-ui` should contain:

```
%maxwell ALL=(ALL) NOPASSWD: /usr/bin/systemctl, /usr/sbin/iptables, /usr/sbin/ip, /usr/bin/pkill, /usr/bin/kill, /usr/bin/killall
```

Replace `maxwell` with the user who runs y-ui.

**Why sudoers is needed:** y-ui calls system commands (iptables, ip rule,
systemctl restart) to manage traffic rules. sudoers with NOPASSWD lets
the web panel execute these without interactive password prompts.

---

## 7. Start Services / 啟動服務

### Start order (must follow this sequence)

```bash
# 1. Start us-proxy (AnyTLS outbound, listens :10810)
sudo systemctl enable --now sing-box-us.service

# 2. Start jp-proxy (VLESS-REALITY outbound, listens :10811)
sudo systemctl enable --now sing-box-jp.service

# 3. Start main sing-box (depends on 10810/10811 being ready)
sudo systemctl enable --now sing-box.service

# 4. Start y-ui panel
sudo systemctl enable --now y-ui.service
```

### Verify all services are running

```bash
sudo systemctl status sing-box-us sing-box-jp sing-box y-ui
```

All four should show `active (running)`.

### Check all ports are listening

```bash
ss -tlnp | grep -E "443|8894|1080|10808|17777|20000|20001|10810|10811|19999"
```

### Check network connectivity

```bash
# Direct internet access
curl -s --max-time 8 -o /dev/null -w "%{http_code}\n" https://www.google.com

# Via us-proxy (10810)
curl -s --max-time 8 --socks5 127.0.0.1:10810 https://www.google.com -o /dev/null -w "%{http_code}\n"

# Via jp-proxy (10811)
curl -s --max-time 8 --socks5 127.0.0.1:10811 https://www.google.com -o /dev/null -w "%{http_code}\n"
```

---

## 8. Verify Deployment / 驗證部署

Run the full check script:

```bash
curl -sL https://raw.githubusercontent.com/whypuss/y-ui/main/scripts/check.sh | sudo bash
```

Expected output:

```
[1/8] Go compiler ............. OK (or N/A — static binary)
[2/8] iptables ............... OK
[3/8] iproute2 ............... OK
[4/8] curl ................... OK
[5/8] systemd ............... OK
[6/8] sing-box binary ........ OK (/etc/sing-box/bin/sing-box)
[7/8] config.json ............ OK (/etc/sing-box/config.json)
[8/8] sudoers ............... OK (/etc/sudoers.d/y-ui)

Deployments ready.
```

---

## 9. Access the Panel / 訪問面板

Open in browser:

```
http://<SERVER_IP>:19999/
```

| Item | Value |
|------|-------|
| URL | `http://<SERVER_IP>:19999/` |
| Default port | 19999 |
| Auth | None (LAN/NAT environment) |
| Admin | The user who configured sudoers |

### If the panel is not accessible

1. Check the service is running:
   ```bash
   sudo systemctl status y-ui
   ```

2. Check the port is listening:
   ```bash
   ss -tlnp | grep 19999
   ```

3. Check firewall rules:
   ```bash
   sudo iptables -L INPUT -n
   ```

4. Open the port if needed:
   ```bash
   sudo iptables -A INPUT -p tcp --dport 19999 -j ACCEPT
   ```

---

## Maintenance Commands / 維護命令

```bash
# View panel logs
sudo journalctl -u y-ui -f

# View sing-box logs
sudo journalctl -u sing-box -f

# Restart all services (in correct order)
sudo systemctl restart sing-box-us sing-box-jp sing-box y-ui

# Stop all services
sudo systemctl stop sing-box y-ui sing-box-us sing-box-jp

# Quick status
sudo systemctl status sing-box sing-box-us sing-box-jp y-ui
```

---

## Known Limitations / 已知限制

1. **No authentication** — the panel has no login. Add nginx + basic auth or
   VPN if exposing to the internet.
2. **No database** — all state is in files (`/etc/sing-box/config.json`,
   `/opt/y-ui/iptables.json`).
3. **No HTTPS on the panel** — the panel uses HTTP. For HTTPS, put nginx in front.
4. **sing-box binary version** — y-ui does not enforce a specific sing-box version.
   Use 1.13.14 for best compatibility (especially on LXC containers).
