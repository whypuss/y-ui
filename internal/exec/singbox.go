package exec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// SystemStatus 系統狀態
type SystemStatus struct {
	SingboxRunning  bool   `json:"singbox_running"`
	SingboxInfo     string `json:"singbox_info"`
	TunActive       bool   `json:"tun_active"`
	TunInfo         string `json:"tun_info"`
	TproxyActive    bool   `json:"tproxy_active"`
	DirectNet       bool   `json:"direct_net"`
	ContainerOllama bool   `json:"container_ollama"`
}

// CommandResult 指令執行結果
type CommandResult struct {
	Ok     bool   `json:"ok"`
	Stdout string `json:"stdout,omitempty"`
	Stderr string `json:"stderr,omitempty"`
	Rc     int    `json:"rc"`
	Error  string `json:"error,omitempty"`
}

// TestSudo 測試 sudo
func TestSudo() error {
	cmd := exec.Command("sudo", "-n", "true")
	_, err := cmd.Output()
	if err != nil && strings.Contains(err.Error(), "must be run") {
		return nil
	}
	return err
}

// RunCommand 執行指令，自動檢測需要 sudo 的指令
func RunCommand(command string) CommandResult {
	args := strings.Fields(command)
	if len(args) == 0 {
		return CommandResult{Ok: false, Error: "empty command"}
	}

	// 檢查 sudo 前綴
	needSudo := false
	argsToRun := args
	if strings.HasPrefix(args[0], "sudo:") {
		needSudo = true
		argsToRun = args[1:]
	}

	// 預設需要 sudo 的指令
	if !needSudo {
		sudoCmds := []string{"iptables", "ip6tables", "ip", "pkill", "nohup", "kill", "killall"}
		for _, sc := range sudoCmds {
			if args[0] == sc {
				needSudo = true
				break
			}
		}
	}

	var argsFull []string
	if needSudo && len(argsToRun) > 0 {
		argsFull = append([]string{"-n"}, argsToRun...)
	} else {
		argsFull = args
	}

	cmd := exec.Command(argsFull[0], argsFull[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		rc := 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			rc = exitErr.ExitCode()
		}
		// sudo 需要密碼 → 嘗試 SINGBOX_SUDO_PASS
		if needSudo && rc != 0 {
			if r := runWithSudoPass(context.Background(), argsToRun); r.Ok {
				return r
			}
		}
	errMsg := err.Error()
	if len(errMsg) > 100 {
		errMsg = errMsg[:100]
	}
	return CommandResult{
		Ok:     false,
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		Rc:     rc,
		Error:  fmt.Sprintf("exit %d: %s", rc, errMsg),
	}
	}

	return CommandResult{
		Ok:     true,
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		Rc:     0,
	}
}

// runWithSudoPass 用密碼執行 sudo
// readPassword 讀取 sudo 密碼：環境變量 → 文件
func readPassword() string {
	p := os.Getenv("SINGBOX_SUDO_PASS")
	if p == "" {
		p = os.Getenv("SUDO_PASS")
	}
	if p == "" {
		// 嘗試讀取 /tmp/.yui-pass
		data, err := os.ReadFile("/tmp/.yui-pass")
		if err == nil {
			p = strings.TrimSpace(string(data))
		}
	}
	return p
}

