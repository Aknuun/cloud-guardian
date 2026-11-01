package router

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// cfgLink is a single proxy link reduced to what xray needs. The raw link is
// never stored: it carries the uuid/password.
type cfgLink struct {
	Protocol string
	Host     string
	Port     int
	Remark   string
	UUID     string
	Password string
	AlterID  int
	Net      string
	Security string
	SNI      string
	HostHdr  string
	Path     string
	Service  string
	Flow     string
	PBK      string
	SID      string
	FP       string
}

func cfgParseLink(raw string) (cfgLink, error) {
	s := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(strings.ToLower(s), "vless://"):
		return cfgParseVless(s)
	case strings.HasPrefix(strings.ToLower(s), "vmess://"):
		return cfgParseVmess(s)
	case strings.HasPrefix(strings.ToLower(s), "trojan://"):
		return cfgParseTrojan(s)
	}
	return cfgLink{}, errors.New("این نوع لینک پشتیبانی نمی‌شود")
}

func cfgPort(v string) (int, error) {
	p, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || p < 1 || p > 65535 {
		return 0, errors.New("پورت نامعتبر است")
	}
	return p, nil
}

func cfgFrag(u *url.URL) string {
	if u.Fragment == "" {
		return ""
	}
	if f, err := url.PathUnescape(u.Fragment); err == nil {
		return strings.TrimSpace(f)
	}
	return strings.TrimSpace(u.Fragment)
}

func cfgParseVless(s string) (cfgLink, error) {
	u, err := url.Parse(s)
	if err != nil || u.Hostname() == "" {
		return cfgLink{}, errors.New("لینک vless خوانا نیست")
	}
	port, err := cfgPort(u.Port())
	if err != nil {
		return cfgLink{}, err
	}
	id := u.User.Username()
	if id == "" {
		return cfgLink{}, errors.New("شناسهٔ کاربر در لینک نیست")
	}
	q := u.Query()
	c := cfgLink{
		Protocol: "vless", Host: u.Hostname(), Port: port, Remark: cfgFrag(u), UUID: id,
		Net:      cfgFirst(q.Get("type"), "tcp"),
		Security: cfgFirst(q.Get("security"), "none"),
		SNI:      cfgFirst(q.Get("sni"), q.Get("peer")),
		HostHdr:  q.Get("host"),
		Path:     q.Get("path"),
		Service:  cfgFirst(q.Get("serviceName"), q.Get("servicename")),
		Flow:     q.Get("flow"),
		PBK:      q.Get("pbk"),
		SID:      q.Get("sid"),
		FP:       cfgFirst(q.Get("fp"), "chrome"),
	}
	return c, nil
}

func cfgParseTrojan(s string) (cfgLink, error) {
	u, err := url.Parse(s)
	if err != nil || u.Hostname() == "" {
		return cfgLink{}, errors.New("لینک trojan خوانا نیست")
	}
	port, err := cfgPort(u.Port())
	if err != nil {
		return cfgLink{}, err
	}
	pw := u.User.Username()
	if pw == "" {
		return cfgLink{}, errors.New("رمز در لینک نیست")
	}
	q := u.Query()
	c := cfgLink{
		Protocol: "trojan", Host: u.Hostname(), Port: port, Remark: cfgFrag(u), Password: pw,
		Net:      cfgFirst(q.Get("type"), "tcp"),
		Security: cfgFirst(q.Get("security"), "tls"),
		SNI:      cfgFirst(q.Get("sni"), q.Get("peer")),
		HostHdr:  q.Get("host"),
		Path:     q.Get("path"),
		Service:  cfgFirst(q.Get("serviceName"), q.Get("servicename")),
		FP:       cfgFirst(q.Get("fp"), "chrome"),
	}
	return c, nil
}

type cfgVmessJSON struct {
	PS   string `json:"ps"`
	Add  string `json:"add"`
	Port any    `json:"port"`
	ID   string `json:"id"`
	AID  any    `json:"aid"`
	Scy  string `json:"scy"`
	Net  string `json:"net"`
	Type string `json:"type"`
	Host string `json:"host"`
	Path string `json:"path"`
	TLS  string `json:"tls"`
	SNI  string `json:"sni"`
	FP   string `json:"fp"`
}

func cfgParseVmess(s string) (cfgLink, error) {
	body := strings.TrimSpace(s[len("vmess://"):])
	body = strings.TrimRight(body, "=")
	var buf []byte
	var err error
	for _, enc := range []*base64.Encoding{base64.RawURLEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.StdEncoding} {
		if buf, err = enc.DecodeString(body); err == nil {
			break
		}
	}
	if err != nil {
		return cfgLink{}, errors.New("بدنهٔ base64 لینک vmess خوانا نیست")
	}
	var v cfgVmessJSON
	if err := json.Unmarshal(buf, &v); err != nil {
		return cfgLink{}, errors.New("json لینک vmess خوانا نیست")
	}
	port, err := cfgPort(fmt.Sprint(v.Port))
	if err != nil {
		return cfgLink{}, err
	}
	if v.Add == "" || v.ID == "" {
		return cfgLink{}, errors.New("میزبان یا شناسه در لینک vmess نیست")
	}
	aid, _ := strconv.Atoi(fmt.Sprint(v.AID))
	sec := "none"
	if strings.EqualFold(v.TLS, "tls") {
		sec = "tls"
	}
	return cfgLink{
		Protocol: "vmess", Host: v.Add, Port: port, Remark: strings.TrimSpace(v.PS),
		UUID: v.ID, AlterID: aid,
		Net:      cfgFirst(v.Net, "tcp"),
		Security: sec,
		SNI:      v.SNI,
		HostHdr:  v.Host,
		Path:     v.Path,
		FP:       cfgFirst(v.FP, "chrome"),
	}, nil
}

func cfgFirst(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// cfgFingerprint is the only thing stored about a link.
func cfgFingerprint(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])[:12]
}

// cfgRedact is safe to log: scheme, host and port only.
func cfgRedact(raw string) string {
	c, err := cfgParseLink(raw)
	if err != nil {
		return "invalid-link"
	}
	return c.Protocol + "://" + c.Host + ":" + strconv.Itoa(c.Port)
}
