package router

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	cfgTestKey      = "cfgtest_hist"
	cfgEngineVer    = "v26.10.31"
	cfgTarget       = "https://cp.cloudflare.com/generate_204"
	cfgMaxLinks     = 20
	cfgMaxLinkBytes = 8192
	cfgSamples      = 5
	cfgHistCap      = 50
	cfgLinkTimeout  = 20 * time.Second
)

// only two engines may run at once, whatever the request mix.
var cfgSlots = make(chan struct{}, 2)

// ---------- engine ----------

func cfgEnginePath() string {
	if p := strings.TrimSpace(os.Getenv("CG_CFGTEST_ENGINE")); p != "" {
		return p
	}
	exe, err := os.Executable()
	if err != nil {
		return "/opt/cloud-guardian/bin/xray"
	}
	return filepath.Join(filepath.Dir(exe), "bin", "xray")
}

func cfgEngineInfo() (bool, string) {
	p := cfgEnginePath()
	if _, err := os.Stat(p); err != nil {
		return false, ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, p, "version").Output()
	if err != nil {
		return false, ""
	}
	fields := strings.Fields(string(out))
	ver := ""
	for i, f := range fields {
		if strings.EqualFold(f, "xray") && i+1 < len(fields) {
			ver = fields[i+1]
			break
		}
	}
	return true, ver
}

// cfgInstallEngine downloads the pinned xray build, checks its published
// SHA-256 and drops the binary next to the server.
func cfgInstallEngine(ctx context.Context) error {
	var asset string
	switch runtime.GOARCH {
	case "amd64":
		asset = "Xray-linux-64.zip"
	case "arm64":
		asset = "Xray-linux-arm64-v8a.zip"
	case "arm":
		asset = "Xray-linux-arm32-v7a.zip"
	case "386":
		asset = "Xray-linux-32.zip"
	default:
		return errors.New("معماری سرور برای هستهٔ تست پشتیبانی نمی‌شود")
	}
	base := "https://github.com/XTLS/Xray-core/releases/download/" + cfgEngineVer + "/" + asset
	cl := &http.Client{Timeout: 120 * time.Second}

	dgst, err := cfgFetch(ctx, cl, base+".dgst", 1<<16)
	if err != nil {
		return fmt.Errorf("گرفتن چک‌سام ممکن نشد: %w", err)
	}
	want := ""
	for _, line := range strings.Split(string(dgst), "\n") {
		if i := strings.Index(line, "SHA2-256="); i >= 0 {
			want = strings.ToLower(strings.TrimSpace(line[i+len("SHA2-256="):]))
		}
	}
	if len(want) != 64 {
		return errors.New("چک‌سام هستهٔ تست خوانا نیست")
	}
	blob, err := cfgFetch(ctx, cl, base, 64<<20)
	if err != nil {
		return fmt.Errorf("دانلود هستهٔ تست نشد: %w", err)
	}
	sum := sha256.Sum256(blob)
	if hex.EncodeToString(sum[:]) != want {
		return errors.New("چک‌سام هستهٔ تست نمی‌خواند؛ نصب متوقف شد")
	}
	zr, err := zip.NewReader(bytes.NewReader(blob), int64(len(blob)))
	if err != nil {
		return errors.New("آرشیو هستهٔ تست خراب است")
	}
	var data []byte
	for _, f := range zr.File {
		if filepath.Base(f.Name) != "xray" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		data, err = io.ReadAll(io.LimitReader(rc, 128<<20))
		rc.Close()
		if err != nil {
			return err
		}
	}
	if len(data) == 0 {
		return errors.New("فایل اجرایی در آرشیو نبود")
	}
	dst := cfgEnginePath()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("ساخت پوشهٔ هسته نشد: %w", err)
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return fmt.Errorf("نوشتن هسته نشد: %w", err)
	}
	return os.Rename(tmp, dst)
}

func cfgFetch(ctx context.Context, cl *http.Client, u string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("پاسخ %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, errors.New("پاسخ خالی بود")
	}
	return b, nil
}

// ---------- ssrf guard ----------

func cfgPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil && v4[0] == 169 && v4[1] == 254 {
		return true
	}
	return false
}

