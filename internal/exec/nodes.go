package exec

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)
// InboundListener sing-box 監聽端口信息
type InboundListener struct {
	Port   int    `json:"port"`
	Type   string `json:"type"`
	Tag    string `json:"tag"`
	TLS    bool   `json:"tls"`
}

// NodeConfig 節點配置（不含敏感憑證）
type NodeConfig struct {
	Type        string `json:"type"`
	Server      string `json:"server"`
	Port        int    `json:"port"`
	ServerName  string `json:"server_name"`
	Tag         string `json:"tag"`
	Protocol    string `json:"protocol"`
}

// GetInboundListeners 讀取 sing-box 所有 inbound listeners
// 讀 config.json inbounds，從 ss -tlnp 獲取真實監聽端口
func GetInboundListeners() ([]InboundListener, CommandResult) {
	// 讀 config.json 獲取 inbound 列表
	cfgPath := "/etc/sing-box/config.json"
	cfgBytes, err := exec.Command("cat", cfgPath).Output()
	if err != nil {
		return nil, CommandResult{Ok: false, Error: "cannot read config.json: " + err.Error()}
	}

	var cfg struct {
		Inbounds []struct {
			Type string `json:"type"`
			Tag  string `json:"tag"`
			Port int    `json:"port"`
			TLS  struct {
				Enabled bool `json:"enabled"`
			} `json:"tls"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(cfgBytes, &cfg); err != nil {
		return nil, CommandResult{Ok: false, Error: "parse config.json: " + err.Error()}
	}

	// 讀 ss -tlnp 獲取真實監聽端口
	ssOut, err := exec.Command("sudo", "-n", "ss", "-tlnp").Output()
	if err != nil {
		ssOut, _ = exec.Command("sudo", "-S", "ss", "-tlnp").Output()
	}
	ssLines := strings.Split(string(ssOut), "\n")

	listeners := []InboundListener{}
	for _, ib := range cfg.Inbounds {
		// 嘗試從 ss 輸出匹配端口
		port := ib.Port
		if port == 0 {
			// 從 tag 提取端口（例如 ANYTLS-17777 → 17777）
			port = extractPortFromTag(ib.Tag)
		}
		// 查找對應端口是否在 ss 輸出中
		for _, line := range ssLines {
			if strings.Contains(line, fmt.Sprintf(":%d ", port)) || strings.Contains(line, fmt.Sprintf(":%d\t", port)) {
				listeners = append(listeners, InboundListener{
					Port: port,
					Type: ib.Type,
					Tag:  ib.Tag,
					TLS:  ib.TLS.Enabled,
				})
				break
			}
		}
	}

	return listeners, CommandResult{Ok: true}
}

// extractPortFromTag 從 tag 名提取端口號
func extractPortFromTag(tag string) int {
	// ANYTLS-17777 → 17777, VLESS-WS-8894 → 8894
	parts := strings.Split(tag, "-")
	if len(parts) >= 2 {
		var port int
		fmt.Sscanf(parts[len(parts)-1], "%d", &port)
		if port > 0 {
			return port
		}
	}
	return 0
}

// GetAnyTLSNode 讀取 AnyTLS outbound 配置（不含敏感憑證如 password/uuid）
func GetAnyTLSNode() NodeConfig {
	node := NodeConfig{Type: "anytls", Protocol: "AnyTLS"}

	// 讀 us-proxy.json
	cmd := exec.Command("sh", "-c", `cat /etc/sing-box/us-proxy.json 2>/dev/null | python3 -c "
import json,sys
c=json.load(sys.stdin)
for o in c.get('outbounds',[]):
    if o.get('type')=='anytls':
        print(o.get('server',''),o.get('server_port',0),o.get('tag',''),o.get('tls',{}).get('server_name',''))
        break
"`)
	out, _ := cmd.Output()
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) >= 3 {
		node.Server = parts[0]
		fmt.Sscanf(parts[1], "%d", &node.Port)
		node.Tag = parts[2]
		if len(parts) >= 4 {
			node.ServerName = parts[3]
		}
		return node
	}

	return node
}

// GetAllNodes 獲取所有外網 outbound 節點（不含敏感信息）
func GetAllNodes() []NodeConfig {
	nodes := []NodeConfig{}

	// 讀 us-proxy.json
	cmd := exec.Command("sh", "-c", `python3 -c "
import json
c=json.load(open('/etc/sing-box/us-proxy.json'))
for o in c.get('outbounds',[]):
    t=o.get('type','?')
    tag=o.get('tag','?')
    server=o.get('server','?')
    port=o.get('server_port',0)
    sn=o.get('tls',{}).get('server_name','')
    flow=o.get('flow','')
    print(f'{t}|{tag}|{server}|{port}|{sn}|{flow}')
" 2>/dev/null`)
	out, _ := cmd.Output()
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Split(strings.TrimSpace(line), "|")
		if len(fields) >= 6 {
			var port int
			fmt.Sscanf(fields[3], "%d", &port)
			nodes = append(nodes, NodeConfig{
				Type:       fields[0],
				Tag:        fields[1],
				Server:     fields[2],
				Port:       port,
				ServerName: fields[4],
				Protocol:   nodeProtocol(fields[0], fields[5]),
			})
		}
	}

	// 讀 jp-proxy.json
	cmd = exec.Command("sh", "-c", `python3 -c "
import json
c=json.load(open('/etc/sing-box/jp-proxy.json'))
for o in c.get('outbounds',[]):
    t=o.get('type','?')
    tag=o.get('tag','?')
    server=o.get('server','?')
    port=o.get('server_port',0)
    sn=o.get('tls',{}).get('server_name','')
    flow=o.get('flow','')
    print(f'{t}|{tag}|{server}|{port}|{sn}|{flow}')
" 2>/dev/null`)
	out, _ = cmd.Output()
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Split(strings.TrimSpace(line), "|")
		if len(fields) >= 6 {
			var port int
			fmt.Sscanf(fields[3], "%d", &port)
			nodes = append(nodes, NodeConfig{
				Type:       fields[0],
				Tag:        fields[1],
				Server:     fields[2],
				Port:       port,
				ServerName: fields[4],
				Protocol:   nodeProtocol(fields[0], fields[5]),
			})
		}
	}

	return nodes
}