func runWithSudoPass(ctx context.Context, args []string) CommandResult {
	if len(args) == 0 {
		return CommandResult{Ok: false, Error: "empty command"}
	}

	password := readPassword()
	if password == "" {
		return CommandResult{Ok: false, Error: "SINGBOX_SUDO_PASS not set (no env var and no /tmp/.yui-pass)"}
	}

	ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	argsFull := append([]string{"-S"}, args...)
	cmd := exec.CommandContext(ctx2, "sudo", argsFull[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	pw, _ := cmd.StdinPipe()
	go func() {
		pw.Write([]byte(password + "\n"))
		pw.Close()
	}()

	err := cmd.Run()
	if err != nil {
		rc := 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			rc = exitErr.ExitCode()
		}
		return CommandResult{
			Ok:     false,
			Stdout: stdout.String(),
			Stderr: stderr.String(),
			Rc:     rc,
			Error:  fmt.Sprintf("exit %d", rc),
		}
	}

	return CommandResult{
		Ok:     true,
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		Rc:     0,
	}
}

// GetSystemStatus 取得系統狀態
func GetSystemStatus() SystemStatus {
	s := SystemStatus{}

	// sing-box 主進程
	s.SingboxRunning, s.SingboxInfo = checkSingboxProcess()

	// TUN
	s.TunActive, s.TunInfo = checkTun()

	// TProxy
	s.TproxyActive = checkTproxy()

	// 外網可達
	s.DirectNet = checkDirectNet()

	// 容器連 Mac Ollama
	s.ContainerOllama = checkContainerOllama()

	return s
}

func checkSingboxProcess() (bool, string) {
	cmd := exec.Command("sh", "-c", `ps aux | grep "sing-box run -c /etc/sing-box/config.json" | grep -v grep | head -1`)
	out, _ := cmd.Output()
	line := strings.TrimSpace(string(out))
	if line != "" {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			pid := parts[1]
			uptimeStr := ""
			if len(parts) >= 9 {
				uptimeStr = parts[8]
			}
			return true, fmt.Sprintf("PID %s, uptime %s", pid, uptimeStr)
		}
		return true, "running"
	}

	cmd = exec.Command("sh", "-c", `ps aux | grep "sing-box" | grep -v grep | head -3`)
	out, _ = cmd.Output()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > 0 && lines[0] != "" {
		return true, fmt.Sprintf("other (%d processes)", len(lines))
	}
	return false, "stopped"
}

func checkTun() (bool, string) {
	// TUN = kernel interface tun0；存在即表示 TUN 啟用
	cmd := exec.Command("sh", "-c", `ip link show tun0 2>/dev/null`)
	out, _ := cmd.Output()
	if strings.Contains(string(out), "tun0") {
		return true, "tun0 active"
	}
	return false, "disabled (no tun0 interface)"
}

func checkTproxy() bool {
	// 檢查 iproute TPROXY rule (fwmark 0x1 → table 100)，唔需要 sudo
	cmd := exec.Command("sh", "-c", `ip rule show 2>/dev/null | grep -c "fwmark 0x1/0x1 lookup 100"`)
	out, _ := cmd.Output()
	n := strings.TrimSpace(string(out))
	return n != "0"
}

func checkDirectNet() bool {
	cmd := exec.Command("sh", "-c", `curl -s -m 3 -o /dev/null -w "%{http_code}" https://www.google.com 2>/dev/null`)
	out, _ := cmd.Output()
	code := strings.TrimSpace(string(out))
	return code == "200" || code == "301" || code == "302"
}

func checkContainerOllama() bool {
	cmd := exec.Command("sh", "-c", `docker exec mem0-dev-mem0-1 timeout 5 python3 -c "import socket; s=socket.socket(); s.settimeout(3); s.connect(('192.168.31.111',11434)); print('OK')" 2>/dev/null`)
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out)) == "OK"
}

