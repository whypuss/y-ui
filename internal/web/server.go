package web

import (
	"encoding/json"
	"net/http"
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
		Action    string         `json:"action"`
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
		// 清完之後恢復網關基礎規則，確保 LAN 設備仍可直連上網
		_ = exec.RestoreGateway("enp4s0f0", "192.168.31.0/24")
	case "iptables-rules":
		result = exec.IptablesRules()
	case "iptables-get":
		cfg := exec.ReadIptablesConfig()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(cfg)
		return
	case "fix-dns":
		result = exec.FixSingboxDNS(nil)
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
* { margin: 0; padding: 0; box-sizing: border-box; }
body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: #0f1117; color: #e1e4e8;
    padding: 20px; max-width: 640px; margin: 0 auto;
}
h1 { text-align: center; margin-bottom: 4px; color: #58a6ff; font-size: 1.6em; }
.subtitle { text-align: center; color: #8b949e; font-size: 0.8em; margin-bottom: 18px; }
.status-bar {
    background: #161b22; border: 1px solid #30363d; border-radius: 8px;
    padding: 12px 14px; margin-bottom: 10px;
    display: flex; justify-content: space-between; align-items: center;
}
.status-label { font-size: 0.85em; color: #8b949e; }
.status-value { font-size: 0.95em; font-weight: 600; }
.on { color: #3fb950; }
.off { color: #f85149; }
.checking { color: #58a6ff; }
.card {
    background: #161b22; border: 1px solid #30363d;
    border-radius: 8px; padding: 14px; margin-bottom: 10px;
}
.card-title { font-size: 0.95em; color: #58a6ff; margin-bottom: 2px; }
.card-desc { font-size: 0.78em; color: #8b949e; margin-bottom: 10px; }
.btn {
    background: #21262d; border: 1px solid #30363d; color: #e1e4e8;
    padding: 9px 12px; border-radius: 6px; cursor: pointer;
    font-size: 0.88em; width: 100%; transition: all 0.15s;
}
.btn:hover { background: #30363d; border-color: #58a6ff; }
.btn.danger { border-color: #f85149; color: #f85149; }
.btn.danger:hover { background: rgba(248,81,73,0.1); }
.btn.success { border-color: #3fb950; color: #3fb950; }
.btn.success:hover { background: rgba(63,185,80,0.1); }
.btn:disabled { opacity: 0.4; cursor: not-allowed; }
.btn-group { display: flex; gap: 6px; }
.btn-group .btn { flex: 1; }
.result {
    margin-top: 8px; padding: 8px;
    background: #0d1117; border-radius: 6px;
    font-size: 0.78em; color: #8b949e;
    white-space: pre-wrap; max-height: 150px; overflow-y: auto;
    display: none;
}
.result.ok { color: #3fb950; display: block; }
.result.err { color: #f85149; display: block; }
.updating { color: #58a6ff; font-size: 0.72em; text-align: center; margin-top: 4px; }
.config-table { width: 100%; margin-bottom: 8px; font-size: 0.82em; }
.config-table td { padding: 4px 2px; color: #8b949e; }
.config-table td:first-child { color: #58a6ff; white-space: nowrap; }
.input { background: #0d1117; border: 1px solid #30363d; color: #e1e4e8; padding: 5px 8px; border-radius: 4px; width: 100%; font-size: 0.95em; }
.input:focus { outline: none; border-color: #58a6ff; }
.checkbox-group { display: flex; flex-wrap: wrap; gap: 6px; margin: 8px 0; }
.checkbox { display: flex; align-items: center; gap: 4px; background: #0d1117; border: 1px solid #30363d; padding: 5px 8px; border-radius: 4px; font-size: 0.82em; color: #e1e4e8; cursor: pointer; }
.checkbox input { margin: 0; }
</style>
</head>
<body>
<h1>y-ui</h1>
<p class="subtitle">Sing-Box Control Panel</p>
<div class="status-bar"><span class="status-label">sing-box</span><span id="sb" class="status-value checking">checking...</span></div>
<div class="status-bar"><span class="status-label">TUN</span><span id="tun" class="status-value checking">checking...</span></div>
<div class="status-bar"><span class="status-label">TProxy</span><span id="tp" class="status-value checking">checking...</span></div>
<div class="status-bar"><span class="status-label">外網</span><span id="net" class="status-value checking">checking...</span></div>
<div class="status-bar"><span class="status-label">容器→Mac Ollama</span><span id="ol" class="status-value checking">checking...</span></div>
<p class="updating" id="ts">refreshing...</p>
<div class="card">
    <div class="card-title">sing-box 重啟</div>
    <div class="card-desc">重啟 sing-box（自動修復 DNS 配置）</div>
    <button class="btn" onclick="execAction('restart-singbox','r-sb')">Restart sing-box</button>
    <div id="r-sb" class="result"></div>
</div>
<div class="card">
    <div class="card-title">TUN</div>
    <div class="card-desc">開/關 sing-box TUN 主進程（保留 SOCKS 代理）</div>
    <div class="btn-group">
        <button class="btn success" onclick="execAction('tun-on','r-tun')">開啟</button>
        <button class="btn danger" onclick="execAction('tun-off','r-tun')">關閉</button>
    </div>
    <div id="r-tun" class="result"></div>
</div>
<div class="card">
    <div class="card-title">TProxy</div>
    <div class="card-desc">開啟/關閉 TProxy</div>
    <div class="btn-group">
        <button class="btn success" onclick="execAction('tproxy-on','r-tp')">開啟</button>
        <button class="btn danger" onclick="execAction('tproxy-off','r-tp')">關閉</button>
    </div>
    <div id="r-tp" class="result"></div>
</div>
<div class="card">
    <div class="card-title">iptables 配置</div>
    <div class="card-desc">填寫參數，勾選規則，按 Apply 生效</div>
    <table class="config-table">
        <tr><td>網卡 (IFACE)</td><td><input type="text" id="i-iface" class="input" value="enp4s0f0"></td></tr>
        <tr><td>TProxy 端口</td><td><input type="number" id="i-port" class="input" value="10808"></td></tr>
        <tr><td>路由器 IP (DNS)</td><td><input type="text" id="i-router" class="input" value="192.168.31.1"></td></tr>
        <tr><td>LAN 網段</td><td><input type="text" id="i-lan" class="input" value="192.168.31.0/24"></td></tr>
    </table>
    <div class="checkbox-group">
        <label class="checkbox"><input type="checkbox" id="i-tproxy80" checked> TPROXY :80</label>
        <label class="checkbox"><input type="checkbox" id="i-tproxy443" checked> TPROXY :443</label>
        <label class="checkbox"><input type="checkbox" id="i-dns" checked> DNS DNAT 53</label>
        <label class="checkbox"><input type="checkbox" id="i-masq" checked> MASQUERADE</label>
        <label class="checkbox"><input type="checkbox" id="i-fwd" checked> FORWARD ACCEPT</label>
        <label class="checkbox"><input type="checkbox" id="i-excl" checked> 排除本機流量（防循環）</label>
    </div>
    <div class="btn-group">
        <button class="btn" onclick="iptablesSave()">保存配置</button>
        <button class="btn success" onclick="iptablesApply()">Apply</button>
        <button class="btn" onclick="iptablesRules()">查看規則</button>
        <button class="btn danger" onclick="iptablesClear()">Clear</button>
    </div>
    <div id="r-iptables" class="result"></div>
</div>
<div class="card">
    <div class="card-title">DNS 修復</div>
    <div class="card-desc">修復 sing-box DNS 配置為 1.12 兼容格式</div>
    <button class="btn" onclick="execAction('fix-dns','r-dns')">Fix DNS</button>
    <div id="r-dns" class="result"></div>
</div>
<script>
async function api(action, body) {
    const payload = { action: action };
    if (body !== undefined) {
        Object.assign(payload, body);
    }
    const opts = {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(payload),
    };
    const res = await fetch('/api', opts);
    return res.json();
}
function show(id, text, ok) {
    const el = document.getElementById(id);
    el.textContent = text;
    el.className = 'result ' + (ok ? 'ok' : 'err');
}
async function execAction(action, resultId) {
    const btn = event.target;
    btn.disabled = true;
    try {
        const d = await api(action);
        if (d.ok) {
            show(resultId, 'OK\n' + (d.stdout||'') + (d.stderr?'\nstderr: '+d.stderr:''), true);
        } else {
            show(resultId, 'FAIL\n' + (d.error||'') + (d.stdout?'\n'+d.stdout:'') + (d.stderr?'\nstderr: '+d.stderr:''), false);
        }
    } catch(e) {
        show(resultId, 'ERROR: ' + e.message, false);
    }
    btn.disabled = false;
}
async function refreshStatus() {
    try {
        const d = await api('status');
        const set = (id, key, fmt) => {
            const el = document.getElementById(id);
            const val = d[key];
            const text = fmt ? fmt(val) : (val ? 'active' : 'disabled');
            el.textContent = text;
            el.className = 'status-value ' + (val ? 'on' : 'off');
        };
        set('sb', 'singbox_running', v => v ? 'running '+d.singbox_info : 'stopped');
        set('tun', 'tun_active', v => v ? 'active' : 'disabled');
        set('tp', 'tproxy_active', v => v ? 'active' : 'disabled');
        set('net', 'direct_net', v => v ? 'reachable' : 'blocked');
        set('ol', 'container_ollama', v => v ? 'connected' : 'disconnected');
        document.getElementById('ts').textContent = 'updated ' + new Date().toLocaleTimeString();
    } catch(e) {
        document.getElementById('ts').textContent = 'error: ' + e.message;
    }
}
setInterval(refreshStatus, 5000);
refreshStatus();
loadIptablesConfig();
function iptablesForm() {
    return {
        interface:     document.getElementById('i-iface').value,
        tproxy_port:   parseInt(document.getElementById('i-port').value) || 10808,
        router_ip:     document.getElementById('i-router').value,
        lan_subnet:    document.getElementById('i-lan').value,
        tproxy_80:     document.getElementById('i-tproxy80').checked,
        tproxy_443:    document.getElementById('i-tproxy443').checked,
        dns_forward:   document.getElementById('i-dns').checked,
        masquerade:    document.getElementById('i-masq').checked,
        forward:       document.getElementById('i-fwd').checked,
        exclude_self:  document.getElementById('i-excl').checked,
    };
}
async function iptablesSave() {
    const cfg = iptablesForm();
    const r = await api('iptables-save', { iptables: cfg });
    show('r-iptables', (r.ok ? 'SAVED\n' : 'FAIL\n') + (r.stdout||r.error||''), r.ok);
}
async function iptablesApply() {
    const r = await api('iptables-apply');
    show('r-iptables', (r.ok ? 'APPLIED\n' : 'FAIL\n') + (r.stdout||'') + (r.stderr?'stderr: '+r.stderr:''), r.ok);
    refreshStatus();
}
async function iptablesClear() {
    const r = await api('iptables-clear');
    show('r-iptables', (r.ok ? 'CLEARED\n' : 'FAIL\n') + (r.stdout||''), r.ok);
    refreshStatus();
}
async function iptablesRules() {
    const r = await api('iptables-rules');
    show('r-iptables', 'RULES:\n' + (r.stdout||'') + (r.stderr?'stderr: '+r.stderr:''), r.ok);
}
async function loadIptablesConfig() {
    try {
        const r = await api('iptables-get');
        // 如果返到全部 default 值（無配置文件），保留 HTML 預設勾選
        if (r.interface === "enp4s0f0" && r.tproxy_port === 10808 && r.router_ip === "192.168.31.1") {
            return; // 無配置文件，唔覆蓋 HTML checked 預設
        }
        document.getElementById('i-iface').value = r.interface || '';
        document.getElementById('i-port').value = r.tproxy_port || '';
        document.getElementById('i-router').value = r.router_ip || '';
        document.getElementById('i-lan').value = r.lan_subnet || '';
        document.getElementById('i-tproxy80').checked = r.tproxy_80 || false;
        document.getElementById('i-tproxy443').checked = r.tproxy_443 || false;
        document.getElementById('i-dns').checked = r.dns_forward || false;
        document.getElementById('i-masq').checked = r.masquerade || false;
        document.getElementById('i-fwd').checked = r.forward || false;
        document.getElementById('i-excl').checked = r.exclude_self || false;
    } catch(e) {
        // 無配置文件，保留 HTML 預設值
    }
}
</script>
</body>
</html>`
}

