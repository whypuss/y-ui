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

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	var req struct{ Action string `json:"action"` }
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
	case "iptables-clear":
		result = exec.ClearIptables(nil)
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
    <div class="card-title">iptables</div>
    <div class="card-desc">清除所有 iptables 規則</div>
    <button class="btn danger" onclick="execAction('iptables-clear','r-iptables')">Clear iptables</button>
    <div id="r-iptables" class="result"></div>
</div>
<div class="card">
    <div class="card-title">DNS 修復</div>
    <div class="card-desc">修復 sing-box DNS 配置為 1.12 兼容格式</div>
    <button class="btn" onclick="execAction('fix-dns','r-dns')">Fix DNS</button>
    <div id="r-dns" class="result"></div>
</div>
<script>
async function api(action) {
    const res = await fetch('/api', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({action: action})
    });
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
</script>
</body>
</html>`
}

