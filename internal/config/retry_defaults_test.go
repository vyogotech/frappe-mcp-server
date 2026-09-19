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

// An omitted server.timeout must not leave the server's reads and writes unbounded.
func TestServerTimeoutDefaultsWhenOmitted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "server:\n  port: 8080\nerpnext:\n  base_url: \"http://frappe.test\"\n  api_key: \"k\"\n  api_secret: \"s\"\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_FILE", path)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Timeout <= 0 {
		t.Fatalf("server timeout with none configured: %s", cfg.Server.Timeout)
	}
}

// The knowledge base switch can be turned off by environment for a site without rag, and a bad value stops startup.
func TestTheKnowledgeBaseSwitchFollowsTheEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "server:\n  port: 8080\nerpnext:\n  base_url: \"http://frappe.test\"\n  api_key: \"k\"\n  api_secret: \"s\"\ntools:\n  knowledge_base: true\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_FILE", path)
	t.Setenv("TOOLS_KNOWLEDGE_BASE", "false")
	if cfg, err := Load(); err != nil || cfg.Tools.KnowledgeBase {
		t.Fatalf("TOOLS_KNOWLEDGE_BASE=false over a file saying true: %v, %v", cfg, err)
	}
	t.Setenv("TOOLS_KNOWLEDGE_BASE", "maybe")
	if _, err := Load(); err == nil {
		t.Fatal("TOOLS_KNOWLEDGE_BASE=maybe was accepted")
	}
}

// installation.md's first option: no config file at all, only environment variables.
func TestLoadWithTheEnvironmentAlone(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("CONFIG_FILE", "")
	t.Setenv("FRAPPE_BASE_URL", "http://frappe.test")
	t.Setenv("FRAPPE_API_KEY", "k")
	t.Setenv("FRAPPE_API_SECRET", "s")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ERPNext.BaseURL != "http://frappe.test" || cfg.Server.Port != 8080 {
		t.Fatalf("base_url %q, port %d", cfg.ERPNext.BaseURL, cfg.Server.Port)
	}
}