// FixSingboxDNS 修復 sing-box DNS 配置為 1.12 兼容格式
func FixSingboxDNS(ctx context.Context) CommandResult {
	script := `python3 << 'PYEOF'
import json
path = "/etc/sing-box/config.json"
with open(path) as f:
    cfg = json.load(f)
for k in ["dns", "dns-servers"]:
    cfg.pop(k, None)
cfg["dns"] = {
    "servers": [
        {"tag": "dns-direct", "address": "8.8.8.8", "address_strategy": "ipv4_only"}
    ],
    "final": "dns-direct",
    "strategy": "ipv4_only"
}
with open(path, "w") as f:
    json.dump(cfg, f, indent=2)
print("DNS config fixed for sing-box 1.12")
PYEOF
`

	cmd := exec.Command("sh", "-c", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr


	err := cmd.Run()
	if err != nil {
		return CommandResult{
			Ok:     false,
			Stdout: stdout.String(),
			Stderr: stderr.String(),
			Error:  fmt.Sprintf("fix failed: %s", err.Error()),
		}
	}
	return CommandResult{
		Ok:     true,
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
}

// RestartSingbox 重啟 sing-box（nohup 背景，不斷 SSH）
func RestartSingbox(ctx context.Context) CommandResult {
	fix := FixSingboxDNS(ctx)
	if !fix.Ok {
		return CommandResult{Ok: false, Error: "DNS fix failed: " + fix.Error}
	}

	// 殺所有舊 sing-box（需要 sudo）
	kill := runCommandWithSudo([]string{"sh", "-c", `ps -o pid= -C sing-box 2>/dev/null | xargs -r kill 2>/dev/null; sleep 1; echo "killed old sing-box"`})

	// 啟動新進程（需要 sudo）
	startScript := `sleep 2 && nohup env ENABLE_DEPRECATED_LEGACY_DNS_SERVERS=true /etc/sing-box/bin/sing-box run -c /etc/sing-box/config.json -C /etc/sing-box/conf > /var/log/sing-box.log 2>&1 & echo "started sing-box" && sleep 4 && ps aux | grep "sing-box run -c /etc/sing-box/config.json" | grep -v grep | head -1`
	start := runCommandWithSudo([]string{"sh", "-c", startScript})

	return CommandResult{
		Ok:     start.Ok,
		Stdout: fix.Stdout + "\n" + kill.Stdout + "\n" + start.Stdout,
		Stderr: fix.Stderr + "\n" + kill.Stderr + "\n" + start.Stderr,
	}
}

// fixSingboxAutoRoute 將 config.json 中 TUN inbound 嘅 auto_route/strict_route 設為 false
// 防止 sing-box 自動寫 policy routing rules（9000–9010），避免擾路由
func fixSingboxAutoRoute() CommandResult {
	script := `python3 << 'PYEOF'
import json
path = "/etc/sing-box/config.json"
with open(path) as f:
    c = json.load(f)
for ib in c.get("inbounds", []):
    if ib.get("type") == "tun":
        ib["auto_route"] = False
        ib["strict_route"] = False
        print("TUN auto_route=False strict_route=False")
        break
with open(path, "w") as f:
    json.dump(c, f, indent=2)
PYEOF
`
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("sh", "-c", script)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return CommandResult{Ok: false, Stdout: stdout.String(), Stderr: stderr.String(), Error: err.Error()}
	}
	return CommandResult{Ok: true, Stdout: stdout.String()}
}

func checkListenerOnPort(port int) bool {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("ss -tlnp 2>/dev/null | grep -c \":%d \"", port))
	out, _ := cmd.Output()
	n := strings.TrimSpace(string(out))
	return n != "0"
}

// startMainProcess 啟動 sing-box 主進程（config.json），帶 TProxy-Mixed :10808 listener
func startMainProcess() CommandResult {
	// 檢查主進程是否已運行
	cmd := exec.Command("sh", "-c", `ps aux | grep "sing-box run -c /etc/sing-box/config.json" | grep -v grep | wc -l`)
	out, _ := cmd.Output()
	if strings.TrimSpace(string(out)) != "0" {
		return CommandResult{Ok: true, Stdout: "main process already running"}
	}

	// 寫 config.json: auto_route=false + strict_route=false（唔寫 policy routing rules，唔擾路由）
	fix := FixSingboxDNS(context.Background())
	if !fix.Ok {
		return CommandResult{Ok: false, Error: "config fix failed: " + fix.Error}
	}
	_ = fix

	// 啟動 systemd service（有 environment + capabilities）
	r := runCommandWithSudo([]string{"sh", "-c", `
echo "starting sing-box-main.service"
systemctl restart sing-box-main.service
sleep 3
systemctl is-active sing-box-main.service
ps aux | grep "sing-box run -c /etc/sing-box/config.json" | grep -v grep | head -1
`})
	return r
}

