package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"y-ui/internal/exec"
)

// Server web server
type Server struct {
	statusFn func() exec.SystemStatus
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) SetStatus(fn func() exec.SystemStatus) {
	s.statusFn = fn
}

func (s *Server) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api", s.handleAPI)
	mux.HandleFunc("/status", s.handleStatus)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(buildHTML()))
}

type InboundListener = exec.InboundListener
type NodeConfig = exec.NodeConfig

type IptablesSaveReq struct {
	Interface   string `json:"interface"`
	TproxyPort  int    `json:"tproxy_port"`
	RouterIP    string `json:"router_ip"`
	LANSubnet   string `json:"lan_subnet"`
	DNSForward  bool   `json:"dns_forward"`
	Masquerade  bool   `json:"masquerade"`
	Forward     bool   `json:"forward"`
	Tproxy80    bool   `json:"tproxy_80"`
	Tproxy443   bool   `json:"tproxy_443"`
	ExcludeSelf bool   `json:"exclude_self"`
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	var req struct {
		Action    string `json:"action"`
		Host      string `json:"host"`
		Port      int    `json:"port"`
		Method    string `json:"method"`
		Server    string `json:"server"`
		ShortID   string `json:"short_id"`
		Iptables  *IptablesSaveReq `json:"iptables,omitempty"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	var result exec.CommandResult
	switch req.Action {
	case "status":
		s.handleStatus(w, r)
		return
	case "restart-singbox":
		result = exec.RestartSingbox(nil)
	case "tun-on":
		result = exec.TunOn()
	case "tun-off":
		result = exec.TunOff()
	case "tproxy-on":
		result = exec.TproxyOn(nil)
	case "tproxy-off":
		result = exec.TproxyOff(nil)
	case "iptables-save":
		if req.Iptables == nil {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"missing iptables config"}`))
			return
		}
		cfg := exec.IptablesConfig{
			Interface:   req.Iptables.Interface,
			TproxyPort:  req.Iptables.TproxyPort,
			RouterIP:    req.Iptables.RouterIP,
			LANSubnet:   req.Iptables.LANSubnet,
			DNSForward:  req.Iptables.DNSForward,
			Masquerade:  req.Iptables.Masquerade,
			Forward:     req.Iptables.Forward,
			Tproxy80:    req.Iptables.Tproxy80,
			Tproxy443:   req.Iptables.Tproxy443,
			ExcludeSelf: req.Iptables.ExcludeSelf,
		}
		result = exec.WriteIptablesConfig(cfg)
	case "iptables-apply":
		cfg := exec.ReadIptablesConfig()
		result = exec.ApplyIptables(nil, cfg)
	case "iptables-clear":
		result = exec.ClearIptables(nil)
		// 清完之後恢復網關基礎規則（用保存嘅 iptables 配置）
		c := exec.ReadIptablesConfig()
		_ = exec.RestoreGateway(c.Interface, c.LANSubnet)
	case "iptables-rules":
		result = exec.IptablesRules()
	case "iptables-get":
		cfg := exec.ReadIptablesConfig()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(cfg)
		return
	case "fix-dns":
		result = exec.FixSingboxDNS(nil)
	case "nodes-list":
		nodes := exec.GetAllNodes()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(nodes)
		return
	case "nodes-inbound":
		listeners, err := exec.GetInboundListeners()
		resp := struct {
			Listeners []InboundListener `json:"listeners"`
			Error     string           `json:"error,omitempty"`
		}{Listeners: listeners}
		if !err.Ok {
			resp.Error = err.Error
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(resp)
		return
	case "anytls-node":
		node := exec.GetAnyTLSNode()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(node)
	case "nodes-public-ip":
		ip, result := exec.GetPublicIP()
		if result.Ok {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "ip": ip})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": result.Error})
		}
		return
	case "nodes-url":
		urlStr, result := exec.GenAnyTLSURLWithParams(req.Host, req.Port)
		if result.Ok {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "url": urlStr})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": result.Error})
		}
		return
	case "nodes-anyreality-url":
		urlStr, result := exec.GenAnyRealityURLWithParams(req.Host, req.Port, req.Server, req.ShortID)
		if result.Ok {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "url": urlStr})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": result.Error})
		}
		return
	case "nodes-anyreality-config":
		cfg, result := exec.GetAnyRealityConfig()
		if result.Ok {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "config": cfg})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": result.Error})
		}
		return
	case "nodes-anyreality-client-json":
		jsonStr, result := exec.GenAnyRealityClientJSON(req.Host, req.Port, req.Server)
		if result.Ok {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "json": jsonStr})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": result.Error})
		}
		return
	case "nodes-anyreality-compare":
		cfg, cOk := exec.GetAnyRealityConfig()
		if !cOk.Ok {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": cOk.Error})
			return
		}
		inbounds, ibOk := exec.GetSingboxInbounds()
		pwd := ""
		if ibOk.Ok {
			for _, ib := range inbounds {
				tag := ib["tag"].(string)
				if strings.Contains(tag, "REALITY") {
					pwd = exec.ReadSingboxPassword(ib)
					break
				}
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true,
			"system": map[string]interface{}{
				"private_key": cfg.PrivateKey,
				"public_key":  cfg.PublicKey,
				"server_name": cfg.ServerName,
				"short_id":    cfg.ShortID,
				"listen_port": cfg.ListenPort,
				"password":    pwd,
			},
		})
		return
	case "nodes-hy2-url":
		urlStr, result := exec.GenHY2URLWithParams(req.Host, req.Port)
		if result.Ok {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "url": urlStr})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": result.Error})
		}
		return
	case "nodes-ss-url":
		method := strings.ToLower(req.Method)
		if method == "" {
			method = "aes-256-gcm"
		}
		urlStr, result := exec.GenSSURLWithParams(req.Host, req.Port, method)
		if result.Ok {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "url": urlStr, "method": method})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": result.Error})
		}
		return
	case "nodes-current-password":
		inbounds, cr := exec.GetSingboxInbounds()
		if !cr.Ok {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": cr.Error})
			return
		}
		pwMap := map[string]string{}
		for _, ib := range inbounds {
			if t, ok := ib["type"]; ok {
				tstr := t.(string)
				pw := exec.ReadSingboxPassword(ib)
				// 映射為前端友好的鍵名
				switch tstr {
				case "hysteria2":
					pwMap["hy2"] = pw
				case "shadowsocks":
					pwMap["ss"] = pw
				case "anytls":
					// 檢查是否係 AnyTLS-Reality（有 reality）
					if tls, ok := ib["tls"].(map[string]interface{}); ok {
						if reality, ok := tls["reality"].(map[string]interface{}); ok {
							if enabled, ok := reality["enabled"].(bool); ok && enabled {
								pwMap["anyreality"] = pw
								// AnyTLS 普通也同時返回
								if _, exists := pwMap["anytls"]; !exists {
									pwMap["anytls"] = pw
								}
								break
							}
						}
					}
					pwMap["anytls"] = pw
				default:
					pwMap[tstr] = pw
				}
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "passwords": pwMap})
		return
	case "nodes-update-uuid":
		newUUID, result := exec.UpdateAnyTLSUUID()
		if result.Ok {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "uuid": newUUID, "stdout": result.Stdout})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": result.Error})
		}
		return
	case "nodes-update-hy2":
		newPw, result := exec.UpdateHY2Password()
		if result.Ok {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "password": newPw, "stdout": result.Stdout})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": result.Error})
		}
		return
	case "nodes-update-anyreality":
		newPw, result := exec.UpdateAnyRealityPassword()
		if result.Ok {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "password": newPw, "stdout": result.Stdout})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": result.Error})
		}
		return
	case "nodes-update-ss":
		newPw, result := exec.UpdateSSPassword()
		if result.Ok {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "password": newPw, "stdout": result.Stdout})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": result.Error})
		}
		return
	default:
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"unknown action"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(s.statusFn())
}

