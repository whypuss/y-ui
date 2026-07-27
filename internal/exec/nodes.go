package exec

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
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
	cfgBytes, err := exec.Command("cat", "/etc/sing-box/config.json").Output()
	if err != nil {
		return "", CommandResult{Ok: false, Error: "cannot read config.json: " + err.Error()}
	}
	var c map[string]interface{}
	if err := json.Unmarshal(cfgBytes, &c); err != nil {
		return "", CommandResult{Ok: false, Error: "parse config.json: " + err.Error()}
	}
	var anytls map[string]interface{}
	for _, v := range c["inbounds"].([]interface{}) {
		if ii, ok := v.(map[string]interface{}); ok && ii["type"] == "anytls" {
			anytls = ii
			break
		}
	}
	if anytls == nil {
		return "", CommandResult{Ok: false, Error: "no AnyTLS inbound found"}
	}
	port, _ := anytls["listen_port"].(float64)
	if port == 0 {
		return "", CommandResult{Ok: false, Error: "no listen_port in AnyTLS inbound"}
	}
	// server_name
	var serverName string
	tls, _ := anytls["tls"].(map[string]interface{})
	if tls != nil {
		s, _ := tls["server_name"].(string)
		serverName = s
	}
	// UUID from users[0].password
	uuids := []string{}
	users, _ := anytls["users"].([]interface{})
	for _, u := range users {
		if ui, ok := u.(map[string]interface{}); ok {
			pw, _ := ui["password"].(string)
			uuids = append(uuids, pw)
		}
	}
	if len(uuids) == 0 {
		// 無 UUID 就自己生成一個
		b := make([]byte, 16)
		rand.Read(b)
		uuids = append(uuids, hex.EncodeToString(b[:]))
	}
	// 優先用第一個 UUID
	uuid := uuids[0]
	// 生成標準連結: anytls://UUID@server_name:port/?insecure=1#anytls-server_name
	values := url.Values{"insecure": {"1"}}
	urlStr := fmt.Sprintf("anytls://%s@%s:%.0f/?%s#anytls-%s",
		uuid, serverName, port, values.Encode(), serverName)
	return urlStr, CommandResult{Ok: true}
}
