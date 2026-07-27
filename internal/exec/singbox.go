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
	// 檢查 sing-box TUN device (tun0) 或 tproxy interface
	cmd := exec.Command("sh", "-c", `ip link show | grep -E "tun|tproxy" | head -3`)
	out, _ := cmd.Output()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			return true, l
		}
	}
	// 檢查 config.json 主進程（帶 TUN）
	cmd2 := exec.Command("sh", "-c", `ps aux | grep "sing-box run -c /etc/sing-box/config.json" | grep -v grep | wc -l`)
	out2, _ := cmd2.Output()
	n := strings.TrimSpace(string(out2))
	if n == "0" {
		return false, "disabled"
	}
	return true, "running (config.json)"
}

func checkTproxy() bool {
	cmd := exec.Command("sh", "-c", `ss -tlnp 2>/dev/null | grep 10808 | head -1`)
	out, _ := cmd.Output()
	return len(strings.TrimSpace(string(out))) > 0
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
		return CommandResult{
			Ok:    false,
			Error: "DNS fix failed: " + fix.Error,
		}
	}

	// 殺舊進程
	kill := RunCommand(`ps -o pid= -C sing-box 2>/dev/null | xargs -r kill 2>/dev/null; echo "killed old sing-box"`)
	if !kill.Ok {
		kill.Stdout = "attempted kill\n"
	}

	// 啟動新進程
	startCmd := `sleep 2 && nohup env ENABLE_DEPRECATED_LEGACY_DNS_SERVERS=true /etc/sing-box/bin/sing-box run -c /etc/sing-box/config.json -C /etc/sing-box/conf > /var/log/sing-box.log 2>&1 & echo "started sing-box" && sleep 3 && ps aux | grep "sing-box run" | grep -v grep | head -1`
	start := RunCommand(startCmd)

	return CommandResult{
		Ok:     start.Ok,
		Stdout: fix.Stdout + "\n" + kill.Stdout + "\n" + start.Stdout,
		Stderr: fix.Stderr + "\n" + start.Stderr,
	}
}

// TproxyOn 啟用 TProxy
func TproxyOn(ctx context.Context) CommandResult {
	script := `
if [ -x "/etc/tproxy-rules.sh" ]; then
    sudo bash "/etc/tproxy-rules.sh"
    echo "TProxy enabled via /etc/tproxy-rules.sh"
elif [ -f "/etc/tproxy-rules.sh" ]; then
    sudo bash "/etc/tproxy-rules.sh"
    echo "TProxy enabled via /etc/tproxy-rules.sh"
else
    sudo iptables -t mangle -A PREROUTING -p tcp -j TPROXY --tproxy-mark 0x1/0x1 --on-port 10808
    echo "TProxy enabled via fallback rules"
fi
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
			Error:  fmt.Sprintf("tproxy on failed: %s", err.Error()),
		}
	}
	return CommandResult{
		Ok:     true,
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
}

// TproxyOff 關閉 TProxy
func TproxyOff(ctx context.Context) CommandResult {
	script := `
sudo iptables -t mangle -F
sudo ip6tables -t mangle -F
echo "TProxy disabled - iptables mangle cleared"
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
			Error:  fmt.Sprintf("tproxy off failed: %s", err.Error()),
		}
	}
	return CommandResult{
		Ok:     true,
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
}

// TunOn 啟動 sing-box TUN 主進程（帶 TUN）
func TunOn() CommandResult {
	// 先修 DNS
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
func TunOff() CommandResult {
	// 直接調用 pkill -f 殺 config.json 主進程（不走 sh -c 避免引號問題）
	return runCommandWithSudo([]string{"pkill", "-f", "sing-box run -c /etc/sing-box/config.json"})
}

// runCommandWithSudo 強制用 sudo 執行單一指令
func runCommandWithSudo(args []string) CommandResult {
	if len(args) == 0 {
		return CommandResult{Ok: false, Error: "empty args"}
	}
	var stdout, stderr bytes.Buffer
	// 先試無密碼 sudo
	cmdArgs := append([]string{"-n"}, args...)
	cmd := exec.Command("sudo", cmdArgs...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		return CommandResult{Ok: true, Stdout: stdout.String() + "\nTUN disabled", Stderr: stderr.String()}
	}
	// 失敗 → 用密碼
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
		// pkill 無進程可殺會 exit 1，亦算成功
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return CommandResult{Ok: true, Stdout: "TUN disabled (no process found)", Stderr: stderr.String()}
		}
		return CommandResult{Ok: false, Stderr: stderr.String(), Error: "sudo pkill failed: " + err.Error()}
	}
	return CommandResult{Ok: true, Stdout: stdout.String() + "\nTUN disabled", Stderr: stderr.String()}
}

// ClearIptables 清除 iptables
func ClearIptables(ctx context.Context) CommandResult {
	script := `
sudo iptables -F
sudo iptables -X
sudo iptables -t nat -F
sudo iptables -t nat -X
sudo iptables -t mangle -F
sudo iptables -t mangle -X
sudo iptables -P INPUT ACCEPT
sudo iptables -P FORWARD ACCEPT
sudo iptables -P OUTPUT ACCEPT
echo "iptables cleared"
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
			Error:  fmt.Sprintf("iptables clear failed: %s", err.Error()),
		}
	}
	return CommandResult{
		Ok:     true,
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
}