// TproxyOn 啟用 TProxy
func TproxyOn(ctx context.Context) CommandResult {
	// 防循環: TUN 開住唔准開 TProxy
	s := GetSystemStatus()
	if s.TunActive {
		return CommandResult{Ok: false, Error: "TUN is active — disable TUN before enabling TProxy (mutually exclusive)"}
	}

	_ = ctx

	// 1. 確保主進程運行，10808 listener 在聽（TPROXY 重定向目標）
	if !checkListenerOnPort(10808) {
		// 自動啟動主進程（帶 auto_route=false，唔寫 policy rules 擾路由）
		_ = FixSingboxDNS(ctx)
		_ = fixSingboxAutoRoute()
		sm := startMainProcess()
		if !sm.Ok {
			return CommandResult{Ok: false, Error: "Failed to start main process: " + sm.Stderr + "\n" + sm.Stdout, Stderr: sm.Stderr, Stdout: sm.Stdout}
		}
		// 等 listener 起來
		time.Sleep(2 * time.Second)
	}
	if !checkListenerOnPort(10808) {
		return CommandResult{Ok: false, Error: "Port 10808 still has no listener after start attempt"}
	}

	// 2. 寫入 mangle TPROXY rules + iproute
	script := `/etc/tproxy-rules.sh`
	r := runCommandWithSudo([]string{"bash", "-c", script})

	// 3. 驗證規則真生效
	time.Sleep(500 * time.Millisecond)
	v := runCommandWithSudo([]string{"sh", "-c", `iptables -t mangle -L PREROUTING -n 2>/dev/null | grep -c TPROXY; ip rule show 2>/dev/null | grep -c "fwmark 0x1" || true`})
	count := strings.TrimSpace(strings.Fields(v.Stdout)[0])

	if !r.Ok {
		return CommandResult{Ok: false, Stdout: r.Stdout, Stderr: r.Stderr, Error: "tproxy-rules.sh failed"}
	}
	if count == "0" {
		return CommandResult{Ok: true, Stdout: r.Stdout, Stderr: r.Stderr + "\n[WARN] TPROXY rules not found in mangle — may need manual config"}
	}

	return CommandResult{Ok: true, Stdout: r.Stdout + fmt.Sprintf("\n[VERIFY] %s TPROXY rule(s) active", count), Stderr: r.Stderr}
}

// TproxyOff 關閉 TProxy
func TproxyOff(ctx context.Context) CommandResult {
	// 清理 mangle + 移除 iproute TPROXY rule
	script := `iptables -t mangle -F; ip6tables -t mangle -F 2>/dev/null; ip rule del fwmark 0x1/0x1 lookup 100 2>/dev/null; ip route del local 0.0.0.0/0 dev lo table 100 2>/dev/null; echo "TProxy disabled - mangle cleared, iproute rules removed"`
	r := runCommandWithSudo([]string{"sh", "-c", script})
	_ = ctx
	return r
}

// TunOn 啟動 sing-box TUN 主進程（帶 TUN）
func TunOn() CommandResult {
	// 防循環: TProxy 開住唔准開 TUN
	s := GetSystemStatus()
	if s.TproxyActive {
		return CommandResult{Ok: false, Error: "TProxy is active — disable TProxy before enabling TUN (mutually exclusive)"}
	}
	fix := FixSingboxDNS(context.Background())
	if !fix.Ok {
		return CommandResult{Ok: false, Error: "DNS fix failed: " + fix.Error}
	}

	// 檢查是否已在運行
	cmd := exec.Command("sh", "-c", `ps aux | grep "sing-box run -c /etc/sing-box/config.json" | grep -v grep | wc -l`)
	out, _ := cmd.Output()
	n := strings.TrimSpace(string(out))
	if n != "0" {
		return CommandResult{Ok: true, Stdout: "TUN already running"}
	}

	// 用 sudo 啟動 nohup（背景，不斷 SSH）
	startScript := `nohup env ENABLE_DEPRECATED_LEGACY_DNS_SERVERS=true /etc/sing-box/bin/sing-box run -c /etc/sing-box/config.json -C /etc/sing-box/conf > /var/log/sing-box.log 2>&1 & sleep 3 && ps aux | grep "sing-box run -c /etc/sing-box/config.json" | grep -v grep | head -1`
	r := runCommandWithSudo([]string{"sh", "-c", startScript})

	return CommandResult{
		Ok:     r.Ok,
		Stdout: "TUN started: " + r.Stdout,
		Stderr: r.Stderr,
	}
}

