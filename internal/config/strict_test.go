package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A misspelled key stops the server at startup: read loosely, auth.oauth2.validate_remote misspelled takes its zero
// value, false, and the server then accepts any sid as an anonymous user while still logging "Authentication enabled".
func TestAMisspelledKeyStopsTheServer(t *testing.T) {
	good := `
server:
  port: 8080
erpnext:
  base_url: "http://frappe.test"
auth:
  enabled: true
  require_auth: true
  oauth2:
    issuer_url: "http://frappe.test"
    token_info_url: "http://frappe.test/api/method/frappe.integrations.oauth2.openid_profile"
    validate_remote: true
`
	load := func(yaml string) (*Config, error) {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("CONFIG_FILE", path)
		return Load()
	}

	cfg, err := load(good)
	if err != nil || !cfg.Auth.OAuth2.ValidateRemote {
		t.Fatalf("the correct config: %v, validate_remote=%v", err, cfg != nil && cfg.Auth.OAuth2.ValidateRemote)
	}
	if _, err := load(strings.Replace(good, "validate_remote", "validate_remot", 1)); err == nil || !strings.Contains(err.Error(), "validate_remot") {
		t.Errorf("a misspelled key: err = %v, want an error naming it", err)
	}
}