func cfgGuardHost(ctx context.Context, host string) error {
	if strings.EqualFold(host, "localhost") {
		return errors.New("این مقصد مجاز نیست (آدرس داخلی)")
	}
	if ip := net.ParseIP(host); ip != nil {
		if cfgPrivateIP(ip) && os.Getenv("CG_CFGTEST_ALLOW_PRIVATE") != "1" {
			return errors.New("این مقصد مجاز نیست (آدرس داخلی)")
		}
		return nil
	}
	if os.Getenv("CG_CFGTEST_ALLOW_PRIVATE") == "1" {
		return nil
	}
	lctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(lctx, host)
	if err != nil {
		return nil // dns_fail is reported by the pre-check, not the guard
	}
	for _, a := range ips {
		if cfgPrivateIP(a.IP) {
			return errors.New("این مقصد مجاز نیست (آدرس داخلی)")
		}
	}
	return nil
}

// ---------- config ----------

func cfgBuildConfig(c cfgLink, socksPort int) map[string]any {
	stream := map[string]any{"network": c.Net}
	switch c.Security {
	case "reality":
		stream["security"] = "reality"
		stream["realitySettings"] = map[string]any{
			"serverName":  c.SNI,
			"fingerprint": c.FP,
			"publicKey":   c.PBK,
			"shortId":     c.SID,
			"spiderX":     "/",
		}
	case "tls":
		stream["security"] = "tls"
		stream["tlsSettings"] = map[string]any{"serverName": c.SNI, "fingerprint": c.FP, "allowInsecure": false}
	default:
		stream["security"] = "none"
	}
	switch c.Net {
	case "ws":
		ws := map[string]any{"path": cfgFirst(c.Path, "/")}
		if c.HostHdr != "" {
			ws["headers"] = map[string]any{"Host": c.HostHdr}
		}
		stream["wsSettings"] = ws
	case "grpc", "gun":
		stream["network"] = "grpc"
		stream["grpcSettings"] = map[string]any{"serviceName": c.Service}
	case "http", "h2":
		hs := map[string]any{"path": cfgFirst(c.Path, "/")}
		if c.HostHdr != "" {
			hs["host"] = []string{c.HostHdr}
		}
		stream["httpSettings"] = hs
	case "tcp":
		if c.HostHdr != "" {
			stream["tcpSettings"] = map[string]any{"header": map[string]any{
				"type": "http", "request": map[string]any{"headers": map[string]any{"Host": []string{c.HostHdr}}},
			}}
		}
	}

	out := map[string]any{"tag": "proxy", "streamSettings": stream}
	switch c.Protocol {
	case "vless":
		user := map[string]any{"id": c.UUID, "encryption": "none"}
		if c.Flow != "" {
			user["flow"] = c.Flow
		}
		out["protocol"] = "vless"
		out["settings"] = map[string]any{"vnext": []any{
			map[string]any{"address": c.Host, "port": c.Port, "users": []any{user}},
		}}
	case "vmess":
		out["protocol"] = "vmess"
		out["settings"] = map[string]any{"vnext": []any{
			map[string]any{"address": c.Host, "port": c.Port, "users": []any{
				map[string]any{"id": c.UUID, "alterId": c.AlterID, "security": "auto"},
			}},
		}}
	case "trojan":
		out["protocol"] = "trojan"
		out["settings"] = map[string]any{"servers": []any{
			map[string]any{"address": c.Host, "port": c.Port, "password": c.Password},
		}}
	}
	return map[string]any{
		"log":       map[string]any{"loglevel": "warning", "access": "none"},
		"inbounds":  []any{map[string]any{"tag": "socks-in", "listen": "127.0.0.1", "port": socksPort, "protocol": "socks", "settings": map[string]any{"auth": "noauth", "udp": false}}},
		"outbounds": []any{out},
	}
}

// ---------- measurement ----------

func cfgFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func cfgSocksDial(ctx context.Context, proxy, host string, port int) (net.Conn, error) {
	d := net.Dialer{}
	c, err := d.DialContext(ctx, "tcp", proxy)
	if err != nil {
		return nil, err
	}
	if dl, ok := ctx.Deadline(); ok {
		c.SetDeadline(dl)
	}
	if _, err := c.Write([]byte{5, 1, 0}); err != nil {
		c.Close()
		return nil, err
	}
	hello := make([]byte, 2)
	if _, err := io.ReadFull(c, hello); err != nil || hello[0] != 5 || hello[1] != 0 {
		c.Close()
		return nil, errors.New("socks handshake failed")
	}
	req := []byte{5, 1, 0, 3, byte(len(host))}
	req = append(req, host...)
	req = append(req, byte(port>>8), byte(port))
	if _, err := c.Write(req); err != nil {
		c.Close()
		return nil, err
	}
	hdr := make([]byte, 4)
	if _, err := io.ReadFull(c, hdr); err != nil {
		c.Close()
		return nil, err
	}
	if hdr[1] != 0 {
		c.Close()
		return nil, fmt.Errorf("socks reply %d", hdr[1])
	}
	n := 0
	switch hdr[3] {
	case 1:
		n = 4
	case 4:
		n = 16
	case 3:
		l := make([]byte, 1)
		if _, err := io.ReadFull(c, l); err != nil {
			c.Close()
			return nil, err
		}
		n = int(l[0])
	default:
		c.Close()
		return nil, errors.New("socks reply address")
	}
	if _, err := io.ReadFull(c, make([]byte, n+2)); err != nil {
		c.Close()
		return nil, err
	}
	c.SetDeadline(time.Time{})
	return c, nil
}

// cfgProbe returns ms from opening the tunnel to the first response byte.
func cfgProbe(ctx context.Context, socksAddr, target string) (float64, error) {
	u, err := url.Parse(target)
	if err != nil {
		return 0, err
	}
	port := 443
	if p := u.Port(); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}
	host := u.Hostname()
	tr := &http.Transport{
		DisableKeepAlives: true,
		DialContext: func(dctx context.Context, _, _ string) (net.Conn, error) {
			return cfgSocksDial(dctx, socksAddr, host, port)
		},
	}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 8 * time.Second}
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	ms := float64(time.Since(start).Microseconds()) / 1000
	if err != nil {
		return ms, err
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return ms, fmt.Errorf("status %d", resp.StatusCode)
	}
	return ms, nil
}

func cfgSummarize(samples []float64) (min, median, p95, jitter, loss float64) {
	ok := make([]float64, 0, len(samples))
	for _, v := range samples {
		if v > 0 {
			ok = append(ok, v)
		}
	}
	if len(samples) > 0 {
		loss = float64(len(samples)-len(ok)) / float64(len(samples))
	}
	if len(ok) == 0 {
		return
	}
	if len(ok) > 1 {
		var sum float64
		for i := 1; i < len(ok); i++ {
			d := ok[i] - ok[i-1]
			if d < 0 {
				d = -d
			}
			sum += d
		}
		jitter = sum / float64(len(ok)-1)
	}
	sort.Float64s(ok)
	min = ok[0]
	median = ok[len(ok)/2]
	if len(ok)%2 == 0 {
		median = (ok[len(ok)/2-1] + ok[len(ok)/2]) / 2
	}
	idx := (95*len(ok) + 99) / 100 // nearest rank
	if idx < 1 {
		idx = 1
	}
	if idx > len(ok) {
		idx = len(ok)
	}
	p95 = ok[idx-1]
	return
}

func cfgVerdict(okCount, total int) string {
	if total <= 0 {
		return "fail"
	}
	need := (total*4 + 4) / 5
	if okCount >= need {
		return "ok"
	}
	if okCount > 0 {
		return "degraded"
	}
	return "fail"
}

