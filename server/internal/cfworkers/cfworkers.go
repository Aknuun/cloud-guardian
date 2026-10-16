// Package cfworkers is a minimal Cloudflare Workers API v4 client used by the
// failover feature: verify a token, list accounts, deploy/delete a Worker
// module, manage KV namespaces, cron schedules and the workers.dev subdomain.
package cfworkers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"
)

const api = "https://api.cloudflare.com/client/v4"

type Error struct{ Msg string }

func (e *Error) Error() string { return e.Msg }

type Client struct {
	token string
	hc    *http.Client
}

func New(token string) *Client {
	return &Client{token: strings.TrimSpace(token), hc: &http.Client{Timeout: 60 * time.Second}}
}

type envelope struct {
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
	Result json.RawMessage `json:"result"`
}

func (c *Client) do(method, path, ctype string, body io.Reader, out any) error {
	req, err := http.NewRequest(method, api+path, body)
	if err != nil {
		return &Error{err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return &Error{trunc(err.Error())}
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var env envelope
	if err := json.Unmarshal(b, &env); err != nil {
		return &Error{trunc(string(b))}
	}
	if !env.Success {
		if len(env.Errors) > 0 {
			return &Error{trunc(env.Errors[0].Message)}
		}
		return &Error{trunc(string(b))}
	}
	if out != nil && len(env.Result) > 0 {
		return json.Unmarshal(env.Result, out)
	}
	return nil
}

func (c *Client) doJSON(method, path string, in, out any) error {
	var rd io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		rd = bytes.NewReader(b)
	}
	return c.do(method, path, "application/json", rd, out)
}

// Verify checks that the token is valid and active.
func (c *Client) Verify() error {
	return c.do("GET", "/user/tokens/verify", "", nil, nil)
}

type Account struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (c *Client) Accounts() ([]Account, error) {
	var out []Account
	err := c.do("GET", "/accounts?per_page=50", "", nil, &out)
	return out, err
}

// WorkersSubdomain returns the account's *.workers.dev subdomain.
func (c *Client) WorkersSubdomain(aid string) (string, error) {
	var out struct {
		Subdomain string `json:"subdomain"`
	}
	err := c.do("GET", "/accounts/"+aid+"/workers/subdomain", "", nil, &out)
	return out.Subdomain, err
}

// ListScripts returns deployed worker script names.
func (c *Client) ListScripts(aid string) ([]string, error) {
	var res []struct {
		ID string `json:"id"`
	}
	if err := c.do("GET", "/accounts/"+aid+"/workers/scripts", "", nil, &res); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(res))
	for _, s := range res {
		out = append(out, s.ID)
	}
	return out, nil
}

// Binding is a Worker binding (KV / plain text / secret).
type Binding struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Text        string `json:"text,omitempty"`
	NamespaceID string `json:"namespace_id,omitempty"`
}

// Module is a module entry in the upload metadata.
type Module struct {
	Name string `json:"name"`
}

// Metadata is the multipart "metadata" part of a module upload.
type Metadata struct {
	MainModule        string    `json:"main_module"`
	CompatibilityDate string    `json:"compatibility_date"`
	UsageModel        string    `json:"usage_model,omitempty"`
	Modules           []Module  `json:"modules,omitempty"`
	Bindings          []Binding `json:"bindings,omitempty"`
}

// DeployModule uploads a single ES-module worker as main_module "worker.js".
func (c *Client) DeployModule(aid, name string, module []byte, md Metadata) error {
	if md.MainModule == "" {
		md.MainModule = "worker.js"
	}
	if md.CompatibilityDate == "" {
		md.CompatibilityDate = "2024-11-01"
	}
	if md.UsageModel == "" {
		md.UsageModel = "standard"
	}
	if len(md.Modules) == 0 {
		md.Modules = []Module{{Name: md.MainModule}}
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mdJSON, _ := json.Marshal(md)

	h1 := textproto.MIMEHeader{}
	h1.Set("Content-Disposition", `form-data; name="metadata"`)
	h1.Set("Content-Type", "application/json")
	p1, _ := mw.CreatePart(h1)
	_, _ = p1.Write(mdJSON)

	h2 := textproto.MIMEHeader{}
	h2.Set("Content-Disposition", `form-data; name="worker.js"; filename="worker.js"`)
	h2.Set("Content-Type", "application/javascript+module")
	p2, _ := mw.CreatePart(h2)
	_, _ = p2.Write(module)

	_ = mw.Close()
	return c.do("PUT", "/accounts/"+aid+"/workers/scripts/"+name, mw.FormDataContentType(), &buf, nil)
}

// DeleteScript removes a worker script.
func (c *Client) DeleteScript(aid, name string) error {
	return c.do("DELETE", "/accounts/"+aid+"/workers/scripts/"+name, "", nil, nil)
}

// SetSchedules sets the cron triggers for a worker.
func (c *Client) SetSchedules(aid, name string, crons []string) error {
	body := make([]map[string]string, 0, len(crons))
	for _, cr := range crons {
		body = append(body, map[string]string{"cron": cr})
	}
	return c.doJSON("PUT", "/accounts/"+aid+"/workers/scripts/"+name+"/schedules", body, nil)
}

// SetDev enables or disables the workers.dev route for a script.
func (c *Client) SetDev(aid, name string, on bool) error {
	return c.doJSON("POST", "/accounts/"+aid+"/workers/scripts/"+name+"/subdomain",
		map[string]bool{"enabled": on, "previews_enabled": true}, nil)
}

// ScriptURL builds the workers.dev endpoint for a script.
func (c *Client) ScriptURL(aid, name, path string) (string, error) {
	sub, err := c.WorkersSubdomain(aid)
	if err != nil || sub == "" {
		return "", &Error{"no workers.dev subdomain"}
	}
	return fmt.Sprintf("https://%s.%s.workers.dev%s", name, sub, path), nil
}

// CreateKV makes a KV namespace and returns its id.
func (c *Client) CreateKV(aid, title string) (string, error) {
	var out struct {
		ID string `json:"id"`
	}
	err := c.doJSON("POST", "/accounts/"+aid+"/storage/kv/namespaces",
		map[string]string{"title": title}, &out)
	return out.ID, err
}

// DeleteKV removes a KV namespace.
func (c *Client) DeleteKV(aid, nsid string) error {
	return c.do("DELETE", "/accounts/"+aid+"/storage/kv/namespaces/"+nsid, "", nil, nil)
}

func trunc(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 240 {
		return s[:240]
	}
	return s
}
