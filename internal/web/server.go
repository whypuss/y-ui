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
<div class="btn-group"><button class="btn" onclick="iptablesSave()">保存配置</button><button class="btn success" onclick="iptablesApply()">Apply</button><button class="btn" onclick="iptablesRules()">查看規則</button><button class="btn danger" onclick="iptablesClear()">Clear</button></div>
<div id="r-iptables" class="result"></div>
</div>
</div>
<div id="page-nodes" class="page">
<h1>節點</h1><p class="subtitle">查看 sing-box 監聽端口與外網出節點</p>
<div class="card"><div class="card-title">Local Listeners</div><div class="card-desc">sing-box config.json 入 face 監聽端口</div><table class="node-table" id="listener-table"><tr><th>端口</th><th>類型</th><th>Tag</th><th>TLS</th></tr></table><div id="r-listeners" class="result"></div></div>
<div class="card"><div class="card-title">外網出節點</div><div class="card-desc">us-proxy / jp-proxy outbound 列表</div><table class="node-table" id="node-table"><tr><th>協議</th><th>Tag</th><th>Server</th><th>端口</th><th>Server Name</th></tr></table><div id="r-nodes" class="result"></div></div>
<div class="card"><div class="card-title">AnyTLS 節點</div><div class="card-desc">當前 AnyTLS outbound 配置（一鍵複製）</div><div id="anytls-detail" class="result ok" style="display:block;color:#8b949e">載入中...</div></div>
</div>
</div>
<script>
function showPage(name,el){document.querySelectorAll('.page').forEach(p=>p.classList.remove('active'));document.getElementById('page-'+name).classList.add('active');document.querySelectorAll('.nav-item').forEach(n=>n.classList.remove('active'));if(el)el.classList.add('active');if(name==='nodes')loadNodes()}
async function api(action,body){const p={action};if(body!==undefined)Object.assign(p,body);const r=await fetch('/api',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(p)});return r.json()}
function show(id,text,ok){const e=document.getElementById(id);e.textContent=text;e.className='result '+(ok?'ok':'err')}
function copyText(t){navigator.clipboard&&navigator.clipboard.writeText(t).then(()=>{}).catch(()=>{})}
async function execAction(action,rid){const b=event.target;b.disabled=true;try{const d=await api(action);if(d.ok)show(rid,'OK\n'+(d.stdout||'')+(d.stderr?'\nstderr: '+d.stderr:''),true);else show(rid,'FAIL\n'+(d.error||'')+(d.stdout?'\n'+d.stdout:'')+(d.stderr?'\nstderr: '+d.stderr:''),false)}catch(e){show(rid,'ERROR: '+e.message,false)}b.disabled=false}
async function refreshStatus(){try{const d=await api('status');const set=(id,k,f)=>{const e=document.getElementById(id);const v=d[k];e.textContent=f?f(v):(v?'active':'disabled');e.className='status-value '+(v?'on':'off')};set('sb','singbox_running',v=>v?'running '+d.singbox_info:'stopped');set('tun','tun_active');set('tp','tproxy_active');set('net','direct_net');document.getElementById('ts').textContent='updated '+new Date().toLocaleTimeString()}catch(e){document.getElementById('ts').textContent='error: '+e.message}}
setInterval(refreshStatus,5000);refreshStatus();loadIptablesConfig()
function iptablesForm(){return{interface:document.getElementById('i-iface').value,tproxy_port:parseInt(document.getElementById('i-port').value)||10808,router_ip:document.getElementById('i-router').value,lan_subnet:document.getElementById('i-lan').value,tproxy_80:document.getElementById('i-tproxy80').checked,tproxy_443:document.getElementById('i-tproxy443').checked,dns_forward:document.getElementById('i-dns').checked,masquerade:document.getElementById('i-masq').checked,forward:document.getElementById('i-fwd').checked,exclude_self:document.getElementById('i-excl').checked}}
async function iptablesSave(){const c=iptablesForm();const r=await api('iptables-save',{iptables:c});show('r-iptables',(r.ok?'SAVED\n':'FAIL\n')+(r.stdout||r.error||''),r.ok)}
async function iptablesApply(){const r=await api('iptables-apply');show('r-iptables',(r.ok?'APPLIED\n':'FAIL\n')+(r.stdout||'')+(r.stderr?'stderr: '+r.stderr:''),r.ok);refreshStatus()}
async function iptablesClear(){const r=await api('iptables-clear');show('r-iptables',(r.ok?'CLEARED\n':'FAIL\n')+(r.stdout||''),r.ok);refreshStatus()}
async function iptablesRules(){const r=await api('iptables-rules');show('r-iptables','RULES:\n'+(r.stdout||'')+(r.stderr?'stderr: '+r.stderr:''),r.ok)}
async function loadIptablesConfig(){try{const r=await api('iptables-get');if(r.interface==='enp4s0f0'&&r.tproxy_port===10808&&r.router_ip==='192.168.31.1')return;document.getElementById('i-iface').value=r.interface||'';document.getElementById('i-port').value=r.tproxy_port||'';document.getElementById('i-router').value=r.router_ip||'';document.getElementById('i-lan').value=r.lan_subnet||'';document.getElementById('i-tproxy80').checked=r.tproxy_80||false;document.getElementById('i-tproxy443').checked=r.tproxy_443||false;document.getElementById('i-dns').checked=r.dns_forward||false;document.getElementById('i-masq').checked=r.masquerade||false;document.getElementById('i-fwd').checked=r.forward||false;document.getElementById('i-excl').checked=r.exclude_self||false}catch(e){}}
async function loadNodes(){
try{const ls=await api('nodes-inbound');const t=document.getElementById('listener-table');t.innerHTML='<tr><th>端口</th><th>類型</th><th>Tag</th><th>TLS</th></tr>';if(ls.error){document.getElementById('r-listeners').textContent=ls.error;document.getElementById('r-listeners').style.display='block'}else{ls.listeners.forEach(l=>{const c=l.type==='anytls'?'anytls':l.type==='vless'?'vless':l.type==='socks'?'socks':l.type==='mixed'?'mixed':'other';t.innerHTML+='<tr><td class="port">'+l.port+'</td><td class="proto proto-'+c+'">'+l.type+'</td><td class="tag">'+l.tag+'</td><td>'+l.tls+'</td></tr>'})}}}catch(e){}
try{const ns=await api('nodes-list');const t=document.getElementById('node-table');t.innerHTML='<tr><th>協議</th><th>Tag</th><th>Server</th><th>端口</th><th>Server Name</th></tr>';if(ns&&ns.length>0){ns.forEach(n=>{const c=n.type==='anytls'?'anytls':n.type==='vless'?'vless':n.type==='socks'?'socks':n.type==='mixed'?'mixed':'other';t.innerHTML+='<tr><td class="proto proto-'+c+'">'+(n.protocol||n.type)+'</td><td class="tag">'+n.tag+'</td><td>'+n.server+'</td><td class="port">'+n.port+'</td><td>'+n.server_name+'</td></tr>'})}}}catch(e){}
try{const a=await api('anytls-node');const e=document.getElementById('anytls-detail');if(a.server){const d='類型: AnyTLS\nServer: '+a.server+'\n端口: '+a.port+'\nTLS SN: '+a.server_name+'\nTag: '+a.tag;const cp=a.server+':'+a.port;e.innerHTML='<div style="color:#58a6ff;margin-bottom:4px">'+d.replace(/\n/g,'<br>')+'</div><div class="copy-row"><div class="copy-box">'+cp+'</div><button class="btn-copy" onclick="copyText(''+cp+'')">📋 複製</button></div>'}else{e.innerHTML='無 AnyTLS 節點配置'}}}catch(e){document.getElementById('anytls-detail').textContent='載入失敗'}
}
</script>
</body>
</html>`
}