var cfgErrFA = map[string]string{
	"parse_error":       "لینک را نتوانستم بخوانم؛ کامل و بدون فاصله بچسبان.",
	"blocked_target":    "این مقصد مجاز نیست (آدرس داخلی).",
	"dns_fail":          "دامنه حل نشد؛ DNS یا خود دامنه مشکل دارد.",
	"tcp_refused":       "پورت پاسخ نداد؛ فایروال یا پنل خاموش است.",
	"tcp_timeout":       "پورت پاسخ نداد؛ فایروال یا پنل خاموش است.",
	"tls_fail":          "TLS رد شد؛ SNI یا گواهی/REALITY نمی‌خواند.",
	"engine_start_fail": "هستهٔ تست بالا نیامد؛ دوباره تلاش کن.",
	"handshake_fail":    "سرور کانفیگ را قبول نکرد؛ UUID/رمز یا پورت اشتباه است.",
	"no_response":       "هندشیک انجام شد ولی پاسخی برنگشت؛ احتمالاً UUID/پورت پنل عوض شده.",
	"engine_missing":    "هستهٔ تست نصب نیست.",
}

// ---------- result ----------

type cfgResult struct {
	FP        string    `json:"fp"`
	Remark    string    `json:"remark"`
	Protocol  string    `json:"protocol"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Verdict   string    `json:"verdict"`
	Pre       cfgPre    `json:"pre"`
	Samples   []float64 `json:"samples"`
	MinMS     float64   `json:"min_ms"`
	MedianMS  float64   `json:"median_ms"`
	P95MS     float64   `json:"p95_ms"`
	JitterMS  float64   `json:"jitter_ms"`
	Loss      float64   `json:"loss"`
	ErrorCode string    `json:"error_code"`
	ErrorFA   string    `json:"error_fa"`
}

type cfgPre struct {
	DNSMS float64 `json:"dns_ms"`
	TCPMS float64 `json:"tcp_ms"`
	TLSMS float64 `json:"tls_ms"`
}

func cfgFail(c cfgLink, code string) cfgResult {
	return cfgResult{
		FP: cfgFingerprint(cfgLinkKey(c)), Remark: c.Remark, Protocol: c.Protocol,
		Host: c.Host, Port: c.Port, Verdict: "fail", Samples: []float64{},
		ErrorCode: code, ErrorFA: cfgErrFA[code],
	}
}

func cfgLinkKey(c cfgLink) string {
	return c.Protocol + "|" + c.Host + ":" + strconv.Itoa(c.Port) + "|" + c.UUID + c.Password + "|" + c.Net + c.Path + c.SNI
}

func (r *Router) cfgTestOne(ctx context.Context, c cfgLink) cfgResult {
	res := cfgFail(c, "no_response")
	res.Verdict = "fail"
	if res.Remark == "" {
		res.Remark = c.Host + ":" + strconv.Itoa(c.Port)
	}
	// dns
	t0 := time.Now()
	lctx, lcancel := context.WithTimeout(ctx, 4*time.Second)
	_, derr := net.DefaultResolver.LookupHost(lctx, c.Host)
	lcancel()
	res.Pre.DNSMS = float64(time.Since(t0).Microseconds()) / 1000
	if derr != nil {
		res.ErrorCode = "dns_fail"
		res.ErrorFA = cfgErrFA["dns_fail"]
		return res
	}
	// tcp
	addr := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	t0 = time.Now()
	conn, terr := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	res.Pre.TCPMS = float64(time.Since(t0).Microseconds()) / 1000
	if terr != nil {
		res.ErrorCode = "tcp_timeout"
		if strings.Contains(strings.ToLower(terr.Error()), "refused") {
			res.ErrorCode = "tcp_refused"
		}
		res.ErrorFA = cfgErrFA[res.ErrorCode]
		return res
	}
	conn.Close()
	// tls is informational: reality rejects a plain handshake on purpose.
	if c.Security == "tls" {
		sni := cfgFirst(c.SNI, c.HostHdr, c.Host)
		t0 = time.Now()
		dctx, dcancel := context.WithTimeout(ctx, 6*time.Second)
		tc, terr := (&tls.Dialer{Config: &tls.Config{ServerName: sni}}).DialContext(dctx, "tcp", addr)
		dcancel()
		res.Pre.TLSMS = float64(time.Since(t0).Microseconds()) / 1000
		if terr == nil {
			tc.Close()
		}
	}

	bin := cfgEnginePath()
	socksPort, err := cfgFreePort()
	if err != nil {
		res.ErrorCode = "engine_start_fail"
		res.ErrorFA = cfgErrFA["engine_start_fail"]
		return res
	}
	dir, err := os.MkdirTemp("", "cfgtest-")
	if err != nil {
		res.ErrorCode = "engine_start_fail"
		res.ErrorFA = cfgErrFA["engine_start_fail"]
		return res
	}
	defer os.RemoveAll(dir)
	cfgPath := filepath.Join(dir, "config.json")
	body, _ := json.Marshal(cfgBuildConfig(c, socksPort))
	if err := os.WriteFile(cfgPath, body, 0o600); err != nil {
		res.ErrorCode = "engine_start_fail"
		res.ErrorFA = cfgErrFA["engine_start_fail"]
		return res
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var logs lockedBuf
	cmd := exec.CommandContext(runCtx, bin, "run", "-c", cfgPath)
	cmd.Stderr = &logs
	cmd.Stdout = &logs
	if err := cmd.Start(); err != nil {
		res.ErrorCode = "engine_start_fail"
		res.ErrorFA = cfgErrFA["engine_start_fail"]
		return res
	}
	done := make(chan struct{})
	go func() { cmd.Wait(); close(done) }()
	defer func() { cancel(); <-done }()

	proxy := "127.0.0.1:" + strconv.Itoa(socksPort)
	ready := false
	for i := 0; i < 60; i++ {
		select {
		case <-done:
			res.ErrorCode = "engine_start_fail"
			res.ErrorFA = cfgErrFA["engine_start_fail"]
			return res
		case <-time.After(50 * time.Millisecond):
		}
		cc, err := net.DialTimeout("tcp", proxy, 300*time.Millisecond)
		if err == nil {
			cc.Close()
			ready = true
			break
		}
	}
	if !ready {
		res.ErrorCode = "engine_start_fail"
		res.ErrorFA = cfgErrFA["engine_start_fail"]
		return res
	}

	samples := make([]float64, cfgSamples)
	errs := 0
	for i := 0; i < cfgSamples; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
			case <-time.After(300 * time.Millisecond):
			}
		}
		ms, err := cfgProbe(ctx, proxy, cfgTarget)
		if err != nil {
			errs++
			samples[i] = 0
			continue
		}
		samples[i] = ms
	}
	res.Samples = samples
	res.MinMS, res.MedianMS, res.P95MS, res.JitterMS, res.Loss = cfgSummarize(samples)
	res.Verdict = cfgVerdict(cfgSamples-errs, cfgSamples)
	if res.Verdict != "fail" {
		res.ErrorCode = ""
		res.ErrorFA = ""
		return res
	}
	res.ErrorCode = "no_response"
	low := strings.ToLower(logs.String())
	if strings.Contains(low, "invalid") || strings.Contains(low, "rejected") || strings.Contains(low, "handshake") || strings.Contains(low, "unauthorized") {
		res.ErrorCode = "handshake_fail"
	}
	if res.Pre.TLSMS > 0 && strings.Contains(low, "tls") {
		res.ErrorCode = "tls_fail"
	}
	res.ErrorFA = cfgErrFA[res.ErrorCode]
	return res
}

// ---------- history ----------

type cfgHistEntry struct {
	FP       string  `json:"fp"`
	Remark   string  `json:"remark"`
	Protocol string  `json:"protocol"`
	Host     string  `json:"host"`
	Port     int     `json:"port"`
	Verdict  string  `json:"verdict"`
	MedianMS float64 `json:"median_ms"`
	Loss     float64 `json:"loss"`
	At       int64   `json:"at"`
}

func (r *Router) cfgHistory() []cfgHistEntry {
	var lst []cfgHistEntry
	if !r.Store.GetJSON(cfgTestKey, &lst) {
		return []cfgHistEntry{}
	}
	if lst == nil {
		return []cfgHistEntry{}
	}
	return lst
}

func (r *Router) cfgHistAdd(res cfgResult) {
	lst := r.cfgHistory()
	entry := cfgHistEntry{
		FP: res.FP, Remark: res.Remark, Protocol: res.Protocol, Host: res.Host, Port: res.Port,
		Verdict: res.Verdict, MedianMS: res.MedianMS, Loss: res.Loss, At: time.Now().Unix(),
	}
	lst = append([]cfgHistEntry{entry}, lst...)
	if len(lst) > cfgHistCap {
		lst = lst[:cfgHistCap]
	}
	r.Store.PutJSON(cfgTestKey, lst, 0)
}

// ---------- handlers ----------

func (r *Router) webToolsCfgTest(w http.ResponseWriter, req *http.Request, uid int64) {
	ok, ver := cfgEngineInfo()
	webOK(w, map[string]any{
		"engine":  map[string]any{"ok": ok, "version": ver, "path": cfgEnginePath()},
		"limits":  map[string]any{"links": cfgMaxLinks, "samples": cfgSamples, "timeout_s": int(cfgLinkTimeout.Seconds())},
		"targets": []string{cfgTarget},
		"history": r.cfgHistory(),
	})
}

// POST /api/tools/cfgtest/hist clears the saved results.
func (r *Router) webCfgTestHistClear(w http.ResponseWriter, req *http.Request, uid int64) {
	r.Store.PutJSON(cfgTestKey, []cfgHistEntry{}, 0)
	webOK(w, map[string]any{"cleared": true})
}

func (r *Router) webCfgTestEngine(w http.ResponseWriter, req *http.Request, uid int64) {
	ctx, cancel := context.WithTimeout(req.Context(), 150*time.Second)
	defer cancel()
	if err := cfgInstallEngine(ctx); err != nil {
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	ok, ver := cfgEngineInfo()
	webOK(w, map[string]any{"ok": ok, "version": ver, "path": cfgEnginePath()})
}

func (r *Router) webCfgTestRun(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Links   []string `json:"links"`
		Samples int      `json:"samples"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20)).Decode(&in); err != nil {
		webErr(w, http.StatusBadRequest, "درخواست خوانا نیست")
		return
	}
	if len(in.Links) == 0 {
		webErr(w, http.StatusBadRequest, "حداقل یک لینک لازم است")
		return
	}
	if len(in.Links) > cfgMaxLinks {
		webErr(w, http.StatusBadRequest, "حداکثر ۲۰ لینک در هر تست")
		return
	}
	if ok, _ := cfgEngineInfo(); !ok {
		webErr(w, http.StatusConflict, cfgErrFA["engine_missing"])
		return
	}
	select {
	case cfgSlots <- struct{}{}:
		defer func() { <-cfgSlots }()
	case <-time.After(3 * time.Second):
		webErr(w, http.StatusTooManyRequests, "صف تست پر است؛ چند لحظه بعد تلاش کن")
		return
	case <-req.Context().Done():
		return
	}

	results := make([]cfgResult, 0, len(in.Links))
	for _, raw := range in.Links {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if len(raw) > cfgMaxLinkBytes {
			results = append(results, cfgResult{Verdict: "fail", ErrorCode: "parse_error", ErrorFA: cfgErrFA["parse_error"], FP: cfgFingerprint(raw), Samples: []float64{}})
			continue
		}
		c, err := cfgParseLink(raw)
		if err != nil {
			results = append(results, cfgResult{Verdict: "fail", ErrorCode: "parse_error", ErrorFA: cfgErrFA["parse_error"], FP: cfgFingerprint(raw), Samples: []float64{}})
			continue
		}
		ctx, cancel := context.WithTimeout(req.Context(), cfgLinkTimeout)
		if err := cfgGuardHost(ctx, c.Host); err != nil {
			cancel()
			res := cfgFail(c, "blocked_target")
			res.FP = cfgFingerprint(raw)
			results = append(results, res)
			continue
		}
		res := r.cfgTestOne(ctx, c)
		cancel()
		res.FP = cfgFingerprint(raw)
		r.cfgHistAdd(res)
		results = append(results, res)
	}
	webOK(w, map[string]any{"results": results})
}

// lockedBuf keeps the tail of xray's stderr without unbounded growth.
type lockedBuf struct {
	mu  sync.Mutex
	buf []byte
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf = append(l.buf, p...)
	if len(l.buf) > 8192 {
		l.buf = l.buf[len(l.buf)-8192:]
	}
	return len(p), nil
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return string(l.buf)
}
