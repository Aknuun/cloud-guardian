// Package config loads the server bot configuration.
package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	BotToken      string `json:"bot_token"`
	AdminID       int64  `json:"admin_id"`
	CFToken       string `json:"cf_token"`
	HZToken       string `json:"hz_token"`
	CFAccountID   string `json:"cf_account_id"`
	FONamespaceID string `json:"fo_namespace_id"`
	CFAPIToken    string `json:"cf_api_token"`
	PublicURL     string `json:"public_url"`
	WorkerName    string `json:"worker_name"`
	WorkerURL     string `json:"worker_url"`
	ReinstallKey  string `json:"reinstall_key"`
	GitHubTarball string `json:"github_tarball"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Failover returns the subset of config used by failover sync.
func (c *Config) FailoverMap() map[string]string {
	return map[string]string{
		"cf_api_token":    c.CFAPIToken,
		"cf_account_id":   c.CFAccountID,
		"fo_namespace_id": c.FONamespaceID,
		"public_url":      c.PublicURL,
		"worker_name":     c.WorkerName,
		"worker_url":      c.WorkerURL,
		"reinstall_key":   c.ReinstallKey,
		"github_tarball":  c.GitHubTarball,
	}
}
