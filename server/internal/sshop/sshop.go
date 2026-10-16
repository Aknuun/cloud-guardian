package sshop

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type Result struct {
	Ok  bool
	Out string
	Err string
}

const InfoCmd = "echo UNAME:$(uname -a); echo UPTIME:$(uptime -p); " +
	"echo LOAD:$(cat /proc/loadavg); " +
	"echo MEM:$(free -m | awk '/Mem:/{print $3\"/\"$2\"MB\"}'); " +
	"echo DISK:$(df -h / | awk 'NR==2{print $3\"/\"$2\" (\"$5\")\"}')"

const DiskCmd = "df -h | head -12"

const StatsCmd = "echo UPTIME_S:$(cat /proc/uptime | cut -d' ' -f1); " +
	"echo LOAD1:$(cat /proc/loadavg | cut -d' ' -f1); " +
	"echo CORES:$(nproc); " +
	"echo MEM_USED_MB:$(free -m | awk '/Mem:/{print $3}'); " +
	"echo MEM_TOTAL_MB:$(free -m | awk '/Mem:/{print $2}'); " +
	"echo DISK_USED:$(df / | awk 'NR==2{print $3}'); " +
	"echo DISK_TOTAL:$(df / | awk 'NR==2{print $2}'); " +
	"echo DISK_PCT:$(df / | awk 'NR==2{print $5}')"

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func authMethods(secret string) ([]ssh.AuthMethod, bool) {
	if strings.HasPrefix(secret, "-----BEGIN") {
		signer, err := ssh.ParsePrivateKey([]byte(secret))
		if err != nil {
			return nil, false
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, true
	}
	if secret == "" {
		return nil, false
	}
	return []ssh.AuthMethod{ssh.Password(secret), ssh.KeyboardInteractive(
		func(name, instruction string, questions []string, echos []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range answers {
				answers[i] = secret
			}
			return answers, nil
		})}, true
}

// RunCmd executes a command over SSH; never panics, returns an error string.
func RunCmd(host, user, secret string, port int, command string, timeout int) Result {
	res := Result{}
	if user == "" {
		user = "root"
	}
	if port == 0 {
		port = 22
	}
	if timeout == 0 {
		timeout = 20
	}
	auth, ok := authMethods(secret)
	if !ok {
		res.Err = "no credentials"
		return res
	}
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(timeout) * time.Second,
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	d := net.Dialer{Timeout: time.Duration(timeout) * time.Second}
	conn, err := d.Dial("tcp", addr)
	if err != nil {
		res.Err = trunc(err.Error(), 300)
		return res
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		conn.Close()
		res.Err = trunc(err.Error(), 300)
		return res
	}
	client := ssh.NewClient(c, chans, reqs)
	defer client.Close()
	sess, err := client.NewSession()
	if err != nil {
		res.Err = trunc(err.Error(), 300)
		return res
	}
	defer sess.Close()
	var stdout, stderr bytes.Buffer
	sess.Stdout = &stdout
	sess.Stderr = &stderr
	done := make(chan error, 1)
	go func() { done <- sess.Run(command) }()
	select {
	case err = <-done:
	case <-time.After(time.Duration(timeout) * time.Second):
		sess.Signal(ssh.SIGKILL)
		res.Err = "timeout"
		return res
	}
	res.Out = trunc(strings.TrimSpace(stdout.String()), 2000)
	res.Err = trunc(strings.TrimSpace(stderr.String()), 300)
	if err != nil {
		if res.Err == "" {
			res.Err = trunc(err.Error(), 300)
		}
		res.Ok = false
		return res
	}
	res.Ok = true
	return res
}

// Stats returns structured stats or {"error": msg}.
func Stats(host, user, secret string, port int) map[string]any {
	r := RunCmd(host, user, secret, port, StatsCmd, 20)
	if !r.Ok {
		m := r.Err
		if m == "" {
			m = "ssh failed"
		}
		return map[string]any{"error": m}
	}
	vals := map[string]string{}
	for _, ln := range strings.Split(r.Out, "\n") {
		if i := strings.Index(ln, ":"); i >= 0 {
			vals[strings.TrimSpace(ln[:i])] = strings.TrimSpace(ln[i+1:])
		}
	}
	cores := atoiMin(vals["CORES"], 1)
	load := atof(vals["LOAD1"])
	memUsed := atoi(vals["MEM_USED_MB"])
	memTotal := atoi(vals["MEM_TOTAL_MB"])
	diskUsed := atoi(vals["DISK_USED"])
	diskTotal := atoi(vals["DISK_TOTAL"])
	gb := func(kb float64) string {
		return fmt.Sprintf("%.1f", round1(kb/1024/1024))
	}
	memPct := 0
	if memTotal > 0 {
		memPct = int(round1(float64(memUsed) / float64(memTotal) * 100))
	}
	return map[string]any{
		"uptimeS": int(atof(vals["UPTIME_S"])),
		"cpu": map[string]any{
			"pct":   int(round1(load / float64(cores) * 100)),
			"cores": cores,
		},
		"mem": map[string]any{
			"usedMb": memUsed, "totalMb": memTotal, "pct": memPct,
		},
		"disk": map[string]any{
			"used":  gb(float64(diskUsed)) + " GB",
			"total": gb(float64(diskTotal)) + " GB",
			"pct":   orDash(vals["DISK_PCT"]),
		},
	}
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func round1(v float64) float64 { return float64(int64(v*10+0.5)) / 10 }

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func atoiMin(s string, mn int) int {
	n := atoi(s)
	if n < mn {
		return mn
	}
	return n
}

func atof(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}
