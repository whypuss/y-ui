#!/usr/bin/env python3
"""Sing-Box 网络控制面板 — 极简,标准库,零依赖"""
import http.server, json, subprocess, time, os, threading

PORT = int(os.environ.get("SINGBOX_PANEL_PORT", "19999"))
PANEL_DIR = os.path.dirname(os.path.abspath(__file__))

SCRIPTS = {
    "tproxy_on": os.path.join(PANEL_DIR, "tproxy-on.sh"),
    "tproxy_off": os.path.join(PANEL_DIR, "tproxy-off.sh"),
    "iptables_clear": os.path.join(PANEL_DIR, "iptables-clear.sh"),
    "restart_singbox": os.path.join(PANEL_DIR, "restart-singbox.sh"),
}


def run_script(name):
    path = SCRIPTS.get(name)
    if not path or not os.path.exists(path):
        return {"ok": False, "error": "script not found: %s" % path}
    try:
        r = subprocess.run(["bash", path], capture_output=True, text=True, timeout=30)
        return {
            "ok": r.returncode == 0,
            "stdout": r.stdout.strip()[-500:],
            "stderr": r.stderr.strip()[-500:],
            "rc": r.returncode,
        }
    except subprocess.TimeoutExpired:
        return {"ok": False, "error": "timeout"}
    except Exception as e:
        return {"ok": False, "error": str(e)}


def get_status():
    out = {
        "singbox_running": False,
        "tun_active": False,
        "tproxy_active": False,
        "tun_info": "",
    }

    # sing-box main process
    r = subprocess.run(["ps", "aux"], capture_output=True, text=True)
    for line in r.stdout.splitlines():
        if "sing-box run -c /etc/sing-box/config.json" in line and "grep" not in line:
            out["singbox_running"] = True
            break

    # TUN device
    r2 = subprocess.run(["ip", "addr", "show"], capture_output=True, text=True)
    for line in r2.stdout.splitlines():
        if "tun" in line and "tunnel" not in line:
            out["tun_active"] = True
            out["tun_info"] = line.strip()
            break

    # TProxy port 10808
    r3 = subprocess.run(["ss", "-tlnp"], capture_output=True, text=True)
    for line in r3.stdout.splitlines():
        if "10808" in line:
            out["tproxy_active"] = True
            break

    return out