// TunOff 關閉 sing-box TUN 主進程（保留 SOCKS 代理）
// 同時清理 sing-box 寫入嘅 policy routing rules（9000–9010）同 table 2022，
// 避免殘留 rule 攔截所有流量去失效嘅 table 2022，令關 TUN 後依然斷網。
func TunOff() CommandResult {
	// 用 systemctl stop（唔會觸發 systemd Restart=always 自動復活）
	// 清理 sing-box auto_route 殘留嘅 policy routing rules + table 2022
	var buf bytes.Buffer
	buf.WriteString(`
echo "stopping sing-box-main.service (no auto-restart)..."
systemctl stop sing-box-main.service 2>/dev/null
sleep 2

# 強制清理 policy routing rules (9000-9010) + flush table 2022
for p in 9000 9001 9002 9003 9004 9005 9006 9007 9008 9009 9010 2022; do
    ip rule del priority $p 2>/dev/null || true
done
ip route flush table 2022 2>/dev/null || true

# 強制移除 tun0 接口（如果殘留）
if ip link show tun0 >/dev/null 2>&1; then
    ip link del tun0 2>/dev/null || true
fi

# 驗證
if ip link show tun0 >/dev/null 2>&1; then
    echo "WARNING: tun0 still exists"
else
    echo "TUN disabled - tun0 removed, rules cleaned"
fi

# 確認主進程死咗
ps aux | grep "sing-box run -c /etc/sing-box/config.json" | grep -v grep | wc -l
`)
	r := runCommandWithSudo([]string{"sh", "-c", buf.String()})
	return r
}

// runCommandWithSudo 強制用 sudo 執行單一指令
func runCommandWithSudo(args []string) CommandResult {
	if len(args) == 0 {
		return CommandResult{Ok: false, Error: "empty args"}
	}
	var stdout, stderr bytes.Buffer
	cmdArgs := append([]string{"-n"}, args...)
	cmd := exec.Command("sudo", cmdArgs...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		return CommandResult{Ok: true, Stdout: stdout.String(), Stderr: stderr.String()}
	}
	password := readPassword()
	if password == "" {
		return CommandResult{Ok: false, Error: "sudo failed + no SINGBOX_SUDO_PASS"}
	}
	cmdArgs = append([]string{"-S"}, args...)
	cmd = exec.Command("sudo", cmdArgs...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	pw, _ := cmd.StdinPipe()
	go func() {
		pw.Write([]byte(password + "\n"))
		pw.Close()
	}()
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return CommandResult{Ok: true, Stdout: stdout.String(), Stderr: stderr.String()}
		}
		return CommandResult{Ok: false, Stderr: stderr.String(), Error: "sudo command failed: " + err.Error()}
	}
	return CommandResult{Ok: true, Stdout: stdout.String(), Stderr: stderr.String()}
}

// ClearIptables 清除 iptables（清除後可能影響 Docker nat，提示用戶）
func ClearIptables(ctx context.Context) CommandResult {
	// 腳本已走 runCommandWithSudo，腳本內唔使 sudo
	script := `iptables -F; iptables -X; iptables -t nat -F; iptables -t nat -X; iptables -t mangle -F; iptables -t mangle -X; iptables -P INPUT ACCEPT; iptables -P FORWARD ACCEPT; iptables -P OUTPUT ACCEPT; echo "iptables cleared (filter+nat+mangle, policies=ACCEPT)"`
	r := runCommandWithSudo([]string{"sh", "-c", script})
	_ = ctx
	return CommandResult{Ok: r.Ok, Stdout: r.Stdout, Stderr: r.Stderr, Error: r.Error}
}

