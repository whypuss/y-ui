package exec

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/crypto/curve25519"
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

	// 讀 ss -tlnp + ss -ulnp 獲取真實監聽端口（TCP + UDP）
	ssOut, err := exec.Command("sudo", "-n", "ss", "-tlnp").Output()
	if err != nil {
		ssOut, _ = exec.Command("sudo", "-S", "ss", "-tlnp").Output()
	}
	ssOutUdp, _ := exec.Command("sudo", "-n", "ss", "-ulnp").Output()
	ssOut = append(ssOut, ssOutUdp...)
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

// getSingboxInbounds 安全讀取 sing-box 所有 inbound 配置（主 config + conf/*.json）
// GetSingboxInbounds 讀取 config.json + conf/*.json 的所有 inbounds
// 不會因類型斷言 panic —— 當 config.json 冇 inbounds 時返回空列表
func GetSingboxInbounds() ([]map[string]interface{}, CommandResult) {
	cfgBytes, err := exec.Command("cat", "/etc/sing-box/config.json").Output()
	if err != nil {
		return nil, CommandResult{Ok: false, Error: "cannot read config.json: " + err.Error()}
	}
	var c map[string]interface{}
	if err := json.Unmarshal(cfgBytes, &c); err != nil {
		return nil, CommandResult{Ok: false, Error: "parse config.json: " + err.Error()}
	}
	var inbounds []map[string]interface{}
	// 安全類型斷言：可能冇 inbounds 字段
	if raw, ok := c["inbounds"].([]interface{}); ok {
		for _, v := range raw {
			if ii, ok := v.(map[string]interface{}); ok {
				inbounds = append(inbounds, ii)
			}
		}
	}
	// 讀 conf/*.json 加載的配置文件
	confDir := "/etc/sing-box/conf"
	if entries, err := filepath.Glob(confDir + "/*.json"); err == nil {
		sort.Strings(entries)
		for _, fp := range entries {
			dat, err := os.ReadFile(fp)
			if err != nil {
				continue
			}
			var fcfg map[string]interface{}
			if err := json.Unmarshal(dat, &fcfg); err != nil {
				continue
			}
			if raw, ok := fcfg["inbounds"].([]interface{}); ok {
				for _, v := range raw {
					if ii, ok := v.(map[string]interface{}); ok {
						inbounds = append(inbounds, ii)
					}
				}
			}
		}
	}
	return inbounds, CommandResult{Ok: true}
}

// findInbound 從 inbound 列表找指定 type，返回第一個匹配 + 是否找到
func findInbound(inbounds []map[string]interface{}, typ string) (map[string]interface{}, bool) {
	for _, ib := range inbounds {
		if t, ok := ib["type"]; ok && t == typ {
			return ib, true
		}
	}
	return nil, false
}

// AnyRealityConfig AnyTLS+Reality 配置摘要
type AnyRealityConfig struct {
	PrivateKey  string `json:"private_key"`
	PublicKey   string `json:"public_key"`
	ServerName  string `json:"server_name"`
	ShortID     string `json:"short_id"`
	HandshakeS  string `json:"handshake_server"`
}

// GetAnyRealityConfig 讀 AnyTLS-Reality inbound 的 REALITY 參數
func GetAnyRealityConfig() (AnyRealityConfig, CommandResult) {
	inbounds, r := GetSingboxInbounds()
	if !r.Ok {
		return AnyRealityConfig{}, r
	}
	var cfg AnyRealityConfig
	for _, ib := range inbounds {
		if t, ok := ib["type"]; ok && t == "anytls" {
			tls, ok := ib["tls"].(map[string]interface{})
			if !ok {
				continue
			}
			sni, _ := tls["server_name"].(string)
			reality, ok := tls["reality"].(map[string]interface{})
			if !ok {
				continue
			}
			enabled, _ := reality["enabled"].(bool)
			if !enabled {
				continue
			}
			pk, _ := reality["private_key"].(string)
			cfg.ServerName = sni
			cfg.PrivateKey = pk
			// 計算 public_key（X25519）
			cfg.PublicKey = x25519PublicKey(pk)
			if hs, ok := reality["handshake"].(map[string]interface{}); ok {
				cfg.HandshakeS, _ = hs["server"].(string)
			}
			if sids, ok := reality["short_id"].([]interface{}); ok && len(sids) > 0 {
				cfg.ShortID, _ = sids[0].(string)
			}
			break
		}
	}
	if cfg.PrivateKey == "" {
		return AnyRealityConfig{}, CommandResult{Ok: false, Error: "no AnyTLS-Reality inbound found"}
	}
	return cfg, CommandResult{Ok: true}
}