def build_html():
    return '''<!DOCTYPE html>
<html lang="zh-Hant">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sing-Box Control Panel</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: #0f1117; color: #e1e4e8;
    padding: 20px; max-width: 640px; margin: 0 auto;
}
h1 { text-align: center; margin-bottom: 6px; color: #58a6ff; font-size: 1.5em; }
.subtitle { text-align: center; color: #8b949e; font-size: 0.85em; margin-bottom: 20px; }
.status-bar {
    background: #161b22; border: 1px solid #30363d; border-radius: 8px;
    padding: 14px 16px; margin-bottom: 12px;
    display: flex; justify-content: space-between; align-items: center;
}
.status-label { font-size: 0.9em; color: #8b949e; }
.status-value { font-size: 1.05em; font-weight: 600; }
.on { color: #3fb950; }
.off { color: #f85149; }
.card {
    background: #161b22; border: 1px solid #30363d;
    border-radius: 8px; padding: 16px; margin-bottom: 12px;
}
.card-title { font-size: 1em; color: #58a6ff; margin-bottom: 4px; }
.card-desc { font-size: 0.8em; color: #8b949e; margin-bottom: 12px; }
.btn {
    background: #21262d; border: 1px solid #30363d; color: #e1e4e8;
    padding: 10px 14px; border-radius: 6px; cursor: pointer;
    font-size: 0.9em; width: 100%; transition: all 0.15s;
}
.btn:hover { background: #30363d; border-color: #58a6ff; }
.btn.danger { border-color: #f85149; color: #f85149; }
.btn.danger:hover { background: rgba(248,81,73,0.1); }
.btn.success { border-color: #3fb950; color: #3fb950; }
.btn.success:hover { background: rgba(63,185,80,0.1); }
.btn:disabled { opacity: 0.4; cursor: not-allowed; }
.btn-group { display: flex; gap: 8px; }
.btn-group .btn { flex: 1; }
.result {
    margin-top: 10px; padding: 10px;
    background: #0d1117; border-radius: 6px;
    font-size: 0.8em; color: #8b949e;
    white-space: pre-wrap; max-height: 160px; overflow-y: auto;
    display: none;
}
.result.ok { color: #3fb950; display: block; }
.result.err { color: #f85149; display: block; }
.updating { color: #58a6ff; font-size: 0.75em; text-align: center; margin-top: 4px; }
</style>
</head>
<body>
<h1>Control Panel</h1>
<p class="subtitle">Maxwell VPS - Network Recovery</p>

<div class="status-bar">
    <span class="status-label">sing-box</span>
    <span id="sb" class="status-value off">checking...</span>
</div>
<div class="status-bar">
    <span class="status-label">TUN</span>
    <span id="tun" class="status-value off">checking...</span>
</div>
<div class="status-bar">
    <span class="status-label">TProxy</span>
    <span id="tp" class="status-value off">checking...</span>
</div>
<p class="updating" id="ts">refreshing...</p>

<div class="card">
    <div class="card-title">TProxy</div>
    <div class="card-desc">Toggle TProxy - turning off restores direct network</div>
    <div class="btn-group">
        <button class="btn success" onclick="exec('tproxy-on')">Turn On</button>
        <button class="btn danger" onclick="exec('tproxy-off')">Turn Off</button>
    </div>
    <div id="r1" class="result"></div>
</div>

<div class="card">
    <div class="card-title">iptables</div>
    <div class="card-desc">Clear all iptables rules - restore system defaults</div>
    <button class="btn danger" onclick="exec('iptables-clear')">Clear iptables</button>
    <div id="r2" class="result"></div>
</div>

<div class="card">
    <div class="card-title">sing-box</div>
    <div class="card-desc">Restart sing-box main process (requires sudo)</div>
    <button class="btn" onclick="exec('restart-singbox')">Restart</button>
    <div id="r3" class="result"></div>
</div>

<div class="card">
    <div class="card-title">Status</div>
    <div class="card-desc">Check current network state</div>
    <button class="btn" onclick="exec('status')">Check Status</button>
    <div id="r4" class="result"></div>
</div>

<script>
const RESULTS = {'tproxy-on':'r1','tproxy-off':'r1','iptables-clear':'r2','restart-singbox':'r3','status':'r4'};
function api(action, body) {
    return fetch('/api', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({action: action} + (body || {}))
    }).then(r => r.json());
}
function show(id, text, ok) {
    var el = document.getElementById(id);
    el.textContent = text;
    el.className = 'result ' + (ok ? 'ok' : 'err');
}
function exec(action) {
    api(action).then(function(d) {
        var rid = RESULTS[action] || 'r1';
        if (action === 'status') {
            show(rid, JSON.stringify(d, null, 2), true);
        } else {
            var msg = d.ok
                ? 'OK\\n' + (d.stdout || '') + (d.stderr ? '\\nstderr: ' + d.stderr : '')
                : 'FAIL ' + d.rc + '\\n' + (d.error || '') + (d.stderr ? '\\nstderr: ' + d.stderr : '');
            show(rid, msg, d.ok);
        }
    });
}
function refreshStatus() {
    api('status').then(function(d) {
        var sb = document.getElementById('sb');
        sb.textContent = d.singbox_running ? 'running' : 'stopped';
        sb.className = 'status-value ' + (d.singbox_running ? 'on' : 'off');
        var t = document.getElementById('tun');
        t.textContent = d.tun_active ? 'active ' + (d.tun_info || '') : 'disabled';
        t.className = 'status-value ' + (d.tun_active ? 'on' : 'off');
        var p = document.getElementById('tp');
        p.textContent = d.tproxy_active ? 'active' : 'disabled';
        p.className = 'status-value ' + (d.tproxy_active ? 'on' : 'off');
        document.getElementById('ts').textContent = 'updated ' + new Date().toLocaleTimeString();
    });
}
setInterval(refreshStatus, 5000);
refreshStatus();
</script>
</body>
</html>'''


def serve_forever():
    handler = type("H", (http.server.BaseHTTPRequestHandler,), {
        "do_GET": lambda self: self._get(),
        "do_POST": lambda self: self._post(),
        "log_message": lambda self, fmt, *a: print("[%s] %s" % (time.strftime("%H:%M:%S"), a[0] if a else "")),
    })

    def _get(self):
        if self.path in ("/", "/index.html"):
            self._send_html()
        else:
            self._json({"error": "not found"}, 404)

    def _post(self):
        cl = self.headers.get("Content-Length")
        data = {}
        if cl:
            raw = self.rfile.read(int(cl))
            try:
                data = json.loads(raw)
            except Exception:
                data = {}
        action = data.get("action", "")
        if action == "status":
            self._json(get_status())
        elif action in SCRIPTS:
            self._json(run_script(action))
        else:
            self._json({"error": "unknown action: %s" % action}, 400)

    def _json(self, d, code=200):
        self.send_response(code)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.end_headers()
        self.wfile.write(json.dumps(d, ensure_ascii=False).encode())

    def _send_html(self):
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.end_headers()
        self.wfile.write(build_html().encode())

    # monkey-patch methods into handler
    handler._get = _get
    handler._post = _post
    handler._json = _json
    handler._send_html = _send_html

    print("Sing-Box Panel starting on 0.0.0.0:%d ..." % PORT)
    print("Access: http://<VPS_IP>:%d/" % PORT)
    print("Panel dir: %s" % PANEL_DIR)
    http.server.HTTPServer(("0.0.0.0", PORT), handler).serve_forever()


if __name__ == "__main__":
    serve_forever()
