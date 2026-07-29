#!/bin/bash
# y-ui deploy: build locally → upload → atomically replace running binary
# Usage: bash deploy.sh
set -euo pipefail

VPS="${1:-23.94.147.211}"
PORT="${2:-20022}"
VERIFY_PORT="${3:-18080}"
BIN_NAME="y-ui-linux"
REMOTE="/opt/y-ui/y-ui"

SSH_PASS="${SSH_PASS:-}"
if [ -z "$SSH_PASS" ]; then
  echo "ERROR: set SSH_PASS env var first (e.g. export SSH_PASS=yourpassword)"; exit 1
fi

C() {
  sshpass -p "$SSH_PASS" ssh -p "$PORT" \
    -o StrictHostKeyChecking=no "root@$VPS" "$1"
}

cd "$(dirname "$0")"

echo "=== [1/5] Build ==="
rm -f "$BIN_NAME"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$BIN_NAME" ./cmd/
LOCAL_SHA=$(sha256sum "$BIN_NAME" | cut -d' ' -f1)
echo "Built: $LOCAL_SHA"

echo "=== [2/5] JS syntax check (embedded) ==="
python3 - <<'PYEOF'
import re
s = open('internal/web/server.go', encoding='utf-8').read()
bt = s.index('return `')
start = s.index('<script>', bt) + len('<script>')
end = s.index('</script>', start)
js = s[start:end]
open('/tmp/precheck.js','w',encoding='utf-8').write(js)
PYEOF
node --check /tmp/precheck.js
echo "JS syntax OK ✓"

echo "=== [3/5] Upload (base64 pipe) ==="
cat "$BIN_NAME" | base64 | C "base64 -d > /tmp/y-ui-up && chmod +x /tmp/y-ui-up"
REMOTE_SHA=$(C "sha256sum /tmp/y-ui-up | cut -d' ' -f1")
if [ "$LOCAL_SHA" != "$REMOTE_SHA" ]; then
  echo "SHA MISMATCH ($LOCAL_SHA vs $REMOTE_SHA) - ABORTING"; exit 1
fi
echo "SHA match ✓"

echo "=== [4/5] Kill old + start new ==="
C "pkill -9 y-ui 2>/dev/null || true; sleep 2"
C "echo '=== after kill ===' && pgrep y-ui || echo 'no y-ui running'"
C "cp /tmp/y-ui-up $REMOTE && chmod +x $REMOTE && rm -f /tmp/y-ui-up"
C "$REMOTE -port 8080 > /tmp/yui.log 2>&1 & echo NEWPID=\$!"
sleep 2
C "netstat -tlnp | grep 8080 || true && echo '--- log ---' && cat /tmp/yui.log || echo 'no log yet'"

echo "=== [5/5] Verify rendered JS + API ==="
curl -s "http://$VPS:$VERIFY_PORT/" > /tmp/r.html
python3 - <<'PYEOF'
import re
html=open('/tmp/r.html').read()
m=re.search(r'<script>(.*?)</script>',html,re.S)
js=m.group(1) if m else ''
open('/tmp/rendered.js','w').write(js)
print('rendered len:', len(js))
PYEOF
node --check /tmp/rendered.js && echo "Rendered JS OK ✓" || { echo "Rendered JS ERROR ✗"; exit 1; }
API=$(curl -s -X POST "http://$VPS:$VERIFY_PORT/api" -H 'Content-Type: application/json' -d '{"action":"status"}')
echo "API: $API"
echo "=== DEPLOY DONE ✓ ==="
