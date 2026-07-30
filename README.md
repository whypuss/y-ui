# y-ui

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat)](https://github.com/whypuss/y-ui)
[![GitHub](https://img.shields.io/github/v/tag/whypuss/y-ui?style=flat)](https://github.com/whypuss/y-ui)

**Sing-Box 輕量級 Web 控制面板** — Go 編寫，單文件靜態編譯，零依賴部署。

面向旁路由（NAT 機）場景，提供一鍵管理 TUN / iptables / TProxy，以及一鍵獲取標準節點分享連結。

> ⚠️ **What y-ui is not:** This is NOT a Docker project. NOT a Node.js project.
> It is a **Go static binary** that manages sing-box via system commands (systemctl, iptables, ip rule).
> Deploy on a bare Linux VPS or router — no container, no database, no runtime needed.

---

## Quick Start

```bash
curl -sL https://raw.githubusercontent.com/whypuss/y-ui/main/scripts/install.sh | sudo bash -s -- --full
```

Full install detects distro/arch, downloads sing-box, configures services, and starts the panel at `http://<SERVER_IP>:19999/`.

Other modes:

```bash
# y-ui only (sing-box already installed)
curl -sL https://raw.githubusercontent.com/whypuss/y-ui/main/scripts/install.sh | sudo bash -s -- --panel

# Uninstall
curl -sL https://raw.githubusercontent.com/whypuss/y-ui/main/scripts/install.sh | sudo bash -s -- --uninstall
```

---

## Documentation

| Document | What it covers |
|----------|----------------|
| [docs/INSTALL.md](docs/INSTALL.md) | Full step-by-step manual installation |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Project architecture, runtime, port map, secrets model |
| [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) | Common failures, diagnosis, recovery commands |

Deployment scripts:

| Script | Usage |
|--------|-------|
| `scripts/install.sh` | Step-by-step install with `--full` / `--panel` / `--uninstall` |
| `scripts/check.sh` | Pre-flight environment check |
| `scripts/status.sh` | Show all service status |
| `scripts/start.sh` / `scripts/stop.sh` | Start / stop all services |
| `scripts/restart.sh` | Restart all services in correct order |
| `scripts/logs.sh` | Live service logs |

---

## Features

### Traffic Control
- **TUN / TProxy toggle** — one-click enable/disable, fully independent modes
- **iptables config** — MASQUERADE / FORWARD / DNS forwarding, never deletes gateway base rules
- **sing-box restart** — auto-detects `sing-box.service` vs `sing-box-main.service`

### Node Share Links
Auto-generates standard share links from `/etc/sing-box/config.json`:

| Protocol | Format |
|----------|--------|
| **AnyTLS** | `anytls://UUID@IP:Port/?insecure=1` |
| **HY2** | `hysteria2://password@IP:Port/?insecure=1` |
| **SS** | `ss://base64(method:password@IP:Port)` |

### Real-time Status
Auto-polls sing-box / TUN / TProxy / external connectivity.

---

## Access

```
http://<SERVER_IP>:19999/
```

No authentication (LAN / NAT environment design). Add nginx + basic auth or VPN before exposing to the internet.

---

## Tech Stack

| Layer | Tech |
|-------|------|
| Language | **Go 1.21+**, static binary |
| Backend | Go stdlib `net/http`, REST API (`POST /api`) |
| Frontend | Embedded HTML/CSS/JS, no framework |
| Runtime | systemd / OpenRC |
| Dependency | **Zero** — no Python / Node / Docker / DB |

---

## Build

```bash
GOOS=linux GOARCH=amd64 go build -trimpath -o y-ui ./cmd
GOOS=linux GOARCH=arm64 go build -trimpath -o y-ui ./cmd
```

---

## Project Structure

```
y-ui/
├── cmd/main.go
├── internal/
│   ├── web/      # HTTP server + API + frontend template
│   └── exec/     # sing-box, node generation, iptables
├── docs/         # INSTALL / ARCHITECTURE / TROUBLESHOOTING
├── scripts/      # install / check / start / stop / status / logs
├── config/       # Configuration templates (placeholder values)
├── systemd/      # systemd unit templates
├── sudoers/      # sudoers template
├── go.mod
└── README.md
```

---

## License

MIT
