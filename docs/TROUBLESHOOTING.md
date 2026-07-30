# y-ui — Troubleshooting / 故障排查

> This guide covers common failures when installing or operating y-ui.
> y-ui manages sing-box through system commands (`systemctl`, `iptables`, `ip`).
> Most issues are environment/permission issues, not y-ui code issues.

---

## Table of Contents / 目錄

1. [General Diagnosis Script](#1-general-diagnosis-script)
2. [Service Will Not Start](#2-service-will-not-start)
3. [Panel Accessible but Cannot Control sing-box](#3-panel-accessible-but-cannot-control-sing-box)
4. [Network Broken After TUN / TProxy Changes](#4-network-broken-after-tun--tproxy-changes)
5. [Port Already in Use](#5-port-already-in-use)
6. [sing-box Binary Missing or Wrong Architecture](#6-sing-box-binary-missing-or-wrong-architecture)
7. [LXC / Container Problems](#7-lxc--container-problems)
8. [Node URL Generated but Client Cannot Connect](#8-node-url-generated-but-client-cannot-connect)
9. [Logs and Debug Output](#9-logs-and-debug-output)

---

## 1. General Diagnosis Script / 通用診斷

Run this first:

```bash
curl -sL https://raw.githubusercontent.com/whypuss/y-ui/main/scripts/check.sh | sudo bash
```

If you cannot download the script, copy it manually from `scripts/check.sh`.

The script checks:
- systemd / OpenRC
- iptables
- iproute2
- sing-box binary
- config files
- systemd services
- panel port
- connectivity

---

## 2. Service Will Not Start / 服務無法啟動

### Symptom

```bash
sudo systemctl status y-ui
```

Shows `failed`, `exit code 203`, or `cannot execute`.

### Common causes

| Cause | Fix |
|-------|-----|
| Binary not executable | `sudo chmod +x /opt/y-ui/y-ui` |
| Wrong architecture binary | Rebuild/download correct binary (`amd64` or `arm64`) |
| Port 19999 occupied | `ss -tlnp \| grep 19999` and stop the conflicting service |
| systemd unit missing | Copy from `systemd/y-ui.service` to `/etc/systemd/system/y-ui.service` |

### Commands

```bash
# Check executable bit
ls -l /opt/y-ui/y-ui

# Check systemd unit exists
sudo systemctl status y-ui.service

# Run directly to see error
sudo /opt/y-ui/y-ui --port 19999
```

---

## 3. Panel Accessible but Cannot Control sing-box / 面板可訪問但無法控制 sing-box

### Symptom

Web panel loads, but buttons for "Restart sing-box", "TProxy On", etc. fail.

### Common causes

| Cause | Fix |
|-------|-----|
| sudoers missing | Install `/etc/sudoers.d/y-ui` |
| Wrong user in sudoers | Replace username with actual panel user |
| sudo password required | Ensure `NOPASSWD` in sudoers or use `/tmp/.yui-pass` |

### Required sudoers entry

```bash
sudo cp /path/to/y-ui/sudoers/y-ui /etc/sudoers.d/y-ui
sudo chmod 440 /etc/sudoers.d/y-ui
```

Example content:

```
maxwell ALL=(ALL) NOPASSWD: /usr/bin/systemctl, /usr/sbin/iptables, /usr/sbin/ip, /usr/bin/pkill, /usr/bin/kill, /usr/bin/killall
```

### Test sudo manually

```bash
sudo -n systemctl restart sing-box
sudo -n iptables -L -n
```

If these fail with `sudo: a password is required`, sudoers is not configured correctly.

---

## 4. Network Broken After TUN / TProxy Changes / TUN/TProxy 後網絡中斷

### Symptom

LAN devices cannot access internet after enabling TUN or TProxy.

### Most common cause

TProxy and TUN rules conflict, or TUN `auto_route=true` causes all traffic
to be redirected into an inactive tun interface.

### Diagnosis

```bash
sudo ip rule show
sudo ip route show table all
sudo ss -tlnp | grep sing-box
sudo iptables -t mangle -L PREROUTING -n
```

### Recovery (if locked out remotely)

From console / SSH session if possible:

```bash
# 1. Disable TProxy rules
sudo iptables -t mangle -F PREROUTING
sudo ip rule del fwmark 0x1/0x1 lookup 100 2>/dev/null || true

# 2. Disable TUN routing rules
for p in 9000 9001 9002 9003 9004 9005 9006 9007 9008 9009 9010; do
    sudo ip rule del priority $p 2>/dev/null || true
done
sudo ip route flush table 2022 2>/dev/null || true
sudo ip link del tun0 2>/dev/null || true

# 3. Restore gateway forwarding rules
sudo bash /etc/sing-box/ip-rules.sh

# 4. Restart sing-box with auto_route=false
sudo sed -i 's/"auto_route": true/"auto_route": false/' /etc/sing-box/config.json
sudo systemctl restart sing-box
```

### Key lesson

Do not enable both TUN (`auto_route=true`) and TProxy at the same time unless
the routing rules are carefully separated. In production, one mode should be active.

---

## 5. Port Already in Use / 端口被占用

### Symptom

```
bind: address already in use
```

### Diagnosis

```bash
ss -tlnp | grep 19999
ss -tlnp | grep 10810
ss -tlnp | grep 10811
```

### Fix

Stop the conflicting process:

```bash
sudo lsof -i :19999
sudo systemctl stop <conflicting-service>
```

---

## 6. sing-box Binary Missing or Wrong Architecture / sing-box 二進制缺失或架構錯誤

### Symptom

```
failed to start sing-box: executable file not found
```

### Fix

```bash
# Check binary exists
ls -l /etc/sing-box/bin/sing-box

# Check architecture
file /etc/sing-box/bin/sing-box

# Download correct binary
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)  B="amd64" ;;
    aarch64|arm64) B="arm64" ;;
    armv7l)        B="arm" ;;
    *)             B="amd64" ;;
esac

curl -sL "https://github.com/SagerNet/sing-box/releases/latest/download/sing-box-linux-${B}.tar.xz" -o /tmp/sing-box.tar.xz
cd /tmp && tar -xf sing-box.tar.xz && sudo mv sing-box /etc/sing-box/bin/
```

---

## 7. LXC / Container Problems / LXC/容器問題

### Symptom

Sing-box starts, but TUN does not work. Or DNS fails.

### Known issues

| Problem | Cause | Fix |
|---------|-------|-----|
| No `/dev/net/tun` | Container lacks TUN device | Ask host to enable TUN |
| DNS fails | Container DNS blocked | Use host/gateway DNS, e.g. `10.0.3.1` |
| Legacy DNS format required | sing-box version mismatch | Run with `ENABLE_DEPRECATED_LEGACY_DNS_SERVERS=true` |

### LXC TUN fix

```bash
# On host, enable TUN for container
# Example for LXC:
sudo lxc config device set <container> tun unix-char path=/dev/net/tun

# Restart container
```

### DNS workaround

```bash
# Set gateway DNS in sing-box config:
# "dns": {
#   "servers": [{ "address": "10.0.3.1" }]
# }

# Or use systemd-resolved:
sudo ln -sf /run/systemd/resolve/stub-resolv.conf /etc/resolv.conf
```

---

## 8. Node URL Generated but Client Cannot Connect / 節點 URL 生成後客戶端無法連接

### Symptom

The panel generates a URL like `anytls://uuid@IP:17777/?insecure=1`,
but the client cannot connect.

### Checklist

1. **Is the server reachable?**
   ```bash
   ssh user@server "sudo ss -tlnp | grep 17777"
   ```

2. **Is the server public IP correct?**
   ```bash
   curl -s https://ipinfo.io/ip
   ```

3. **Is the port blocked by firewall?**
   ```bash
   sudo iptables -L INPUT -n | grep 17777
   ```

4. **Is the protocol supported by the client?**
   - VLESS-REALITY requires sing-box client or sclang-style parser
   - NEKOBOX does **not** support REALITY without full manual config
   - AnyTLS URL requires compatible client

5. **Are passwords/UUIDs consistent?**
   y-ui generates URLs from `/etc/sing-box/config.json`. If config changed manually,
   URLs may be outdated. Re-fetch from the panel.

---

## 9. Logs and Debug Output / 日誌和調試

### y-ui panel logs

```bash
sudo journalctl -u y-ui -f
```

### sing-box logs

```bash
sudo journalctl -u sing-box -f
sudo journalctl -u sing-box-us -f
sudo journalctl -u sing-box-jp -f
```

### Manual service restart logs

```bash
sudo systemctl status sing-box -l
```

### Config validation

```bash
# Validate config.json
sudo python3 -m json.tool /etc/sing-box/config.json >/dev/null && echo "config OK"
```

### Network debug

```bash
# Direct
curl -s -m 5 -w "%{http_code}\n" https://www.google.com -o /dev/null

# Proxy via panel
curl --socks5 127.0.0.1:10810 https://www.google.com -o /dev/null -w "%{http_code}\n"

# TUN rules
sudo ip rule show

# TProxy rules
sudo iptables -t mangle -L PREROUTING -n -v
```
