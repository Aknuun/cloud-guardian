package router

import (
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/fosync"
)

func (r *Router) foSnap() map[string]any {
	health := r.foVal("health_url")
	if health == "" {
		if pub := strings.TrimRight(r.foVal("public_url"), "/"); pub != "" {
			health = pub + "/health"
		}
	}
	username, _ := r.Store.GetOK("bot_username")
	return map[string]any{
		"ts":            time.Now().Unix(),
		"bot_username":  username,
		"cf_account_id": r.foVal("cf_account_id"),
		"worker_name":   r.foVal("worker_name"),
		"health_url":    health,
		"cf_tokens":     r.cfAccs(),
		"hz_tokens":     r.hzAccs(),
		"ln_tokens":     r.lnAccs(),
		"arvan_tokens":  r.arvanAccList(),
	}
}

// SyncPush pushes the failover snapshot to the Worker KV (best-effort).
func (r *Router) SyncPush() {
	tok, acc, ns := r.foVal("cf_api_token"), r.foVal("cf_account_id"), r.foVal("fo_namespace_id")
	if tok == "" || acc == "" || ns == "" {
		return
	}
	snap := r.foSnap()
	fosync.PushAsync(func() { fosync.PushSnapshot(tok, acc, ns, snap) })
}

// SyncCredentials returns the credentials needed for a DB backup push.
func (r *Router) SyncCredentials() (token, account, ns string) {
	return r.foVal("cf_api_token"), r.foVal("cf_account_id"), r.foVal("fo_namespace_id")
}
