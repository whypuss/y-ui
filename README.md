# y-ui

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat)](https://github.com/whypuss/y-ui)
[![GitHub](https://img.shields.io/github/v/tag/whypuss/y-ui?style=flat)](https://github.com/whypuss/y-ui)

**Sing-Box 輕量級 Web 控制面板** — Go 編寫，單文件靜態編譯，零依賴部署。

面向旁路由（NAT 機）場景，提供一鍵管理 TUN / iptables / TProxy，以及一鍵獲取標準節點分享連結。

> ⚠️ **y-ui 唔係咩：** 唔係 Docker 項目，唔係 Node.js 項目。佢係 **Go 靜態編譯二進制**，透過系統命令（systemctl / iptables / ip rule）管理 sing-box。部署喺 Linux VPS 或路由器上——唔需要容器、數據庫或運行時環境。

---

## Quick Start / 快速開始

```bash
curl -sL https://raw.githubusercontent.com/whypuss/y-ui/main/scripts/install.sh | sudo bash -s -- --full
```

完整安裝會自動檢測發行版和架構，下載 sing-box，配置服務，啟動面板（`http://<SERVER_IP>:19999/`）。

### 安裝參數

| 參數 | 說明 | 預設 |
|------|------|------|
| `--full` | **完整安裝**。自動下載 sing-box 二進制、生成配置文件、安裝 systemd 服務、配置 sudoers，最後啟動全部服務。一次性搞定全部依賴。 | — |
| `--panel` | **僅安裝 y-ui**。適合已經有 sing-box 嘅機器，只裝控制面板同服務管理腳本，唔會覆蓋現有 sing-box 配置。 | — |
| `--port` | **面板端口**。控制 y-ui Web 面板嘅監聽端口，預設 19999。如果端口被佔用或者想換一個，就加呢個參數。 | `19999` |
| `--version` | **sing-box 版本**。指定要下載嘅 sing-box 版本號，例如 `1.13.14`。留空自動用最新版。LXC 容器建議用 `1.13.14`。 | `latest` |
| `--admin` | **運行用戶**。指定用邊個系統用戶運行 y-ui，同時寫入 sudoers 配置。唔填就用當前用戶。 | 當前用戶 |
| `--uninstall` | **解除安裝**。停止並刪除 y-ui 嘅 systemd 服務、sudoers 配置，保留 sing-box 數據。不會刪除 sing-box 本身。 | — |

### 常見組合

```bash
# 完整安裝，用 8888 端口
curl -sL https://raw.githubusercontent.com/whypuss/y-ui/main/scripts/install.sh | sudo bash -s -- --full --port 8888

# 完整安裝，指定 sing-box 1.13.14 + 自定義端口 + 指定用戶
curl -sL https://raw.githubusercontent.com/whypuss/y-ui/main/scripts/install.sh | sudo bash -s -- --full --port 8888 --version 1.13.14 --admin maxwell

# 已有 sing-box，只裝面板（換端口）
curl -sL https://raw.githubusercontent.com/whypuss/y-ui/main/scripts/install.sh | sudo bash -s -- --panel --port 5588

# 解除安裝
curl -sL https://raw.githubusercontent.com/whypuss/y-ui/main/scripts/install.sh | sudo bash -s -- --uninstall
```

其他方式：

### 僅安裝 y-ui（已有 sing-box）

```bash
curl -sL https://raw.githubusercontent.com/whypuss/y-ui/main/scripts/install.sh | sudo bash -s -- --panel
```

### 解除安裝

```bash
curl -sL https://raw.githubusercontent.com/whypuss/y-ui/main/scripts/install.sh | sudo bash -s -- --uninstall
```

---

## Documentation / 文檔

| 文檔 | 內容 |
|------|------|
| [docs/INSTALL.md](docs/INSTALL.md) | 手動安裝完整指南（分步式） |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | 項目架構、運行時、端口映射、Secrets 模型 |
| [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) | 常見故障、診斷、恢復命令 |

部署腳本：

| 腳本 | 用途 |
|------|------|
| `scripts/install.sh` | 分步安裝（`--full` / `--panel` / `--uninstall`） |
| `scripts/check.sh` | 環境檢查（安裝前） |
| `scripts/status.sh` | 查看所有服務狀態 |
| `scripts/start.sh` / `scripts/stop.sh` | 啟動 / 停止全部服務 |
| `scripts/restart.sh` | 按正確順序重啟全部服務 |
| `scripts/logs.sh` | 實時查看服務日誌 |
| `scripts/disable-tproxy.sh` | 關閉 TProxy 規則 |

---

## Features / 功能

### Traffic Control / 流量控制
- **TUN / TProxy 開關** — 一鍵開啟/關閉，兩種模式完全獨立
- **iptables 配置** — MASQUERADE / FORWARD / DNS 轉發，從不會刪除網關基礎規則（FORWARD + MASQUERADE）
- **sing-box 重啟** — 自動識別 `sing-box.service` vs `sing-box-main.service`

### Node Share Links / 節點分享連結
自動從 `/etc/sing-box/config.json` 生成標準分享連結：

| 協議 | 格式 |
|------|------|
| **AnyTLS** | `anytls://UUID@IP:Port/?insecure=1` |
| **HY2** | `hysteria2://password@IP:Port/?insecure=1` |
| **SS** | `ss://base64(method:password@IP:Port)` |

### Real-time Status / 實時狀態
自動輪詢 sing-box / TUN / TProxy / 外網連接狀態。

---

## Access / 訪問

```
http://<SERVER_IP>:19999/
```

無登入機制（設計目標：內網/NAT 環境）。暴露在公網前請加 nginx + basic auth 或 VPN。

---

## Tech Stack / 技術棧

| 層 | 技術 |
|----|------|
| 語言 | **Go 1.21+**，靜態編譯 |
| 後端 | Go 標準庫 `net/http`，REST API (`POST /api`) |
| 前端 | 內嵌 HTML/CSS/JS，無框架 |
| 運行時 | systemd / OpenRC |
| 依賴 | **零** — 無需 Python / Node / Docker / 數據庫 |

---

## Build / 編譯

```bash
GOOS=linux GOARCH=amd64 go build -trimpath -o y-ui ./cmd
GOOS=linux GOARCH=arm64 go build -trimpath -o y-ui ./cmd
```

---

## Project Structure / 項目結構

```
y-ui/
├── cmd/main.go
├── internal/
│   ├── web/      # HTTP server + API + frontend template
│   └── exec/     # sing-box, node generation, iptables
├── docs/         # INSTALL / ARCHITECTURE / TROUBLESHOOTING
├── scripts/      # install / check / start / stop / status / logs
├── config/       # 配置模板（占位符，無真實凭证）
├── systemd/      # systemd unit 模板
├── sudoers/      # sudoers 模板
├── go.mod
└── README.md
```

---

## License / 許可證

MIT
