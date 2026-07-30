# y-ui — Architecture / 架構說明

> y-ui is a **static-compiled Go binary**. No Docker, no Node.js, no Python, no database.
> y-ui 係一個 **Go 靜態編譯二進制**——唔需要 Docker、Node.js、Python 或數據庫。
>
> It is a thin control panel that generates REST API endpoints to manage a separate **sing-box** process.
> 佢係一個輕量控制面板，透過 REST API 管理獨立嘅 **sing-box** 進程。

---

## 1. Technology Stack / 技術棧

| Layer | Tech |
|-------|------|
| Language | Go 1.21+ |
| Build | `go build -trimpath -o y-ui ./cmd` → single static binary |
| HTTP Server | Go `net/http` stdlib, embedded HTML (no framework) |
| OS Interaction | `os/exec.Command` → sudo → iptables / ip rule / systemctl |
| Target | Linux (amd64 / arm64 / arm), systemd or OpenRC |

**y-ui does not proxy traffic.** It only manages:
**y-ui 唔代理流量**，佢只管理：
- sing-box process lifecycle
- iptables / TProxy rules
- TUN policy routing
- Node URL generation (reading `/etc/sing-box/config.json`)

**sing-box** does the actual traffic routing.
**sing-box** 負責實際嘅流量路由。

---

## 2. Component Map / 組件圖

```
┌──────────────────────────────────────────────┐
│                  Linux (VPS / NAT)            │
│                                              │
│  /opt/y-ui/                                   │
│  ├── y-ui                          (binary)  │
│  │      └── 0.0.0.0:19999   (Web Panel API) │
│  │         └── POST /api  → management       │
│  │         └── GET  /status → health check   │
│  │                                              │
│  systemd:                                       │
│  ├── y-ui.service        (runs /opt/y-ui/y-ui)│
│  ├── sing-box.service    (main config.json)   │
│  ├── sing-box-us.service (us-proxy.json)      │
│  └── sing-box-jp.service (jp-proxy.json)      │
│                                               │
│  /etc/sing-box/                               │
│  ├── config.json         (7 inbounds + TUN)   │
│  ├── us-proxy.json       (AnyTLS outbound)    │
│  ├── jp-proxy.json       (VLESS-REALITY)      │
│  ├── ip-rules.sh         (LAN/lo bypass)      │
│  ├── anytls.key          (EXCLUDED from git)  │
│  └── anytls.pem          (EXCLUDED from git)  │
│                                               │
│  sudoers: /etc/sudoers.d/y-ui                │
│  config:  /opt/y-ui/iptables.json            │
└──────────────────────────────────────────────┘
```

## 3. Runtime Architecture / 運行時架構

```
LAN devices (192.168.31.0/24)
    │
    │  default GW = Maxwell (192.168.31.55)
    │
    ▼
┌───────────────────────────────────────┐
│  Maxwell (192.168.31.55)              │
│                                       │
│  ├─ Web Panel: 0.0.0.0:19999          │
│  │     (y-ui HTTP server)             │
│  │                                     │
│  ├─ sing-box main (config.json)       │
│  │   inbounds:                        │
│  │   • VLESS-REALITY :443             │
│  │   • VLESS-WS      :8894            │
│  │   • SOCKS5        :1080            │
│  │   • TProxy-Mixed  :10808           │
│  │   • AnyTLS        :17777           │
│  │   • HY2 (UDP)     :20000           │
│  │   • SS            :20001           │
│  │   • TUN tun0      auto_route       │
│  │   outbounds:                       │
│  │   • socks → 127.0.0.1:10810       │
│  │   • socks → 127.0.0.1:10811       │
│  │                                     │
│  ├─ us-proxy (us-proxy.json)          │
│  │   socks :10810 → AnyTLS → VPS      │
│  │                                     │
│  └─ jp-proxy (jp-proxy.json)          │
│      socks :10811 → VLESS-REALITY→VPS │
│                                       │
│  Traffic flow (TProxy mode):          │
│  LAN TCP 80/443 → iptables TPROXY     │
│    → 10808 → sing-box route            │
│    → socks 10810 or 10811              │
│    → Internet                          │
│                                       │
│  Traffic flow (TUN mode):             │
│  LAN all → ip rule 9001 (table 2022)  │
│    → tun0 → sing-box route             │
│    → Internet                          │
└───────────────────────────────────────┘
```

## 4. Directory Layout / 目錄結構

| Path | Purpose |
|------|---------|
| `/opt/y-ui/` | y-ui binary + iptables.json |
| `/opt/y-ui/y-ui` | Main binary |
| `/etc/sing-box/` | sing-box config directory |
| `/etc/sing-box/config.json` | Main process config (7 inbounds + TUN) |
| `/etc/sing-box/us-proxy.json` | us-proxy AnyTLS outbound |
| `/etc/sing-box/jp-proxy.json` | jp-proxy VLESS-REALITY outbound |
| `/etc/sing-box/conf/` | Additional config files (loaded via `conf/`) |
| `/etc/systemd/system/` | systemd service files |
| `/etc/sudoers.d/y-ui` | NOPASSWD sudo rules for y-ui |
| `/etc/sysctl.d/99-yui-forward.conf` | `ip_forward = 1` |

## 5. systemd Services (start order) / 服務啟動順序

```
1. sing-box-us.service   (us-proxy, listens :10810)
2. sing-box-jp.service   (jp-proxy, listens :10811)
3. sing-box.service      (main config, depends on 10810/10811)
4. y-ui.service          (web panel, manages 1-3)
```

## 6. Key Paths (hardcoded in y-ui source) / 關鍵路徑

| Path | Usage | Source |
|------|-------|--------|
| `/etc/sing-box/config.json` | Main config read/write | `nodes.go`, `singbox.go` |
| `/etc/sing-box/us-proxy.json` | AnyTLS outbound config | `nodes.go` |
| `/etc/sing-box/jp-proxy.json` | VLESS-REALITY outbound | `nodes.go` |
| `/etc/sing-box/conf/*.json` | Additional inbounds | `nodes.go` |
| `/etc/tproxy-rules.sh` | TProxy rules script | `singbox.go` |
| `/opt/y-ui/iptables.json` | iptables config state | `iptables.go` |

## 7. Security Model / 安全模型

- **No auth** on the panel (NAT / LAN environment design)
- System commands use **sudo** with NOPASSWD via `/etc/sudoers.d/y-ui`
- Secrets (`.key`, `.pem`, `private_key`, `public_key`) are **NOT** in git
- For internet exposure: add nginx + basic auth or VPN

## 8. Ports / 端口

| Port | Service | Protocol |
|------|---------|----------|
| 19999 | y-ui Web Panel | TCP |
| 443 | VLESS-REALITY | TCP |
| 8894 | VLESS-WS | TCP |
| 1080 | SOCKS5 | TCP |
| 10808 | TProxy-Mixed | TCP |
| 17777 | AnyTLS | TCP |
| 20000 | HY2 | UDP |
| 20001 | SS | TCP |
| 10810 | us-proxy SOCKS | TCP |
| 10811 | jp-proxy SOCKS | TCP |