// nodeProtocol 根據 type + flow 生成顯示協議名
func nodeProtocol(typ, flow string) string {
	if typ == "anytls" {
		return "AnyTLS"
	}
	if typ == "vless" {
		if flow == "xtls-rprx-vision" {
			return "VLESS-TLS"
		}
		return "VLESS"
	}
	return strings.Title(typ)
}

// GenAnyTLSURL 讀取 AnyTLS inbound 配置，生成 anytls:// 標準節點連結
func GenAnyTLSURL() (string, CommandResult) {
	return GenAnyTLSURLWithParams("", 0)
}

// GenAnyTLSURLWithParams 帶 host/端口參數嘅版本
func GenAnyTLSURLWithParams(host string, port int) (string, CommandResult) {
	cfgBytes, err := exec.Command("cat", "/etc/sing-box/config.json").Output()
	if err != nil {
		return "", CommandResult{Ok: false, Error: "cannot read config.json: " + err.Error()}
	}
	var c map[string]interface{}
	if err := json.Unmarshal(cfgBytes, &c); err != nil {
		return "", CommandResult{Ok: false, Error: "parse config.json: " + err.Error()}
	}
	// 找 AnyTLS inbound，取 UUID
	var anytls map[string]interface{}
	for _, v := range c["inbounds"].([]interface{}) {
		if ii, ok := v.(map[string]interface{}); ok && ii["type"] == "anytls" {
			anytls = ii
			break
		}
	}
	var uuid string
	if anytls != nil {
		users, _ := anytls["users"].([]interface{})
		for _, u := range users {
			if ui, ok := u.(map[string]interface{}); ok {
				pw, _ := ui["password"].(string)
				uuid = pw
				break
			}
		}
	}
	if uuid == "" {
		return "", CommandResult{Ok: false, Error: "no UUID found in AnyTLS inbound"}
	}
	// 取得本機公網 IP
	publicIP, err := getPublicIP()
	if err != nil {
		return "", CommandResult{Ok: false, Error: "cannot get public IP: " + err.Error()}
	}
	// 端口默認從 config 讀
	port2 := 17777
	if anytls != nil {
		p, _ := anytls["listen_port"].(float64)
		if p > 0 {
			port2 = int(p)
		}
	}
	if port > 0 {
		port2 = port
	}
	host2 := publicIP
	if host != "" {
		host2 = host
	}
	values := url.Values{"insecure": {"1"}}
	urlStr := fmt.Sprintf("anytls://%s@%s:%d/?%s#anytls-%s", uuid, host2, port2, values.Encode(), host2)
	return urlStr, CommandResult{Ok: true}
}