// x25519PublicKey REALITY public_key = X25519 scalar mult(private_key, basepoint)
// 客戶端（sing-box / sbyt7 教程）必須填 public_key 連接
func x25519PublicKey(privateKey string) string {
	if privateKey == "" {
		return ""
	}
	var priv, pub, base [32]byte
	b, err := base64.RawURLEncoding.DecodeString(privateKey)
	if err != nil {
		b, err = base64.StdEncoding.DecodeString(privateKey)
		if err != nil {
			return ""
		}
	}
	if len(b) != 32 {
		return ""
	}
	copy(priv[:], b)
	copy(base[:], curve25519.Basepoint)
	curve25519.ScalarMult(&pub, &priv, &base)
	return base64.RawURLEncoding.EncodeToString(pub[:])
}

// readSingboxPassword 從 users[] 讀 password（兼容頂層 password + users 結構）
func ReadSingboxPassword(ib map[string]interface{}) string {
	if pw, ok := ib["password"].(string); ok && pw != "" {
		return pw
	}
	if users, ok := ib["users"].([]interface{}); ok && len(users) > 0 {
		if u, ok := users[0].(map[string]interface{}); ok {
			if pw, ok := u["password"].(string); ok {
				return pw
			}
		}
	}
	return ""
}

// updateInboundPassword 安全更新指定 type 的 password（兼容主 config + conf/）
func updateInboundPassword(typ string, newPw string) (string, CommandResult) {
	// 1. 嘗試主 config.json
	cfgBytes, err := exec.Command("cat", "/etc/sing-box/config.json").Output()
	if err != nil {
		return "", CommandResult{Ok: false, Error: "read: " + err.Error()}
	}
	var c map[string]interface{}
	if err := json.Unmarshal(cfgBytes, &c); err != nil {
		return "", CommandResult{Ok: false, Error: "parse: " + err.Error()}
	}
	if raw, ok := c["inbounds"].([]interface{}); ok {
		for i, v := range raw {
			if ii, ok := v.(map[string]interface{}); ok && ii["type"] == typ {
				users, _ := ii["users"].([]interface{})
				for _, u := range users {
					if ui, ok := u.(map[string]interface{}); ok {
						ui["password"] = newPw
					}
				}
				if typ == "shadowsocks" {
					ii["password"] = newPw
				}
				c["inbounds"].([]interface{})[i] = ii
				out, _ := json.MarshalIndent(c, "", "  ")
				err = os.WriteFile("/etc/sing-box/config.json", out, 0644)
				if err == nil {
					return "updated /etc/sing-box/config.json", CommandResult{Ok: true}
				}
				break
			}
		}
	}

	// 2. 嘗試 conf/
	confDir := "/etc/sing-box/conf"
	if entries, err := filepath.Glob(confDir + "/*.json"); err == nil {
		for _, fp := range entries {
			dat, err := os.ReadFile(fp)
			if err != nil {
				continue
			}
			var fcfg map[string]interface{}
			if err := json.Unmarshal(dat, &fcfg); err != nil {
				continue
			}
			if raw, ok := fcfg["inbounds"].([]interface{}); ok {
				for j, v := range raw {
					if ii, ok := v.(map[string]interface{}); ok && ii["type"] == typ {
						users, _ := ii["users"].([]interface{})
						for _, u := range users {
							if ui, ok := u.(map[string]interface{}); ok {
								ui["password"] = newPw
							}
						}
						if typ == "shadowsocks" {
							ii["password"] = newPw
						}
						fcfg["inbounds"].([]interface{})[j] = ii
						out, _ := json.MarshalIndent(fcfg, "", "  ")
						err = os.WriteFile(fp, out, 0644)
						if err == nil {
							return "updated " + fp, CommandResult{Ok: true}
						}
						break
					}
				}
			}
		}
	}

	return "", CommandResult{Ok: false, Error: "no inbound of type " + typ + " found in config or conf/"}
}
func GenAnyTLSURL() (string, CommandResult) {
	return GenAnyTLSURLWithParams("", 0)
}

