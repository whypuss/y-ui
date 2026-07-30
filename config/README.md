# Config Templates

These are **templates** — they contain placeholder values and are NOT for production use.

**Before deploying:**
1. Copy to `/etc/sing-box/`
2. Replace all `REPLACE_*` placeholders with real values
3. Generate new UUIDs: `cat /proc/sys/kernel/random/uuid`
4. Generate new passwords: `openssl rand -base64 24`
5. Generate Reality key pair: use `openssl rand -hex 32` for private key