// GetPublicIP 讀取本機公網 IPv4
func GetPublicIP() (string, CommandResult) {
	ip, err := getPublicIP()
	if err != nil {
		return "", CommandResult{Ok: false, Error: err.Error()}
	}
	return ip, CommandResult{Ok: true}
}

func getPublicIP() (string, error) {
	// 嘗試多個源
	sources := []string{
		"https://ipinfo.io/ip",
		"https://api.ipify.org",
		"https://icanhazip.com",
	}
	for _, src := range sources {
		out, err := exec.Command("sh", "-c", fmt.Sprintf("curl -s -m 5 %s", src)).Output()
		if err != nil {
			continue
		}
		ip := strings.TrimSpace(string(out))
		if len(ip) > 0 && !strings.HasPrefix(ip, ":") {
			return ip, nil
		}
	}
	return "", fmt.Errorf("all IP sources failed")
}

// newUUID 生成標準 UUIDv4
func newUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]))
}

// UpdateAnyTLSUUID 生成新 UUID → 寫入 config.json → 重啟 sing-box
func UpdateAnyTLSUUID() (string, CommandResult) {
	cfgBytes, err := exec.Command("cat", "/etc/sing-box/config.json").Output()
	if err != nil {
		return "", CommandResult{Ok: false, Error: "read config.json: " + err.Error()}
	}
	var c map[string]interface{}
	if err := json.Unmarshal(cfgBytes, &c); err != nil {
		return "", CommandResult{Ok: false, Error: "parse config.json: " + err.Error()}
	}
	new := newUUID()
	// 寫入 AnyTLS inbound
	found := false
	for i, v := range c["inbounds"].([]interface{}) {
		if ii, ok := v.(map[string]interface{}); ok && ii["type"] == "anytls" {
			users, _ := ii["users"].([]interface{})
			for _, u := range users {
				if ui, ok := u.(map[string]interface{}); ok {
					ui["password"] = new
					found = true
				}
			}
			c["inbounds"].([]interface{})[i] = ii
			break
		}
	}
	if !found {
		return "", CommandResult{Ok: false, Error: "no AnyTLS inbound found"}
	}
	// 寫回 config.json
	out, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", CommandResult{Ok: false, Error: "marshal config: " + err.Error()}
	}
	// 直接寫入 config.json（面板以 root 運行）
	err = os.WriteFile("/etc/sing-box/config.json", out, 0644)
	if err != nil {
		return "", CommandResult{Ok: false, Error: "write config.json: " + err.Error()}
	}
	// 重啟 sing-box 使生效
	r := RestartSingbox(nil)
	if !r.Ok {
		return "", CommandResult{Ok: false, Error: "restart sing-box failed: " + r.Error}
	}
	return new, CommandResult{
		Ok:     true,
		Stdout: "UUID updated: " + new + "\n" + r.Stdout,
		Stderr: r.Stderr,
	}
}
