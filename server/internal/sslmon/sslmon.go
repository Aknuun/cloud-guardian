// Package sslmon checks TLS certificate expiry by direct handshake.
package sslmon

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/store"
)

type Result struct {
	Host   string `json:"host"`
	OK     bool   `json:"ok"`
	Days   int    `json:"days,omitempty"`
	Expiry string `json:"expiry,omitempty"`
	Error  string `json:"error,omitempty"`
}

type Monitor struct {
	Host string `json:"host"`
}

func GetMonitors(st *store.Store) []Monitor {
	var lst []Monitor
	st.GetJSON("ssl_monitor", &lst)
	return lst
}

func CheckHost(host string, timeout time.Duration) Result {
	host = strings.ToLower(strings.TrimSpace(host))
	res := Result{Host: host}
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", host+":443",
		&tls.Config{ServerName: host})
	if err != nil {
		res.Error = trunc(err.Error(), 150)
		return res
	}
	defer conn.Close()
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		res.Error = "no certificate"
		return res
	}
	na := certs[0].NotAfter.UTC()
	res.OK = true
	res.Days = int(time.Until(na).Hours() / 24)
	res.Expiry = na.Format("2006-01-02")
	return res
}

func CheckAll(st *store.Store) []Result {
	mons := GetMonitors(st)
	out := make([]Result, 0, len(mons))
	for _, m := range mons {
		out = append(out, CheckHost(m.Host, 20*time.Second))
	}
	return out
}

func ResultLine(r Result, warnDays int) string {
	if !r.OK {
		return fmt.Sprintf("🔴 <code>%s</code> — خطا: %s", r.Host, Escape(trunc(r.Error, 80)))
	}
	em := "🟢"
	if r.Days <= warnDays {
		em = "🔴"
	}
	return fmt.Sprintf("%s <code>%s</code> — %d روز مانده", em, r.Host, r.Days)
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func Escape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}
