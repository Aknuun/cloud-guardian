// Package fosync pushes a small snapshot and an optional gzipped DB backup to
// the failover Worker's KV (Cloudflare API). Best-effort; never panics.
package fosync

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"
)

const kvMax = 24 * 1024 * 1024

func put(apiToken, accountID, namespaceID, key string, body []byte, timeout time.Duration) bool {
	if apiToken == "" || accountID == "" || namespaceID == "" {
		return false
	}
	url := "https://api.cloudflare.com/client/v4/accounts/" + accountID +
		"/storage/kv/namespaces/" + namespaceID + "/values/" + key
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	cli := &http.Client{Timeout: timeout}
	resp, err := cli.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// PushSnapshot stores the failover snapshot under the "fo_snap" key.
func PushSnapshot(apiToken, accountID, namespaceID string, snap map[string]any) bool {
	if apiToken == "" || accountID == "" || namespaceID == "" {
		return false
	}
	body, err := json.Marshal(snap)
	if err != nil || len(body) > kvMax {
		return false
	}
	return put(apiToken, accountID, namespaceID, "fo_snap", body, 25*time.Second)
}

// BackupDB stores a base64(gzip(db)) copy under the "fo_db" key.
func BackupDB(apiToken, accountID, namespaceID, dbPath string) bool {
	if apiToken == "" || accountID == "" || namespaceID == "" {
		return false
	}
	raw, err := os.ReadFile(dbPath)
	if err != nil {
		return false
	}
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, 6)
	if _, err := zw.Write(raw); err != nil {
		return false
	}
	if err := zw.Close(); err != nil {
		return false
	}
	packed := make([]byte, base64.StdEncoding.EncodedLen(buf.Len()))
	base64.StdEncoding.Encode(packed, buf.Bytes())
	if len(packed) > kvMax {
		return false
	}
	return put(apiToken, accountID, namespaceID, "fo_db", packed, 40*time.Second)
}

// PushAsync runs fn in a goroutine so a network stall never blocks the bot.
func PushAsync(fn func()) {
	go func() {
		defer func() { _ = recover() }()
		fn()
	}()
}