func buildHTML() string {
	return `<!DOCTYPE html>
<html lang="zh-Hant">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>y-ui - Sing-Box Control Panel</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#0f1117;color:#e1e4e8;margin:0;display:flex;min-height:100vh}
.sidebar{width:130px;background:#161b22;border-right:1px solid #30363d;display:flex;flex-direction:column;flex-shrink:0}
.sidebar-h{padding:14px 12px 10px;font-size:1.1em;color:#58a6ff;font-weight:600;text-align:center;border-bottom:1px solid #30363d}
.sidebar-nav{flex:1;padding:8px 0}
.nav-item{display:flex;align-items:center;gap:8px;padding:11px 14px;color:#8b949e;cursor:pointer;font-size:0.92em;transition:all .15s;border-left:3px solid transparent}
.nav-item:hover{background:#0d1117;color:#e1e4e8}
.nav-item.active{background:#0d1117;color:#58a6ff;border-left-color:#58a6ff}
.nav-icon{font-size:1.05em;width:20px;text-align:center}
.main{flex:1;padding:18px;max-width:720px;margin:0 auto;width:100%}
.page{display:none} .page.active{display:block}
h1{text-align:center;margin-bottom:4px;color:#58a6ff;font-size:1.5em}
.subtitle{text-align:center;color:#8b949e;font-size:0.78em;margin-bottom:14px}
.status-bar{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:10px 12px;margin-bottom:8px;display:flex;justify-content:space-between;align-items:center}
.status-label{font-size:0.82em;color:#8b949e} .status-value{font-size:0.92em;font-weight:600}
.on{color:#3fb950} .off{color:#f85149} .checking{color:#58a6ff}
.card{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:13px;margin-bottom:10px}
.card-title{font-size:0.92em;color:#58a6ff;margin-bottom:2px} .card-desc{font-size:0.76em;color:#8b949e;margin-bottom:8px}
.btn{background:#21262d;border:1px solid #30363d;color:#e1e4e8;padding:8px 10px;border-radius:6px;cursor:pointer;font-size:0.85em;width:100%;transition:all .15s}
.btn:hover{background:#30363d;border-color:#58a6ff} .btn.danger{border-color:#f85149;color:#f85149} .btn.danger:hover{background:rgba(248,81,73,.1)}
.btn.success{border-color:#3fb950;color:#3fb950} .btn.success:hover{background:rgba(63,185,80,.1)} .btn:disabled{opacity:.4;cursor:not-allowed}
.btn-group{display:flex;gap:6px} .btn-group .btn{flex:1}
.result{margin-top:6px;padding:7px;background:#0d1117;border-radius:6px;font-size:0.76em;color:#8b949e;white-space:pre-wrap;max-height:140px;overflow-y:auto;display:none}
.result.ok{color:#3fb950;display:block} .result.err{color:#f85149;display:block}
.updating{color:#58a6ff;font-size:0.7em;text-align:center;margin-top:4px}
.config-table{width:100%;margin-bottom:6px;font-size:0.8em} .config-table td{padding:3px 2px;color:#8b949e} .config-table td:first-child{color:#58a6ff;white-space:nowrap}
.input{background:#0d1117;border:1px solid #30363d;color:#e1e4e8;padding:5px 7px;border-radius:4px;width:100%;font-size:0.92em} .input:focus{outline:none;border-color:#58a6ff}
.checkbox-group{display:flex;flex-wrap:wrap;gap:5px;margin:7px 0}
.checkbox{display:flex;align-items:center;gap:4px;background:#0d1117;border:1px solid #30363d;padding:5px 7px;border-radius:4px;font-size:0.8em;color:#e1e4e8;cursor:pointer}
.checkbox input{margin:0}
.proto{display:inline-block;padding:2px 6px;border-radius:4px;font-size:0.72em;font-weight:600}
.proto-anytls{background:#1f3a5f;color:#58a6ff} .proto-vless{background:#2d3a1f;color:#a5d6a7}
.proto-socks{background:#3a2d1f;color:#d4a574} .proto-mixed{background:#3a1f3a;color:#ce93d8} .proto-other{background:#2d2d3a;color:#aaa}
.copy-row{display:flex;gap:4px;margin-bottom:8px}
.copy-row .copy-box{flex:1;background:#0d1117;border:1px solid #30363d;border-radius:6px;padding:7px 9px;font-family:monospace;font-size:0.75em;color:#e1e4e8;word-break:break-all;margin:0}
.btn-copy{padding:7px 9px;font-size:0.8em;white-space:nowrap;background:#21262d;border:1px solid #30363d;color:#58a6ff;border-radius:6px;cursor:pointer;transition:all .15s}
.btn-copy:hover{background:#30363d;border-color:#58a6ff}
.node-table{width:100%;font-size:0.78em;margin-bottom:4px}
.node-table th,.node-table td{padding:5px 3px;text-align:left}
.node-table th{color:#58a6ff;font-weight:500} .node-table td{color:#e1e4e8;word-break:break-all}
.node-table .port{color:#3fb950;font-family:monospace} .node-table .tag{color:#f0e68c;font-family:monospace}
@media(max-width:680px){body{flex-direction:column} .sidebar{width:100%;flex-direction:row;border-right:none;border-bottom:1px solid #30363d} .sidebar-h{padding:8px;font-size:1em;border-bottom:none;border-right:1px solid #30363d} .sidebar-nav{display:flex;flex-direction:row;justify-content:center;padding:3px} .nav-item{padding:7px 10px;font-size:0.82em} .main{padding:10px}}
</style>
</head>
<body>
<div class="sidebar">
  <div class="sidebar-h">y-ui</div>
  <div class="sidebar-nav">
    <div class="nav-item active" onclick="showPage('home',this)"><span class="nav-icon">🏠</span><span>主頁</span></div>
    <div class="nav-item" onclick="showPage('nodes',this)"><span class="nav-icon">🌐</span><span>節點</span></div>
  </div>
</div>
<div class="main">
<div id="page-home" class="page active">
<h1>y-ui</h1><p class="subtitle">Sing-Box Control Panel</p>
<div class="status-bar"><span class="status-label">sing-box</span><span id="sb" class="status-value checking">checking...</span></div>
<div class="status-bar"><span class="status-label">TUN</span><span id="tun" class="status-value checking">checking...</span></div>
<div class="status-bar"><span class="status-label">TProxy</span><span id="tp" class="status-value checking">checking...</span></div>
<div class="status-bar"><span class="status-label">外網</span><span id="net" class="status-value checking">checking...</span></div>
<p class="updating" id="ts">refreshing...</p>
<div class="card"><div class="card-title">sing-box 重啟</div><div class="card-desc">重啟 sing-box（auto_route=false）</div><button class="btn" onclick="execAction('restart-singbox','r-sb')">Restart sing-box</button><div id="r-sb" class="result"></div></div>
<div class="card"><div class="card-title">TUN</div><div class="card-desc">開/關 sing-box TUN 主進程</div><div class="btn-group"><button class="btn success" onclick="execAction('tun-on','r-tun')">開啟</button><button class="btn danger" onclick="execAction('tun-off','r-tun')">關閉</button></div><div id="r-tun" class="result"></div></div>
<div class="card"><div class="card-title">TProxy</div><div class="card-desc">開啟/關閉 TProxy（需 TUN 先開）</div><div class="btn-group"><button class="btn success" onclick="execAction('tproxy-on','r-tp')">開啟</button><button class="btn danger" onclick="execAction('tproxy-off','r-tp')">關閉</button></div><div id="r-tp" class="result"></div></div>
<div class="card">
<div class="card-title">iptables 配置</div><div class="card-desc">填寫參數，勾選規則，按 Apply 生效</div>
<table class="config-table">
<tr><td>網卡 (IFACE)</td><td><input type="text" id="i-iface" class="input" value="eth0"></td></tr>
<tr><td>TProxy 端口</td><td><input type="number" id="i-port" class="input" value="10808"></td></tr>
<tr><td>路由器 IP (DNS)</td><td><input type="text" id="i-router" class="input" value="192.168.1.1"></td></tr>
<tr><td>LAN 網段</td><td><input type="text" id="i-lan" class="input" value="192.168.1.0/24"></td></tr>
</table>
<div class="checkbox-group">
<label class="checkbox"><input type="checkbox" id="i-tproxy80" checked> TPROXY :80</label>
<label class="checkbox"><input type="checkbox" id="i-tproxy443" checked> TPROXY :443</label>
<label class="checkbox"><input type="checkbox" id="i-dns" checked> DNS DNAT 53</label>
<label class="checkbox"><input type="checkbox" id="i-masq" checked> MASQUERADE</label>
<label class="checkbox"><input type="checkbox" id="i-fwd" checked> FORWARD ACCEPT</label>
<label class="checkbox"><input type="checkbox" id="i-excl" checked> 排除本機流量（防循環）</label>
</div>
<div class="btn-group"><button class="btn" onclick="iptablesSave()">保存配置</button><button class="btn success" onclick="iptablesApply()">Apply</button><button class="btn" onclick="iptablesRules()">查看規則</button><button class="btn danger" onclick="iptablesClear()">Clear</button></div>
<div id="r-iptables" class="result"></div>
</div>
</div>
<div id="page-nodes" class="page">
<h1>節點</h1><p class="subtitle">一鍵獲取標準分享連結</p>
<div class="status-bar"><span class="status-label">公網 IP</span><span id="pubip" class="status-value">載入中...</span></div>

<div class="card">
<div class="card-title">🌐 AnyTLS</div>
<div class="card-desc">anytls://UUID@IP:Port/?insecure=1</div>
<table class="config-table">
<tr><td>內部端口</td><td><input type="number" id="anytls-port" class="input" placeholder="17777"></td></tr>
<tr><td>映射端口</td><td><input type="number" id="anytls-mapped" class="input" placeholder="留空=內部端口"></td></tr>
<tr><td>系統密碼</td><td><span id="anytls-pwd" class="status-value">載入中...</span></td></tr>
</table>
<div class="btn-group"><button class="btn success" onclick="getAnyTLSNode()">獲取節點</button><button class="btn" onclick="updateAnyTLSUUID()">更新 UUID</button><button class="btn" onclick="viewAnyTLSPassword()">查看系統密碼</button></div>
<div id="r-anytls" class="result"></div>
<div id="c-anytls" class="copy-row" style="display:none"><div class="copy-box" id="anytls-box"></div><button class="btn-copy" onclick="copyText(document.getElementById('anytls-box').textContent)">📋 複製</button></div>
</div>

<div class="card">
<div class="card-title">🔴 AnyTLS+Reality</div>
<div class="card-desc">anytls://password@IP:Port/?server=SN&shortId=ID&insecure=1</div>
<table class="config-table">
<tr><td>內部端口</td><td><input type="number" id="anyreality-port" class="input" placeholder="444"></td></tr>
<tr><td>映射端口</td><td><input type="number" id="anyreality-mapped" class="input" placeholder="留空=內部端口"></td></tr>
<tr><td>server_name</td><td><input type="text" id="anyreality-server" class="input" placeholder="www.epicgames.com"></td></tr>
<tr><td>short_id</td><td><input type="text" id="anyreality-shortid" class="input" placeholder="留空=無"></td></tr>
<tr><td>private_key</td><td><input type="text" id="anyreality-privatekey" class="input" placeholder="服務器端私鑰" readonly></td></tr>
<tr><td>public_key</td><td><input type="text" id="anyreality-publickey" class="input" placeholder="客戶端用（留空=自動）" readonly></td></tr>
<tr><td>系統密碼</td><td><span id="anyreality-pwd" class="status-value">載入中...</span></td></tr>
</table>
<div id="anyreality-compare" class="compare-table"></div>
<div class="btn-group"><button class="btn success" onclick="getAnyRealityNode()">獲取節點</button><button class="btn" onclick="updateAnyRealityPassword()">更新密碼</button><button class="btn" onclick="viewPassword('anyreality')">查看系統密碼</button></div>
<div id="r-anyreality" class="result"></div>
<div id="c-anyreality" class="copy-row" style="display:none"><div class="copy-box" id="anyreality-box"></div><button class="btn-copy" onclick="copyText(document.getElementById('anyreality-box').textContent)">📋 複製</button></div>
<div id="c-anyreality-json" class="copy-row" style="display:none"><div class="config-table" style="font-family:monospace;font-size:12px;word-break:break-all"><div class="status-value" id="anyreality-json"></div></div><button class="btn-copy" onclick="copyText(document.getElementById('anyreality-json').textContent)">📋 複製 JSON</button></div>
</div>

<div class="card">
<div class="card-title">🚀 HY2 (Hysteria 2)</div>
<div class="card-desc">hysteria2://password@IP:Port/?insecure=1</div>
<table class="config-table">
<tr><td>內部端口</td><td><input type="number" id="hy2-port" class="input" placeholder="20000"></td></tr>
<tr><td>映射端口</td><td><input type="number" id="hy2-mapped" class="input" placeholder="留空=內部端口"></td></tr>
<tr><td>系統密碼</td><td><span id="hy2-pwd" class="status-value">載入中...</span></td></tr>
</table>
<div class="btn-group"><button class="btn success" onclick="getHY2Node()">獲取節點</button><button class="btn" onclick="updateHY2Password()">更新密碼</button><button class="btn" onclick="viewPassword('hy2')">查看系統密碼</button></div>
<div id="r-hy2" class="result"></div>
<div id="c-hy2" class="copy-row" style="display:none"><div class="copy-box" id="hy2-box"></div><button class="btn-copy" onclick="copyText(document.getElementById('hy2-box').textContent)">📋 複製</button></div>
</div>

<div class="card">
<div class="card-title">🔗 SS (Shadowsocks)</div>
<div class="card-desc">ss://base64(method:password@IP:Port)</div>
<table class="config-table">
<tr><td>內部端口</td><td><input type="number" id="ss-port" class="input" placeholder="20001"></td></tr>
<tr><td>映射端口</td><td><input type="number" id="ss-mapped" class="input" placeholder="留空=內部端口"></td></tr>
<tr><td>加密</td><td><select id="ss-method" class="input"><option>aes-256-gcm</option><option>aes-128-gcm</option><option>chacha20-poly1305</option><option>chacha20-ietf-poly1305</option></select></td></tr>
<tr><td>系統密碼</td><td><span id="ss-pwd" class="status-value">載入中...</span></td></tr>
</table>
<div class="btn-group"><button class="btn success" onclick="getSSNode()">獲取節點</button><button class="btn" onclick="updateSSPassword()">更新密碼</button><button class="btn" onclick="viewPassword('ss')">查看系統密碼</button></div>
<div id="r-ss" class="result"></div>
<div id="c-ss" class="copy-row" style="display:none"><div class="copy-box" id="ss-box"></div><button class="btn-copy" onclick="copyText(document.getElementById('ss-box').textContent)">📋 複製</button></div>
</div>

</div>
<script>
function showPage(name,el){document.querySelectorAll('.page').forEach(p=>p.classList.remove('active'));document.getElementById('page-'+name).classList.add('active');document.querySelectorAll('.nav-item').forEach(n=>n.classList.remove('active'));if(el)el.classList.add('active')}
async function api(action,body){const p={action};if(body!==undefined)Object.assign(p,body);const r=await fetch('/api',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(p)});return r.json()}
function show(id,text,ok){const e=document.getElementById(id);e.textContent=text;e.className='result '+(ok?'ok':'err')}
function copyText(t){try{var ta=document.createElement('textarea');ta.value=t;ta.style.cssText='position:fixed;left:-9999px;top:-9999px;width:1px;height:1px;opacity:0';document.body.appendChild(ta);ta.focus();ta.select();ta.setSelectionRange(0,t.length);document.execCommand('copy');document.body.removeChild(ta);alert('已複製到剪貼簿')}catch(e){alert('複製失敗')}}
async function execAction(action,rid){const b=event.target;b.disabled=true;try{const d=await api(action);if(d.ok)show(rid,'OK\n'+(d.stdout||'')+(d.stderr?'\nstderr: '+d.stderr:''),true);else show(rid,'FAIL\n'+(d.error||'')+(d.stdout?'\n'+d.stdout:'')+(d.stderr?'\nstderr: '+d.stderr:''),false)}catch(e){show(rid,'ERROR: '+e.message,false)}b.disabled=false}
async function refreshStatus(){try{const d=await api('status');const set=(id,k,f)=>{const e=document.getElementById(id);const v=d[k];e.textContent=f?f(v):(v?'active':'disabled');e.className='status-value '+(v?'on':'off')};set('sb','singbox_running',v=>v?'running '+d.singbox_info:'stopped');set('tun','tun_active');set('tp','tproxy_active');set('net','direct_net');document.getElementById('ts').textContent='updated '+new Date().toLocaleTimeString()}catch(e){document.getElementById('ts').textContent='error: '+e.message}}
setInterval(refreshStatus,5000);refreshStatus();loadIptablesConfig();loadPublicIP();loadInboundPorts()
function iptablesForm(){return{interface:document.getElementById('i-iface').value,tproxy_port:parseInt(document.getElementById('i-port').value)||10808,router_ip:document.getElementById('i-router').value,lan_subnet:document.getElementById('i-lan').value,tproxy_80:document.getElementById('i-tproxy80').checked,tproxy_443:document.getElementById('i-tproxy443').checked,dns_forward:document.getElementById('i-dns').checked,masquerade:document.getElementById('i-masq').checked,forward:document.getElementById('i-fwd').checked,exclude_self:document.getElementById('i-excl').checked}}
async function iptablesSave(){const c=iptablesForm();const r=await api('iptables-save',{iptables:c});show('r-iptables',(r.ok?'SAVED\n':'FAIL\n')+(r.stdout||r.error||''),r.ok)}
async function iptablesApply(){const r=await api('iptables-apply');show('r-iptables',(r.ok?'APPLIED\n':'FAIL\n')+(r.stdout||'')+(r.stderr?'stderr: '+r.stderr:''),r.ok);refreshStatus()}
async function iptablesClear(){const r=await api('iptables-clear');show('r-iptables',(r.ok?'CLEARED\n':'FAIL\n')+(r.stdout||''),r.ok);refreshStatus()}
async function iptablesRules(){const r=await api('iptables-rules');show('r-iptables','RULES:\n'+(r.stdout||'')+(r.stderr?'stderr: '+r.stderr:''),r.ok)}
async function loadIptablesConfig(){try{const r=await api('iptables-get');document.getElementById('i-iface').value=r.interface||'';document.getElementById('i-port').value=r.tproxy_port||'';document.getElementById('i-router').value=r.router_ip||'';document.getElementById('i-lan').value=r.lan_subnet||'';document.getElementById('i-tproxy80').checked=r.tproxy_80||false;document.getElementById('i-tproxy443').checked=r.tproxy_443||false;document.getElementById('i-dns').checked=r.dns_forward||false;document.getElementById('i-masq').checked=r.masquerade||false;document.getElementById('i-fwd').checked=r.forward||false;document.getElementById('i-excl').checked=r.exclude_self||false}catch(e){}}
async function loadPublicIP(){api('nodes-public-ip').then(function(d){if(d.ok)document.getElementById('pubip').textContent=d.ip;else document.getElementById('pubip').textContent='載入失敗'}).catch(function(){document.getElementById('pubip').textContent='載入失敗'})}
// 本地緩存：輸入框端口不丟失
function lcGet(k){try{return localStorage.getItem(k)}catch(e){return null}}
function lcSet(k,v){try{localStorage.setItem(k,v)}catch(e){}}
async function loadInboundPorts(){try{const r=await api('nodes-inbound');var fromAPI={};if(r.listeners&&r.listeners.length>0){r.listeners.forEach(function(x){var t=(x.tag||'').toLowerCase();if(t.indexOf('anytls')>=0)fromAPI.anytls=x.port;else if(t.indexOf('hysteria2')>=0||t.indexOf('hy2')>=0)fromAPI.hy2=x.port;else if(t.indexOf('shadowsocks')>=0||t.indexOf('ss-')>=0)fromAPI.ss=x.port})}}catch(e){}
['anytls','hy2','ss'].forEach(function(p){
  var lp=lcGet(p+'-port');var mp=lcGet(p+'-mapped');
  if(lp)document.getElementById(p+'-port').value=lp;
  else if(fromAPI[p])document.getElementById(p+'-port').value=fromAPI[p];
  if(mp)document.getElementById(p+'-mapped').value=mp;
});
// 緩存 ss-method
var sm=lcGet('ss-method');if(sm)document.getElementById('ss-method').value=sm;
// 監聽所有輸入框變化即時寫回 localStorage
['anytls-port','anytls-mapped','hy2-port','hy2-mapped','ss-port','ss-mapped','ss-method'].forEach(function(id){var e=document.getElementById(id);if(e)e.addEventListener('input',function(){lcSet(id,e.value)})})}
function showNode(boxId,copyId,resultId,url){document.getElementById(boxId).textContent=url;document.getElementById(copyId).style.display='flex';show(resultId,'✅ 已生成',true)}
function getPort(insideId,mappedId){var m=document.getElementById(mappedId).value;return parseInt(m)||parseInt(document.getElementById(insideId).value)||0}
async function getAnyTLSNode(){var port=getPort('anytls-port','anytls-mapped');const u=await api('nodes-url',{port:port});if(u.ok&&u.url){showNode('anytls-box','c-anytls','r-anytls',u.url)}else{show('r-anytls','FAIL: '+(u.error||''),false)}}
async function getHY2Node(){var port=getPort('hy2-port','hy2-mapped');const u=await api('nodes-hy2-url',{port:port});if(u.ok&&u.url){showNode('hy2-box','c-hy2','r-hy2',u.url)}else{show('r-hy2','FAIL: '+(u.error||''),false)}}
async function getSSNode(){var port=getPort('ss-port','ss-mapped');var method=document.getElementById('ss-method').value;const u=await api('nodes-ss-url',{port:port,method:method});if(u.ok&&u.url){showNode('ss-box','c-ss','r-ss',u.url)}else{show('r-ss','FAIL: '+(u.error||''),false)}}
async function updateAnyTLSUUID(){const b=event.target;b.disabled=true;try{const r=await api('nodes-update-uuid');if(r.ok){show('r-anytls','✅ UUID 已更新\n'+r.stdout,true);b.disabled=false}else{show('r-anytls','FAIL: '+(r.error||''),false);b.disabled=false}}catch(e){show('r-anytls','ERROR: '+e.message,false);b.disabled=false}}
async function updateHY2Password(){const b=event.target;b.disabled=true;try{const r=await api('nodes-update-hy2');if(r.ok){show('r-hy2','✅ 密碼已更新\n'+r.stdout,true);b.disabled=false}else{show('r-hy2','FAIL: '+(r.error||''),false);b.disabled=false}}catch(e){show('r-hy2','ERROR: '+e.message,false);b.disabled=false}}
async function updateSSPassword(){const b=event.target;b.disabled=true;try{const r=await api('nodes-update-ss');if(r.ok){show('r-ss','✅ 密碼已更新\\n'+r.stdout,true);b.disabled=false}else{show('r-ss','FAIL: '+(r.error||''),false);b.disabled=false}}catch(e){show('r-ss','ERROR: '+e.message,false);b.disabled=false}}
async function viewAnyTLSPassword(){const d=await viewPassword('anytls')}
async function viewPassword(type){const t={anytls:'anytls-pwd',hy2:'hy2-pwd',ss:'ss-pwd',anyreality:'anyreality-pwd'};const rid={anytls:'r-anytls',hy2:'r-hy2',ss:'r-ss',anyreality:'r-anyreality'};const label={anytls:'AnyTLS',hy2:'HY2',ss:'SS',anyreality:'AnyTLS+Reality'};try{const d=await api('nodes-current-password');var pw=d.passwords&&d.passwords[type];if(pw){document.getElementById(t[type]).textContent=pw;show(rid[type],label[type]+' 系統密碼: '+pw,true)}else{show(rid[type],label[type]+' 未找到密碼',false)}}catch(e){show(rid[type],'ERROR: '+e.message,false)}}
(function(){api('nodes-current-password').then(function(d){var pw=d.passwords;if(pw&&pw.anytls)document.getElementById('anytls-pwd').textContent=pw.anytls;if(pw&&pw.anyreality)document.getElementById('anyreality-pwd').textContent=pw.anyreality;if(pw&&pw.hy2)document.getElementById('hy2-pwd').textContent=pw.hy2;if(pw&&pw.ss)document.getElementById('ss-pwd').textContent=pw.ss}).catch(function(){})})();(function(){api('nodes-anyreality-config').then(function(d){if(d.ok&&d.config){if(d.config.private_key)document.getElementById('anyreality-privatekey').value=d.config.private_key;if(d.config.public_key)document.getElementById('anyreality-publickey').value=d.config.public_key;else document.getElementById('anyreality-publickey').placeholder='客戶端自動（無需手動填寫）';if(d.config.server_name)document.getElementById('anyreality-server').value=d.config.server_name;if(d.config.short_id)document.getElementById('anyreality-shortid').value=d.config.short_id;if(d.config.listen_port)document.getElementById('anyreality-port').value=d.config.listen_port}}).catch(function(){})})();(function(){api('nodes-anyreality-compare').then(function(d){if(d.ok&&d.system){var c=document.getElementById('anyreality-compare');c.innerHTML='<table class="config-table"><tr><th colspan="3">🔴 節點 vs 系統 對比</th></tr><tr><td>password</td><td class="on">'+d.system.password+'</td><td class="on">'+d.system.password+'</td></tr><tr><td>server_name</td><td class="on">'+(d.system.server_name||'無')+'</td><td class="on">'+(d.system.server_name||'無')+'</td></tr><tr><td>short_id</td><td class="on">'+(d.system.short_id||'空')+'</td><td class="on">'+(d.system.short_id||'空')+'</td></tr><tr><td>public_key</td><td class="on">'+(d.system.public_key||'計算中').substring(0,12)+'...</td><td class="on">'+(d.system.public_key||'計算中').substring(0,12)+'...</td></tr><tr><td>private_key</td><td class="on">'+d.system.private_key.substring(0,12)+'...</td><td class="on">'+d.system.private_key.substring(0,12)+'...</td></tr></table>'}}).catch(function(){})})()
async function getAnyRealityNode(){var port=getPort('anyreality-port','anyreality-mapped');var sv=document.getElementById('anyreality-server').value;var sid=document.getElementById('anyreality-shortid').value;const u=await api('nodes-anyreality-url',{port:port,server:sv,short_id:sid});if(u.ok&&u.url){showNode('anyreality-box','c-anyreality','r-anyreality',u.url);showCompare(u.url)}else{show('r-anyreality','FAIL: '+(u.error||''),false)};(function(){api('nodes-anyreality-client-json',{port:port,server:sv}).then(function(j){if(j.ok&&j.json){document.getElementById('anyreality-json').textContent=j.json;document.getElementById('c-anyreality-json').style.display='flex'}}).catch(function(){})})();const j=await api('nodes-anyreality-client-json',{port:port,server:sv});if(j.ok&&j.json){document.getElementById('anyreality-json').textContent=j.json}}
function showCompare(url){var box=document.getElementById('anyreality-compare');var pwd=document.getElementById('anyreality-pwd').textContent||'';var sni=document.getElementById('anyreality-server').value||'';var sid=document.getElementById('anyreality-shortid').value||'';var priv=document.getElementById('anyreality-privatekey').value||'';var mapped=document.getElementById('anyreality-mapped').value||'';var inside=document.getElementById('anyreality-port').value||'';var usedPort=mapped||inside;
box.innerHTML='<table class="config-table"><tr><th colspan="3">節點 vs 系統 對比</th></tr><tr><td>映射端口</td><td><span class="status-value">'+(usedPort||'-')+'</span></td><td><span class="status-value">'+inside+'</span></td></tr><tr><td>password</td><td><span class="status-value">'+pwd+'</span></td><td><span class="status-value">'+pwd+'</span></td></tr><tr><td>server_name (SNI)</td><td><span class="status-value">'+(sni||'未填')+'</span></td><td><span class="status-value">'+(sni||'www.epicgames.com')+'</span></td></tr><tr><td>short_id</td><td><span class="status-value">'+(sid||'空')+'</span></td><td><span class="status-value">'+(sid||'空')+'</span></td></tr><tr><td>private_key</td><td><span class="status-value">'+priv.substring(0,16)+'...</span></td><td><span class="status-value">'+priv.substring(0,16)+'...</span></td></tr></table>'}
async function updateAnyRealityPassword(){const b=event.target;b.disabled=true;try{const r=await api('nodes-update-anyreality');if(r.ok){show('r-anyreality','✅ 密碼已更新\n'+r.stdout,true);b.disabled=false}else{show('r-anyreality','FAIL: '+(r.error||''),false);b.disabled=false}}catch(e){show('r-anyreality','ERROR: '+e.message,false);b.disabled=false}}
</script>
</body>
</html>`
}


