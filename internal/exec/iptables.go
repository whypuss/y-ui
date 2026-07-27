package exec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
)

const iptablesConfigPath = "/opt/y-ui/iptables.json"

// IptablesConfig iptables 規則配置
type IptablesConfig struct {
	Interface    string `json:"interface"`     // 對外網卡
	TproxyPort   int    `json:"tproxy_port"`   // TProxy 目標端口
	RouterIP     string `json:"router_ip"`     // 主路由器 IP（DNS DNAT 目標）
	LANSubnet    string `json:"lan_subnet"`    // LAN 網段（留作擴展）
	DNSForward   bool   `json:"dns_forward"`   // DNS 轉發 53 → 路由器
	Masquerade   bool   `json:"masquerade"`    // NAT MASQUERADE
	Forward      bool   `json:"forward"`       // FORWARD ACCEPT
	Tproxy80     bool   `json:"tproxy_80"`     // TPROXY TCP 80
	Tproxy443    bool   `json:"tproxy_443"`    // TPROXY TCP 443
	ExcludeSelf  bool   `json:"exclude_self"`  // 排除本機流量，防止 TPROXY 循環
}

// ReadIptablesConfig 讀取 iptables 配置
func ReadIptablesConfig() IptablesConfig {
	cfg := defaultIptablesConfig()
	data, err := os.ReadFile(iptablesConfigPath)
	if err != nil {
		return cfg
	}
	json.Unmarshal(data, &cfg)
	if cfg.Interface == "" {
		cfg = defaultIptablesConfig()
	}
	return cfg
}

// WriteIptablesConfig 保存配置
func WriteIptablesConfig(cfg IptablesConfig) CommandResult {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return CommandResult{Ok: false, Error: "marshal failed: " + err.Error()}
	}
	err = os.WriteFile(iptablesConfigPath, data, 0644)
	if err != nil {
		return CommandResult{Ok: false, Error: "write failed: " + err.Error()}
	}
	return CommandResult{Ok: true, Stdout: "config saved to " + iptablesConfigPath}
}

func defaultIptablesConfig() IptablesConfig {
	return IptablesConfig{
		Interface:  "enp4s0f0",
		TproxyPort: 10808,
		RouterIP:   "192.168.31.1",
		LANSubnet:  "192.168.31.0/24",
	}
}

// ApplyIptables 根據配置勾選項生成 iptables 規則
func ApplyIptables(ctx context.Context, cfg IptablesConfig) CommandResult {
	_ = ctx
	var buf bytes.Buffer
	buf.WriteString("#!/bin/bash\nset -e\n")
	buf.WriteString(fmt.Sprintf("IFACE=\"%s\"\n", cfg.Interface))
	buf.WriteString(fmt.Sprintf("TPROXY_PORT=\"%d\"\n", cfg.TproxyPort))
	buf.WriteString(fmt.Sprintf("ROUTER_IP=\"%s\"\n", cfg.RouterIP))
	buf.WriteString("\n")

	// 自動取得本機 IP（來自指定網卡）
	buf.WriteString(`MY_IP=$(ip route get 1.2.3.4 dev "$IFACE" | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1)}' | head -1)
echo "MY_IP=$MY_IP"
`)

	// iproute table 100（僅當 TPROXY 開啟時）
	if cfg.Tproxy80 || cfg.Tproxy443 {
		buf.WriteString(`ip rule del fwmark 0x1/0x1 lookup 100 2>/dev/null || true
ip rule add fwmark 0x1/0x1 lookup 100
ip route add local 0.0.0.0/0 dev lo table 100 2>/dev/null || true
`)
	}

	// 排除本機流量（插入 PREROUTING 第一位，防止 TPROXY 循環）
	if cfg.ExcludeSelf {
		buf.WriteString(`# 本機流量不經 TPROXY（防循環）
iptables -t mangle -C PREROUTING 1 -s "$MY_IP" -j ACCEPT 2>/dev/null || iptables -t mangle -I PREROUTING 1 -s "$MY_IP" -j ACCEPT
`)
	}

	if cfg.Tproxy80 {
		buf.WriteString(fmt.Sprintf(
			"iptables -t mangle -C PREROUTING -p tcp --dport 80 -j TPROXY --on-port %d --tproxy-mark 0x1/0x1 2>/dev/null || iptables -t mangle -A PREROUTING -p tcp --dport 80 -j TPROXY --on-port %d --tproxy-mark 0x1/0x1\n",
			cfg.TproxyPort, cfg.TproxyPort))
	}
	if cfg.Tproxy443 {
		buf.WriteString(fmt.Sprintf(
			"iptables -t mangle -C PREROUTING -p tcp --dport 443 -j TPROXY --on-port %d --tproxy-mark 0x1/0x1 2>/dev/null || iptables -t mangle -A PREROUTING -p tcp --dport 443 -j TPROXY --on-port %d --tproxy-mark 0x1/0x1\n",
			cfg.TproxyPort, cfg.TproxyPort))
	}
	if cfg.DNSForward {
		buf.WriteString(fmt.Sprintf(
			"iptables -t nat -C PREROUTING -i \"$IFACE\" -p udp --dport 53 -j DNAT --to-destination %s:53 2>/dev/null || iptables -t nat -A PREROUTING -i \"$IFACE\" -p udp --dport 53 -j DNAT --to-destination %s:53\n"+
				"iptables -t nat -C PREROUTING -i \"$IFACE\" -p tcp --dport 53 -j DNAT --to-destination %s:53 2>/dev/null || iptables -t nat -A PREROUTING -i \"$IFACE\" -p tcp --dport 53 -j DNAT --to-destination %s:53\n",
			cfg.RouterIP, cfg.RouterIP, cfg.RouterIP, cfg.RouterIP))
	}
	if cfg.Masquerade {
		buf.WriteString(`iptables -t nat -C POSTROUTING -o "$IFACE" -j MASQUERADE 2>/dev/null || iptables -t nat -A POSTROUTING -o "$IFACE" -j MASQUERADE
`)
	}
	if cfg.Forward {
		buf.WriteString(`iptables -C FORWARD -i "$IFACE" -o "$IFACE" -j ACCEPT 2>/dev/null || iptables -A FORWARD -i "$IFACE" -o "$IFACE" -j ACCEPT
`)
	}
	buf.WriteString("echo \"[iptables] rules applied\"\n")

	return runCommandWithSudo([]string{"bash", "-c", buf.String()})
}

// IptablesRules 當前規則快照
func IptablesRules() CommandResult {
	script := `echo "=== filter ===" && iptables -L -n -v && echo "=== nat ===" && iptables -t nat -L -n -v && echo "=== mangle ===" && iptables -t mangle -L -n -v && echo "=== ip rule ===" && ip rule show`
	return runCommandWithSudo([]string{"sh", "-c", script})
}
