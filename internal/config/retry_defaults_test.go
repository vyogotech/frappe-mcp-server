package config

import (
	"os"
	"path/filepath"
	"testing"
)

// An omitted max_delay must not cap every backoff at zero.
func TestRetryDelaysDefaultWhenOmitted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "server:\n  port: 8080\nerpnext:\n  base_url: \"http://frappe.test\"\n  api_key: \"k\"\n  api_secret: \"s\"\n  retry:\n    max_attempts: 5\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_FILE", path)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if r := cfg.ERPNext.Retry; r.InitialDelay <= 0 || r.MaxDelay < r.InitialDelay {
		t.Fatalf("retry delays with none configured: initial %s, max %s", r.InitialDelay, r.MaxDelay)
	}
}
