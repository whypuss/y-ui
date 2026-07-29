# TUN 功能保留規則（硬約束）

> **來源**：2026-07-29 Maxwell (192.168.31.55) tun 因升級/覆蓋遺失，sudo log 03:46 記錄 `removed 1 tun inbounds`。
> **執行者**：歪貓
> **生效日期**：2026-07-29

## 核心規則
**絕對不得刪除、移除、覆蓋或破壞 tun inbound 功能，無論任何型號的 VPS（實體機、VM、LXC 容器、OpenVZ 均包括）。**
- 即使某型號 VPS（如 LXC）因環境限制（無 /dev/net/tun）暫時無法啟動 TUN，**也不得從 config.json 或代碼中刪除 tun inbound**。
- LXC 上 fallback 用 TProxy，但 tun 配置原封不動保留。

## 根本原因
2026-07-29 Maxwell 的 `/etc/sing-box/config.json` 於 03:46 被手動 `sudo python3` 刪走 tun inbound：
```python
cfg["inbounds"] = [i for i in cfg["inbounds"] if i.get("type") != "tun"]
```
此後 y-ui TunOn() 搜尋 type=="tun" 入面搵唔到，**靜默成功但無效果**。

## 執行約束

### 1. 任何寫 config.json 的函數（SaveInboundPort、UpdateInboundPort、Update*Password、Update*UUID）
- 讀入 config.json 後修改，**寫出前必須確保 inbounds 數量和類型完整**。
- 若原 config 有 `type: "tun"` inbound，寫回時**必須保留**。
- 推薦用 JSON patch（字串置換 `listen_port` / `password`）代替全量 `json.MarshalIndent` 重寫，避免欄位遺失。
- **測試要求**：任何修改 config.json 的函數必須有一個 test case，驗證寫入前後 tun inbound 存在且字段完整。

### 2. y-ui TunOn / TunOff
- **只修改 tun inbound 的 `auto_route` 字段**，不得增刪 tun inbound 本身。
- 如果 config.json 中無 tun inbound，返回明確錯誤 `no tun inbound found`，唔好靜默成功。

### 3. deploy.sh / 升級腳本
- 升級**僅替換 binary**，絕對不得覆蓋 `/etc/sing-box/config.json`。
- 升級完成後必須驗證 config.json 中 tun inbound 仍然存在（`grep '"type": "tun"'`）。

### 4. 部署驗證
- 部署新版本到任何 VPS 後，執行：
  ```bash
  grep -c '"tun"' /etc/sing-box/config.json
  ```
  若返回 0 → 報錯，要求修復。

## TUN 標準配置（備份參考）
Maxwell 原有效配置（已用此配置恢復）：
```json
{
  "type": "tun",
  "tag": "TUN",
  "interface_name": "tun0",
  "address": ["172.19.0.1/30"],
  "stack": "mixed",
  "strict_route": true,
  "auto_route": true
}
```
> 註：sing-box 1.13 上 `address` 字段雖報 legacy warning，但實際可工作（已驗證）。如需新格式，用 `inet4_address: ["172.19.0.1/30"]`（**數組格式，非字串**）。

## 違反本規則即視為開發事故
- 任何導致 tun inbound 遺失的 commit/patch 必須在兩擊規則內停止並報錯。
- 用戶要求：「不管什麼型號的 VPS，都唔好刪 tun 功能。」
