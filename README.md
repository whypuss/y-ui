# y-ui

Sing-Box 輕量級 Web 控制面板 — 單文件，零依賴，標準庫，純 Python。

## 功能

| 功能 | 說明 |
|------|------|
| TProxy 開關 | 開啟/關閉 iptables TProxy 重定向規則 |
| iptables 清除 | 恢復系統默認網絡狀態 |
| sing-box 重啟 | 重啟主進程 (systemctl restart) |
| 實時狀態 | 自動輪詢 sing-box/TUN/TProxy 狀態 |

## 安裝

```bash
# 部署到 VPS
bash deploy.sh
```

手動部署：
```bash
# 1. 複製所有文件到目標目錄
cp -r . /opt/singbox-panel/
cd /opt/singbox-panel

# 2. 啟動
nohup python3 panel.py > panel.log 2>&1 &
```

## 訪問

```
http://<VPS_IP>:19999/
```

## 文件結構

```
panel.py            — 主程序 (零依賴, http.server)
tproxy-on.sh        — 啟用 TProxy
tproxy-off.sh       — 關閉 TProxy
iptables-clear.sh   — 清除所有 iptables 規則
restart-singbox.sh  — 重啟 sing-box
deploy.sh           — 自動部署腳本
```

## 安全性

- 默認綁定 `0.0.0.0:19999`，可通過 NAT/VPN 訪問
- 無登入機制（設計目標：內網/隧道環境）
- 所有操作通過 shell 腳本執行，需 sudo 密碼

## 技術

- Python 3 標準庫 (`http.server`), 無 Flask/依賴
- 前端：原生 HTML/CSS/JS, 無框架
- 後端：REST API (`POST /api`), JSON 響應

## 注意事項

- TProxy 關閉後流量直連，需確保有基本網絡通路
- 清除 iptables 後所有端口轉發規則會丟失
- 部署到公開網絡前請添加認證層（如 nginx + basic auth）
