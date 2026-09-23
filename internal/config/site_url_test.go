package config

import (
	"os"
	"path/filepath"
	"testing"
)

// One site named once has to start an authenticating server: a sid-only deployment sets no OAuth2 endpoint, and one
// that introspects Bearer tokens elsewhere keeps the endpoint it set.
func TestASiteNamedOnceIsEnough(t *testing.T) {
	load := func(yaml string) (*Config, error) {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("CONFIG_FILE", path)
		t.Setenv("OAUTH_TOKEN_INFO_URL", "")
		t.Setenv("OAUTH_ISSUER_URL", "")
		t.Setenv("FRAPPE_BASE_URL", "")
		return Load()
	}

	sidOnly := `
server:
  port: 8080
erpnext:
  base_url: "http://frappe.test"
auth:
  enabled: true
  require_auth: true
  oauth2:
    validate_remote: true
`
	cfg, err := load(sidOnly)
	if err != nil {
		t.Fatalf("a site named once: %v", err)
	}
	if want := "http://frappe.test" + frappeUserinfoPath; cfg.Auth.OAuth2.TokenInfoURL != want {
		t.Errorf("token_info_url = %q, want %q", cfg.Auth.OAuth2.TokenInfoURL, want)
	}

	elsewhere := sidOnly + "    token_info_url: \"http://idp.test/userinfo\"\n"
	cfg, err = load(elsewhere)
	if err != nil {
		t.Fatalf("an endpoint of its own: %v", err)
	}
	if cfg.Auth.OAuth2.TokenInfoURL != "http://idp.test/userinfo" {
		t.Errorf("token_info_url = %q, want the configured one", cfg.Auth.OAuth2.TokenInfoURL)
	}
}
