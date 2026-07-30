# y-ui — Troubleshooting Guide / 故障排查

> This guide covers real failure scenarios during and after y-ui deployment.
> Before asking for help, run `scripts/check.sh` — 80% of issues are detected there.

---

## Table of Contents / 目錄

1. [Sing-Box Service Will Not Start](#1-sing-box-service-will-not-start)
2. [Port Not Listening](#2-port-not-listening)
3. [No Internet After Enabling TUN](#3-no-internet-after-enabling-tun)
4. [Node Generation Fails or Produces Wrong URLs](#4-node-generation-fails-or-produces-wrong-urls)
5. [Service Crashes Repeatedly (systemd restart loop)](#5-service-crashes-repeatedly-systemd-restart-loop)
6. [Permission Denied: iptables / ip rule](#6-permission-denied-iptables--ip-rule)
7. [iptables Rules Vanish After Reboot](#7-iptables-rules-vanish-after-reboot)
8. [DNS Does Not Resolve](#8-dns-does-not-resolve)
9. [Config Validation Errors](#9-config-validation-errors)
10. [Multiple Sing-Box Instances Conflict](#10-multiple-sing-box-instances-conflict)
11. [Firewall Blocks the Panel](#11-firewall-blocks-the-panel)
12. [TProxy Rule Auto-Injection at Boot](#12-tproxy-rule-auto-injection-at-boot)
13. [Panel Shows "Not Connected" or Blank](#13-panel-shows-not-connected-or-blank)
14. [Uninstall / Clean Up](#14-uninstall--clean-up)

---

## 1. Sing-Box Service Will Not Start

### Symptom

```bash
systemctl status sing-box
# sing-box.service: Main process exited, code=exited, status=1/FAILURE
```

### Diagnosis

```bash
# Check the log for the actual error
journalctl -u sing-box.service -n 50 --no-pager

# Validate the config
/etc/sing-box/bin/sing-box check -c /etc/sing-box/config.json

# Check if port 443 or 10808 is already in use
ss -tlnp | grep -E "443|10808|8894"
```

### Common causes

- **Config syntax error**: `config.json` has invalid JSON. Fix with `sing-box check`.
- **Port conflict**: Another process already uses port 443 or 10808. Kill it or change the config port.
- **Wrong binary path**: `ExecStart` points to a non-existent file. Verify with:
  ```bash
  ls -la /etc/sing-box/bin/sing-box
  ```
- **Missing TUN device** (LXC only): `/dev/net/tun` does not exist. Create it:
  ```bash
  mkdir -p /dev/net && mknod /dev/net/tun c 10 200 && chmod 600 /dev/net/tun
  ```

---

## 2. Port Not Listening

### Symptom

```bash
ss -tlnp | grep 443
# (no output)
```

But service status says `active (running)`.

### Diagnosis

```bash
# Check what ports sing-box actually listens on
ss -tlnp | grep sing-box

# Check if the config file has the expected inbounds
grep listen_port /etc/sing-box/config.json
```

### Fix

- Verify the config uses `listen_port` (not `port`) — sing-box 1.11+ uses `listen_port`.
- If `ss` shows the port but another IP (e.g. `127.0.0.1:443` instead of `0.0.0.0:443`), the inbound `listen` field is set to localhost. Change to `0.0.0.0` or remove the `listen` field.

---

## 3. No Internet After Enabling TUN

### Symptom

- `curl www.google.com` hangs
- `ping 8.8.8.8` hangs
- But ping to the local gateway (e.g. `192.168.31.1`) works

### Root cause

When TUN is enabled, **ALL** outbound traffic is routed through sing-box. If sing-box has no working outbound node, traffic is sent into a black hole. This includes DNS and the server's own management traffic.

### Diagnosis

```bash
# Check TUN route rules (90xx)
ip rule list

# Check tun0 interface
ip addr show tun0

# Check sing-box outbound status
journalctl -u sing-box -n 20 --no-pager | grep -i "outbound\|error\|failed"

# Test with TUN disabled (temporary fix)
sudo bash scripts/disable-tproxy.sh
# Then try internet again
```

### Recovery — restore internet first

```bash
# Step 1: Disable TUN rule (remove 90xx routes)
ip rule del table 2022 2>/dev/null
ip rule del table 2021 2>/dev/null
ip route flush table 2022 2>/dev/null
ip route flush table 2021 2>/dev/null

# Step 2: Verify internet works
curl -s --max-time 8 https://www.google.com -o /dev/null -w "%{http_code}\n"
```

### Fix the root cause

1. Ensure sing-box config has at least ONE working outbound node.
2. If using port mapping, verify `nc -zv <VPS_IP> <PORT>` from the server.
3. For LXC: ensure `tun = "true"` in `/etc/lxc/<container>/config`.

---

## 4. Node Generation Fails or Produces Wrong URLs

### Symptom

- The panel shows "node generation failed"
- Or the generated URL does not work on the client

### Diagnosis

```bash
# Check if config.json exists
ls -la /etc/sing-box/config.json

# Verify JSON is valid
python3 -c "import json; json.load(open('/etc/sing-box/config.json'))" && echo "OK"

# Check if sing-box config has the expected inbound types
grep -E '"type"|"tag"' /etc/sing-box/config.json
```

### Common causes

- **UUID is a placeholder**: The config uses `UUID-CHANGE-THIS`. Generate a real one:
  ```bash
  cat /proc/sys/kernel/random/uuid
  ```
- **Secrets missing**: `anytls.key`, `tls.key`, `private_key`, `reality private_key` files are absent or empty.
- **Wrong protocol type**: The config uses an inbound type that y-ui does not know how to convert.
- **Port number mismatch**: The config port does not match the actual listening port.

---

## 5. Service Crashes Repeatedly (systemd restart loop)

### Symptom

```bash
systemctl status sing-box
# sing-box.service: Service Restarted 20 times in the last 3 minutes
```

### Diagnosis

```bash
journalctl -u sing-box -n 100 --no-pager | tail -30
```

Look for the crash line (usually `panic:`, `fatal error:`, or `signal 11`).

### Common causes

- **Config error**: `sing-box check` will catch most.
- **Binary version mismatch**: Config uses a feature from a newer sing-box than what's installed.
- **Out of memory**: On low-RAM boxes, sing-box may OOM. Check with `dmesg | grep -i oom`.

### Fix

1. `sing-box check -c /etc/sing-box/config.json` to validate.
2. If OOM, add swap: `fallocate -l 512M /swap && mkswap /swap && swapon /swap`.
3. If still crashing, revert to the last known-good config:
   ```bash
   ls /etc/sing-box/*.bak.* | sort -r | head -3
   cp /etc/sing-box/config.json.bak.LATEST /etc/sing-box/config.json
   systemctl restart sing-box
   ```

---

## 6. Permission Denied: iptables / ip rule

### Symptom

The panel tries to change firewall rules and returns "permission denied".

### Diagnosis

```bash
# Check if sudoers file exists
cat /etc/sudoers.d/y-ui

# Test sudo access for sing-box
sudo -n iptables -L -n 2>&1
sudo -n ip rule list 2>&1
```

### Fix

The sudoers file `/etc/sudoers.d/y-ui` must exist and be readable:

```
maxwell ALL=(ALL) NOPASSWD: /usr/sbin/iptables, /usr/bin/ip, /usr/bin/sing-box, /usr/sbin/ss, /usr/bin/curl
```

Replace `maxwell` with the actual user.

---

## 7. iptables Rules Vanish After Reboot

### Symptom

After reboot, `iptables -L` shows empty FORWARD/POSTROUTING rules.

### Diagnosis

```bash
# Check if ip-rules.sh has execute permission
ls -la /etc/sing-box/ip-rules.sh

# Check if iptables-persistent is installed
dpkg -l | grep iptables-persistent  # Debian/Ubuntu
rpm -q iptables-services             # CentOS/RHEL
```

### Fix

The `ip-rules.sh` script is triggered on boot. Make sure:

```bash
chmod +x /etc/sing-box/ip-rules.sh
```

For persistent rules on Debian/Ubuntu, install `iptables-persistent` and save:

```bash
iptables-save > /etc/iptables/rules.v4
```

For systemd, make sure `ip-rules.sh` is called from a boot unit (e.g. `sing-box.service`'s `ExecStartPre`).

---

## 8. DNS Does Not Resolve

### Symptom

```bash
curl www.google.com        # FAILS
curl 8.8.8.8               # WORKS (with -k for https)
ping 8.8.8.8               # WORKS
```

### Diagnosis

DNS is being routed through sing-box, but sing-box has no DNS resolver configured.

### Fix

1. Edit `/etc/sing-box/config.json`, add a DNS section:
   ```json
   "dns": {
     "servers": [
       { "tag": "cloudflare", "address": "https://1.1.1.1/dns-query" }
     ]
   }
   ```
2. Restart sing-box.

Alternatively, for LXC containers with UDP 53 restrictions, use the gateway DNS:

```bash
echo "nameserver 10.0.3.1" > /etc/resolv.conf
```

---

## 9. Config Validation Errors

### Symptom

```bash
sing-box check -c /etc/sing-box/config.json
# Error: unknown field "port"
# Error: invalid type for "server"
```

### Common errors

| Error | Cause | Fix |
|-------|-------|-----|
| `unknown field "port"` | Using old `port` instead of `listen_port` | Rename to `listen_port` |
| `invalid type for "server"` | Wrong data type in the outbound section | Check config against sing-box docs |
| `invalid tag` | Duplicate inbound tag names | Make all tags unique |
| `unknown field "tls_reality"` | Wrong field name for REALITY TLS | Use `tls: { reality: { enabled: true } }` |

### Tool

```bash
# Full validation
/etc/sing-box/bin/sing-box check -c /etc/sing-box/config.json
```

---

## 10. Multiple Sing-Box Instances Conflict

### Symptom

- `ss -tlnp` shows ports bound by two different `sing-box` processes
- One of the services fails to start because the port is already taken

### Diagnosis

```bash
# Check all sing-box processes
ps aux | grep sing-box | grep -v grep

# Check port ownership
ss -tlnp | grep -E "443|1080|10808"
```

### Fix

Only one `sing-box` process per port. The correct layout:

- `sing-box.service` → `config.json` → ports 443, 8894, 10808, 1080, 17777, 20000, 20001
- `sing-box-us.service` → `us-proxy.json` → port 10810 (SOCKS5 only)
- `sing-box-jp.service` → `jp-proxy.json` → port 10811 (SOCKS5 only)

If a third instance is running (e.g. leftover from a different install), stop it:

```bash
systemctl stop sing-box-main
systemctl disable sing-box-main
```

---

## 11. Firewall Blocks the Panel

### Symptom

- `ss -tlnp` shows port 19999 listening
- But `curl http://<SERVER_IP>:19999` from another machine times out

### Diagnosis

```bash
# Check local firewall
sudo iptables -L INPUT -n | grep 19999

# Check ufw (Ubuntu)
sudo ufw status

# Check firewall-cmd (CentOS)
sudo firewall-cmd --list-ports
```

### Fix

**iptables:**
```bash
sudo iptables -I INPUT -p tcp --dport 19999 -j ACCEPT
```

**ufw:**
```bash
sudo ufw allow 19999/tcp
```

**firewall-cmd:**
```bash
sudo firewall-cmd --add-port=19999/tcp --permanent
sudo firewall-cmd --reload
```

**Cloud firewall:** If the server is behind a cloud provider (AWS, Hetzner, etc.), check the security group / cloud firewall rules and open port 19999.

---

## 12. TProxy Rule Auto-Injection at Boot

### Symptom

- TUN was working fine
- After reboot, `iptables -L -t tproxy` shows TPROXY rules that block TUN
- Internet is broken again

### Root cause

The `tproxy-rules.sh` service is marked as `active (exited)` but systemd's `Wants=` or `Restart=` causes it to re-run and inject TPROXY iptables rules.

### Fix

```bash
# Disable tproxy-rules service permanently
systemctl disable tproxy-rules
systemctl stop tproxy-rules

# Also disable the service file itself
rm -f /etc/systemd/system/tproxy-rules.service

systemctl daemon-reload

# Verify
systemctl status tproxy-rules
```

The rules file will still exist at `/etc/tproxy-rules.sh` but will not be executed on boot.

---

## 13. Panel Shows "Not Connected" or Blank

### Symptom

- The y-ui panel loads but shows "not connected" or a blank page
- The `GET /status` API returns empty or an error

### Diagnosis

```bash
# Test the API endpoint
curl http://localhost:19999/status

# Check if y-ui is listening
ss -tlnp | grep 19999

# Check y-ui logs
journalctl -u y-ui -n 50 --no-pager
```

### Fix

1. **y-ui not running**: `systemctl restart y-ui`
2. **y-ui crashed**: Check `journalctl -u y-ui -n 50` for the crash message.
3. **Frontend failed to load**: Clear browser cache, or open with `?force-refresh` to bypass.
4. **Port conflict**: Another process is on 19999. Change the port in the service:
   ```bash
   sed -i 's/--port 19999/--port 5588/' /etc/systemd/system/y-ui.service
   systemctl daemon-reload && systemctl restart y-ui
   ```

---

## 14. Uninstall / Clean Up

### Remove y-ui only

```bash
sudo bash scripts/install.sh --uninstall
```

This removes:
- `y-ui.service`
- `sing-box.service`, `sing-box-us.service`, `sing-box-jp.service`
- `/etc/sudoers.d/y-ui`
- `/opt/y-ui/`

Does NOT remove:
- `/etc/sing-box/` — keep for a clean sing-box-only setup
- Installed binaries

### Full removal (including sing-box)

```bash
sudo systemctl stop sing-box sing-box-us sing-box-jp y-ui
sudo systemctl disable sing-box sing-box-us sing-box-jp y-ui

sudo rm -rf /opt/y-ui
sudo rm -rf /etc/sing-box
sudo rm -f /etc/tproxy-rules.sh
sudo rm -f /etc/sudoers.d/y-ui

# Clean iptables rules
sudo bash /etc/sing-box/ip-rules.sh --clear 2>/dev/null
sudo iptables -F
sudo iptables -X
sudo ip6tables -F
sudo ip6tables -X
sudo ip rule del table 2022 2>/dev/null
sudo ip rule del table 2021 2>/dev/null
sudo ip route flush table 2022 2>/dev/null
sudo ip route flush table 2021 2>/dev/null
```

### Remove TUN device

```bash
ip link set tun0 down 2>/dev/null
ip link delete tun0 2>/dev/null
```

---

> ⚠️ **Before changing any rules manually**, always run `scripts/check.sh` and `scripts/status.sh` to understand the current state.
>
> When in doubt, revert to backup config:
> ```bash
> ls /etc/sing-box/*.bak.* | sort -r | head -3
> cp /etc/sing-box/config.json.bak.LATEST /etc/sing-box/config.json
> systemctl restart sing-box
> ```
