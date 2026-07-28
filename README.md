# y-ui

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat)](https://github.com/whypuss/y-ui)
[![GitHub](https://img.shields.io/github/v/tag/whypuss/y-ui?style=flat)](https://github.com/whypuss/y-ui)

**Sing-Box 輕量級 Web 控制面板** — Go 編寫，單文件靜態編譯，零依賴部署。

面向旁路由（NAT 機）場景，提供一鍵管理 TUN / iptables / TProxy，以及一鍵獲取標準節點分享連結。

---

## 一鍵部署

### 方式一：一條命令完整部署（安裝 sing-box + y-ui）

```bash
curl -sL https://raw.githubusercontent.com/whypuss/y-ui/main/install.sh | sudo bash -s -- --full
```

安裝過程中會提示選擇 sing-box 版本（直接 Enter 用最新穩定版）。腳本自動：
- 檢測 Linux 發行版（Ubuntu/Debian/CentOS/Alpine）和 CPU 架構
- 安裝依賴（curl, iptables, iproute2, git, sudo）
- 下載安裝 sing-box
- 生成默認 `config.json`（AnyTLS + SOCKS + Mixed inbound）
- 安裝 systemd service（sing-box + y-ui）
- 配置 sudoers 權限（重啟 sing-box / iptables / kill）
- 開放防火牆端口
- 啟動服務

```bash
# 指定 sing-box 版本
sudo bash install.sh --full 1.13.14
```

### 方式二：在現有 sing-box 上部署 y-ui

```bash
curl -sL https://raw.githubusercontent.com/whypuss/y-ui/main/install.sh | sudo bash -s -- --panel
```

### 解除安裝

```bash
sudo bash install.sh --uninstall
```

> **注意：** 由於目前 repo 暫無 GitHub Release，初次部署 y-ui 時腳本會提示上傳 `/tmp/y-ui` binary（或用本地編譯），之後創建 Release 加附 binary 附件即可完全一條命令自動部署。

---

## 功能

### 一鍵開關 — 修復網絡出問題

| 功能 | 說明 |
|------|------|
| **TUN 開關** | 一鍵開啟/關閉 sing-box TUN 主進程（控制 `auto_route`） |
| **TProxy 開關** | 一鍵開啟/關閉 iptables TProxy 重定向規則 |
| **iptables 配置** | 填寫參數、勾選規則、按 Apply 生效；獨立 MASQUERADE / FORWARD / DNS 控制 |
| **iptables 清除** | 一鍵清除 TProxy 規則（**保留網關基礎 FORWARD + MASQUERADE**，不會斷網） |
| **sing-box 重啟** | 重啟主進程（`systemctl restart sing-box`） |
| **實時狀態** | 自動輪詢 sing-box / TUN / TProxy / 外網連接狀態 |

> TUN 和 TProxy **完全獨立**，互不衝突。網關基礎規則（FORWARD + MASQUERADE）**永不被刪除**，任何操作下都不會斷網。

### 一鍵獲取節點

面板內置節點頁，自動讀取 `config.json` 生成標準分享連結，一鍵複製：

| 協議 | 連結格式 |
|------|---------|
| **AnyTLS** | `anytls://UUID@IP:Port/?insecure=1` |
| **HY2 (Hysteria 2)** | `hysteria2://password@IP:Port/?insecure=1` |
| **SS (Shadowsocks)** | `ss://base64(method:password@IP:Port)` |

每個協議支持：
- **獲取節點**：自動讀取密碼/UUID，生成標準連結
- **更新密碼/UUID**：生成新密碼 → 寫入 `config.json` → 重啟 sing-box，確保密碼一致
- **內部端口 / 映射端口**：NAT 機可填映射端口，留空則直接用內部端口
- **公網 IP 自動獲取**：首次加載自動讀取外網 IP

---

## 訪問

```
http://<VPS_IP>:19999/
```

---

## 技術架構

| 層 | 技術 |
|----|------|
| 語言 | **Go 1.21+**，靜態編譯，單文件部署 |
| 後端 | Go 標準庫 `net/http`，REST API (`POST /api`)，JSON 響應 |
| 前端 | 內嵌原生 HTML/CSS/JS，無框架，sidebar 佈局 |
| 依賴 | **零依賴**，無需 Python / Node / Docker / 數據庫 |
| 架構 | amd64 / arm64 / arm 交叉編譯 |

---

## 項目結構

```
y-ui/
├── cmd/main.go            # 程序入口
├── internal/
│   ├── web/
│   │   └── server.go      # HTTP 服務器 + API 路由 + 前端模板
│   └── exec/
│       ├── singbox.go     # sing-box 進程管理、TUN/TProxy 狀態
│       ├── nodes.go       # 節點生成（AnyTLS/HY2/SS）
│       └── iptables.go    # iptables 規則管理
├── install.sh             # 一鍵部署腳本
├── deploy.sh              # 舊版 SSH 部署腳本（已廢棄）
├── go.mod
└── README.md
```

---

## 安全與權限

- 默認綁定 `0.0.0.0:19999`，通過 NAT/內網訪問
- 無登入機制（設計目標：旁路由內網環境）
- 所有系統管理操作通過 sudoers 配置（NOPASSWD），無需交互式輸入密碼
- 建議在公開網絡前加認證層（nginx + basic auth / VPN 隧道）

---

## 編譯

```bash
GOOS=linux GOARCH=amd64 go build -trimpath -o y-ui ./cmd
GOOS=linux GOARCH=arm64 go build -trimpath -o y-ui ./cmd
```

---

## License

MIT