// GenAnyTLSURLWithParams 帶 host/端口參數嘅版本
func GenAnyTLSURLWithParams(host string, port int) (string, CommandResult) {
	inbounds, r := GetSingboxInbounds()
	if !r.Ok {
		return "", r
	}
	anytls, found := findInbound(inbounds, "anytls")
	if !found {
		return "", CommandResult{Ok: false, Error: "no AnyTLS inbound found in config or conf/"}
	}
	uuid := ReadSingboxPassword(anytls)
	if uuid == "" {
		return "", CommandResult{Ok: false, Error: "no UUID found in AnyTLS inbound"}
	}
	publicIP, err := getPublicIP()
	if err != nil {
		return "", CommandResult{Ok: false, Error: "cannot get public IP: " + err.Error()}
	}
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

// genHY2URLWithParams 生成 hysteria2:// 標準節點連結（帶 host/端口參數）
func GenHY2URLWithParams(host string, port int) (string, CommandResult) {
	inbounds, r := GetSingboxInbounds()
	if !r.Ok {
		return "", r
	}
	hy2, _ := findInbound(inbounds, "hysteria2")
	password := ""
	if hy2 != nil {
		password = ReadSingboxPassword(hy2)
	}
	if password == "" {
		password = newUUID()
	}
	port2 := 22222
	if hy2 != nil {
		p, _ := hy2["listen_port"].(float64)
		if p > 0 {
			port2 = int(p)
		}
	}
	if port > 0 {
		port2 = port
	}
	publicIP, err := getPublicIP()
	if err != nil {
		return "", CommandResult{Ok: false, Error: "cannot get public IP: " + err.Error()}
	}
	host2 := publicIP
	if host != "" {
		host2 = host
	}
	values := url.Values{"insecure": {"1"}}
	urlStr := fmt.Sprintf("hysteria2://%s@%s:%d/?%s#hy2-%s", url.PathEscape(password), host2, port2, values.Encode(), host2)
	return urlStr, CommandResult{Ok: true}
}

// genSSURLWithParams 生成 ss:// 標準節點連結（帶 host/端口/方法參數）
func GenSSURLWithParams(host string, port int, method string) (string, CommandResult) {
	inbounds, r := GetSingboxInbounds()
	if !r.Ok {
		return "", r
	}
	ss, _ := findInbound(inbounds, "shadowsocks")
	password := ""
	if ss != nil {
		password = ReadSingboxPassword(ss)
	}
	if password == "" {
		password = randBase64(32)
	}
	method2 := "aes-256-gcm"
	if ss != nil {
		m, ok := ss["method"].(string)
		if ok && m != "" {
			method2 = m
		}
	}
	if method != "" {
		method2 = method
	}
	port2 := 33333
	if ss != nil {
		p, _ := ss["listen_port"].(float64)
		if p > 0 {
			port2 = int(p)
		}
	}
	if port > 0 {
		port2 = port
	}
	publicIP, err := getPublicIP()
	if err != nil {
		return "", CommandResult{Ok: false, Error: "cannot get public IP: " + err.Error()}
	}
	host2 := publicIP
	if host != "" {
		host2 = host
	}
	creds := fmt.Sprintf("%s:%s", method2, password)
	encoded := base64.URLEncoding.EncodeToString([]byte(creds))
	urlStr := fmt.Sprintf("ss://%s@%s:%d#ss-%s", encoded, host2, port2, host2)
	return urlStr, CommandResult{Ok: true}
}

// GenAnyRealityURLWithParams 生成 AnyTLS-Reality URL（含 REALITY server/short_id）
func GenAnyRealityURLWithParams(host string, port int, serverNameOverride string, shortIDOverride string) (string, CommandResult) {
	inbounds, r := GetSingboxInbounds()
	if !r.Ok {
		return "", r
	}
	// 尋找 type=anytls 且 TLS 啟用 reality 的 inbound
	var realityIB map[string]interface{}
	for _, ib := range inbounds {
		t, _ := ib["type"].(string)
		if t != "anytls" {
			continue
		}
		tls, ok := ib["tls"].(map[string]interface{})
		if !ok {
			continue
		}
		reality, rok := tls["reality"].(map[string]interface{})
		if !rok {
			continue
		}
		enabled, _ := reality["enabled"].(bool)
		if !enabled {
			continue
		}
		realityIB = ib
		break
	}
	if realityIB == nil {
		return "", CommandResult{Ok: false, Error: "no AnyTLS-Reality inbound found"}
	}
	password := ReadSingboxPassword(realityIB)
	if password == "" {
		password = newUUID()
	}
	port2 := 443
	if p, ok := realityIB["listen_port"].(float64); ok && p > 0 {
		port2 = int(p)
	}
	if port > 0 {
		port2 = port
	}
	publicIP, err := getPublicIP()
	if err != nil {
		return "", CommandResult{Ok: false, Error: "cannot get public IP: " + err.Error()}
	}
	host2 := publicIP
	if host != "" {
		host2 = host
	}
	// 讀 REALITY 參數（config 預設，用戶輸入可覆蓋）
	serverName, shortID := "", ""
	if tls, ok := realityIB["tls"].(map[string]interface{}); ok {
		serverName, _ = tls["server_name"].(string)
		if reality, ok := tls["reality"].(map[string]interface{}); ok {
			if hs, ok := reality["handshake"].(map[string]interface{}); ok {
				s, _ := hs["server"].(string)
				if s != "" {
					serverName = s
				}
			}
			if sids, ok := reality["short_id"].([]interface{}); ok && len(sids) > 0 {
				shortID, _ = sids[0].(string)
			}
		}
	}
	// 用戶輸入覆蓋
	if serverNameOverride != "" {
		serverName = serverNameOverride
	}
	if shortIDOverride != "" {
		shortID = shortIDOverride
	}
	if serverName == "" {
		return "", CommandResult{Ok: false, Error: "reality server_name not configured"}
	}
	// anytls://password@host:port/?server=SN&shortId=ID&insecure=1
	values := url.Values{"insecure": {"1"}, "server": {serverName}}
	if shortID != "" {
		values["shortId"] = []string{shortID}
	}
	urlStr := fmt.Sprintf("anytls://%s@%s:%d/?%s#anyreality-%s", password, host2, port2, values.Encode(), host2)
	return urlStr, CommandResult{Ok: true}
}

// randBase64 生成隨機 base64 字符串
func randBase64(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawStdEncoding.EncodeToString(b)
}

// updateHY2Password 生成新密碼 → 寫入配置 → 重啟
func UpdateHY2Password() (string, CommandResult) {
	newPw := newUUID()
	msg, r := updateInboundPassword("hysteria2", newPw)
	if !r.Ok {
		return "", r
	}
	r2 := RestartSingbox(nil)
	if !r2.Ok {
		return "", CommandResult{Ok: false, Error: "restart sing-box failed: " + r2.Error}
	}
	return newPw, CommandResult{
		Ok: true, Stdout: "HY2 password updated: " + newPw + "\n" + msg + "\n" + r2.Stdout, Stderr: r2.Stderr,
	}
}

// updateSSPassword 生成新密碼 → 寫入配置 → 重啟
func UpdateSSPassword() (string, CommandResult) {
	newPw := randBase64(32)
	msg, r := updateInboundPassword("shadowsocks", newPw)
	if !r.Ok {
		return "", r
	}
	r2 := RestartSingbox(nil)
	if !r2.Ok {
		return "", CommandResult{Ok: false, Error: "restart sing-box failed: " + r2.Error}
	}
	return newPw, CommandResult{
		Ok: true, Stdout: "SS password updated: " + newPw + "\n" + msg + "\n" + r2.Stdout, Stderr: r2.Stderr,
	}
}

// UpdateAnyRealityPassword 更新 AnyTLS-Reality 密碼
func UpdateAnyRealityPassword() (string, CommandResult) {
	newPw := newUUID()
	// 找 AnyTLS-Reality 配置文件（含 reality 的 anytls inbound）
	var targetFile string
	// 1. 主 config.json
	cfgBytes, err := exec.Command("cat", "/etc/sing-box/config.json").Output()
	if err == nil {
		var c map[string]interface{}
		if json.Unmarshal(cfgBytes, &c) == nil {
			if raw, ok := c["inbounds"].([]interface{}); ok {
			for _, v := range raw {
				if ii, ok := v.(map[string]interface{}); ok && ii["type"] == "anytls" {
					if tls, ok := ii["tls"].(map[string]interface{}); ok {
						if reality, ok := tls["reality"].(map[string]interface{}); ok {
							if enabled, ok := reality["enabled"].(bool); ok && enabled {
								targetFile = "/etc/sing-box/config.json"
								break
							}
						}
					}
				}
			}
			}
		}
	}
	// 2. conf/
	if targetFile == "" {
		entries, _ := filepath.Glob("/etc/sing-box/conf/*.json")
		for _, fp := range entries {
			dat, err := os.ReadFile(fp)
			if err != nil {
				continue
			}
			var fcfg map[string]interface{}
			if json.Unmarshal(dat, &fcfg) != nil {
				continue
			}
			if raw, ok := fcfg["inbounds"].([]interface{}); ok {
			for _, v := range raw {
				if ii, ok := v.(map[string]interface{}); ok && ii["type"] == "anytls" {
					if tls, ok := ii["tls"].(map[string]interface{}); ok {
						if reality, ok := tls["reality"].(map[string]interface{}); ok {
							if enabled, ok := reality["enabled"].(bool); ok && enabled {
								targetFile = fp
								break
							}
						}
					}
				}
			}
				if targetFile != "" {
					break
				}
			}
		}
	}
	if targetFile == "" {
		return "", CommandResult{Ok: false, Error: "no AnyTLS-Reality config found"}
	}
	// 讀取目標文件，更新 password
	dat, err := os.ReadFile(targetFile)
	if err != nil {
		return "", CommandResult{Ok: false, Error: "read " + targetFile + ": " + err.Error()}
	}
	var fcfg map[string]interface{}
	if err := json.Unmarshal(dat, &fcfg); err != nil {
		return "", CommandResult{Ok: false, Error: "parse " + targetFile + ": " + err.Error()}
	}
	if raw, ok := fcfg["inbounds"].([]interface{}); ok {
		for i, v := range raw {
			if ii, ok := v.(map[string]interface{}); ok && ii["type"] == "anytls" {
				users, _ := ii["users"].([]interface{})
				for _, u := range users {
					if ui, ok := u.(map[string]interface{}); ok {
						ui["password"] = newPw
					}
				}
				fcfg["inbounds"].([]interface{})[i] = ii
			}
		}
	}
	out, _ := json.MarshalIndent(fcfg, "", "  ")
	if err := os.WriteFile(targetFile, out, 0644); err != nil {
		return "", CommandResult{Ok: false, Error: "write " + targetFile + ": " + err.Error()}
	}
	msg := "updated " + targetFile
	r := RestartSingbox(nil)
	if !r.Ok {
		return "", CommandResult{Ok: false, Error: "restart sing-box failed: " + r.Error}
	}
	return newPw, CommandResult{
		Ok: true, Stdout: "AnyTLS-Reality password updated: " + newPw + "\n" + msg + "\n" + r.Stdout, Stderr: r.Stderr,
	}
}

// UpdateAnyTLSUUID 生成新 UUID → 寫入配置 → 重啟 sing-box
func UpdateAnyTLSUUID() (string, CommandResult) {
	newUUID := newUUID()
	msg, r := updateInboundPassword("anytls", newUUID)
	if !r.Ok {
		return "", r
	}
	r2 := RestartSingbox(nil)
	if !r2.Ok {
		return "", CommandResult{Ok: false, Error: "restart sing-box failed: " + r2.Error}
	}
	return newUUID, CommandResult{
		Ok: true, Stdout: "UUID updated: " + newUUID + "\n" + msg + "\n" + r2.Stdout, Stderr: r2.Stderr,
	}
}

