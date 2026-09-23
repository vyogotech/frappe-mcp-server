package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// frappeUserinfoPath is Frappe's own OpenID userinfo method, whitelisted as frappe.integrations.oauth2.openid_profile.
const frappeUserinfoPath = "/api/method/frappe.integrations.oauth2.openid_profile"

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	ERPNext ERPNextConfig `yaml:"erpnext"`
	Logging LoggingConfig `yaml:"logging"`
	Auth    AuthConfig    `yaml:"auth"`
	Tools   ToolsConfig   `yaml:"tools"`
}

// DefaultConfirmationRedeemMethod is the frappe_ai method a write tool's one-time token is spent against. There is no
// switch that turns the gate off: a site without the method refuses every write, which is the fail-closed side.
const DefaultConfirmationRedeemMethod = "frappe_ai.api.confirm.redeem"

// ToolsConfig switches on tools that need more than Frappe itself
type ToolsConfig struct {
	KnowledgeBase bool `yaml:"knowledge_base"` // search_knowledge_base answers only where the rag app is installed
	// ConfirmationRedeemMethod names the whitelisted Frappe method a write tool redeems its confirmation token against.
	ConfirmationRedeemMethod string `yaml:"confirmation_redeem_method"`
}

type ServerConfig struct {
	Host    string        `yaml:"host"`
	Port    int           `yaml:"port"`
	Timeout time.Duration `yaml:"timeout"`
}

type ERPNextConfig struct {
	BaseURL   string          `yaml:"base_url"`
	APIKey    string          `yaml:"api_key"`
	APISecret string          `yaml:"api_secret"`
	Timeout   time.Duration   `yaml:"timeout"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Retry     RetryConfig     `yaml:"retry"`
}

type RateLimitConfig struct {
	RequestsPerSecond int `yaml:"requests_per_second"`
	Burst             int `yaml:"burst"`
}

type RetryConfig struct {
	MaxAttempts  int           `yaml:"max_attempts"`
	InitialDelay time.Duration `yaml:"initial_delay"`
	MaxDelay     time.Duration `yaml:"max_delay"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
}

type AuthConfig struct {
	Enabled     bool             `yaml:"enabled"`
	RequireAuth bool             `yaml:"require_auth"`
	OAuth2      OAuth2Config     `yaml:"oauth2"`
	TokenCache  TokenCacheConfig `yaml:"token_cache"`
}

type OAuth2Config struct {
	// TokenInfoURL introspects Bearer tokens; it defaults to the userinfo endpoint of erpnext.base_url.
	TokenInfoURL string `yaml:"token_info_url"`
	// IssuerURL is still read so deployments that set it keep starting; sessions are validated against erpnext.base_url.
	IssuerURL string `yaml:"issuer_url"`

	// Trusted backend clients (can provide user context headers)
	TrustedClients []string `yaml:"trusted_clients"`

	// Token validation
	ValidateRemote bool `yaml:"validate_remote"`

	// HTTP client timeout
	Timeout time.Duration `yaml:"timeout"`
}

type TokenCacheConfig struct {
	TTL time.Duration `yaml:"ttl"`
}

// Load reads config.yaml, or the file CONFIG_FILE names, and lets environment variables override it.
func Load() (*Config, error) {
	// Load .env file if it exists (ignore errors as .env file might not exist)
	_ = godotenv.Load()

	// Load from YAML file
	configFile := "config.yaml"
	if envFile := os.Getenv("CONFIG_FILE"); envFile != "" {
		configFile = envFile
	}
	// Sanitize the config file path: filepath.Clean normalises traversal components
	// and is recognised by gosec as a G703 (path traversal) sanitizer.
	configFile = filepath.Clean(configFile)
	if strings.Contains(configFile, "..") {
		return nil, fmt.Errorf("invalid config file path: %s", configFile)
	}

	data, err := os.ReadFile(configFile)
	if errors.Is(err, fs.ErrNotExist) && os.Getenv("CONFIG_FILE") == "" {
		data, err = nil, nil // no file asked for and none there: the environment carries the settings
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	// read strictly: a misspelled key would take its zero value, and for auth that value is the insecure one
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&config); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Override with environment variables
	if err := config.loadFromEnv(); err != nil {
		return nil, fmt.Errorf("failed to load from environment: %w", err)
	}

	config.applyDefaults()

	// Validate configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// applyDefaults fills in every value a config file may leave out.
func (c *Config) applyDefaults() {
	// Set default rate limits if not provided
	if c.ERPNext.RateLimit.RequestsPerSecond == 0 {
		c.ERPNext.RateLimit.RequestsPerSecond = 10
	}
	if c.ERPNext.RateLimit.Burst == 0 {
		c.ERPNext.RateLimit.Burst = 20
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	// an omitted timeout would leave the server's reads and writes unbounded; two minutes covers sid validation and a tool
	// call's 60 s deadline
	if c.Server.Timeout == 0 {
		c.Server.Timeout = 2 * time.Minute
	}
	// an omitted delay would make every retry immediate
	if c.ERPNext.Retry.InitialDelay == 0 {
		c.ERPNext.Retry.InitialDelay = 500 * time.Millisecond
	}
	if c.ERPNext.Retry.MaxDelay == 0 {
		c.ERPNext.Retry.MaxDelay = 5 * time.Second
	}
	if c.Tools.ConfirmationRedeemMethod == "" {
		c.Tools.ConfirmationRedeemMethod = DefaultConfirmationRedeemMethod
	}
	// one site URL has to point the whole server at a site: Frappe serves its own OAuth2 introspection
	if c.Auth.OAuth2.TokenInfoURL == "" && c.ERPNext.BaseURL != "" {
		c.Auth.OAuth2.TokenInfoURL = strings.TrimSuffix(c.ERPNext.BaseURL, "/") + frappeUserinfoPath
	}
}

// envString sets *dst when key carries a value.
func envString(key string, dst *string) {
	if v := os.Getenv(key); v != "" {
		*dst = v
	}
}

// envRenamed prefers key and falls back to the name it replaced, saying so when the old one is used.
func envRenamed(key, legacy string, dst *string) {
	if v := os.Getenv(key); v != "" {
		*dst = v
		return
	}
	if v := os.Getenv(legacy); v != "" {
		*dst = v
		slog.Warn(legacy + " is deprecated; rename to " + key)
	}
}

// envParsed sets *dst from key through parse. A value parse rejects is an error naming the variable,
// never a zero: "True" or "1" used to read as false and silently switch authentication off on a
// server whose config file had enabled it.
func envParsed[T any](key string, dst *T, parse func(string) (T, error)) error {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parsed, err := parse(v)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	*dst = parsed
	return nil
}

func (c *Config) loadFromEnv() error {
	// ERPNEXT_* is the deprecated fallback that keeps upgraded deployments starting; drop it no earlier than 2026-10-01.
	envRenamed("FRAPPE_BASE_URL", "ERPNEXT_BASE_URL", &c.ERPNext.BaseURL)
	envRenamed("FRAPPE_API_KEY", "ERPNEXT_API_KEY", &c.ERPNext.APIKey)
	envRenamed("FRAPPE_API_SECRET", "ERPNEXT_API_SECRET", &c.ERPNext.APISecret)

	envString("SERVER_HOST", &c.Server.Host)
	envString("LOG_LEVEL", &c.Logging.Level)
	envString("OAUTH_TOKEN_INFO_URL", &c.Auth.OAuth2.TokenInfoURL)
	envString("OAUTH_ISSUER_URL", &c.Auth.OAuth2.IssuerURL)

	// TOOLS_KNOWLEDGE_BASE: a deployment whose site has no rag turns the knowledge base off without a
	// config file of its own. The first rejection wins, in the order written here.
	for _, err := range []error{
		envParsed("SERVER_PORT", &c.Server.Port, strconv.Atoi),
		envParsed("TOOLS_KNOWLEDGE_BASE", &c.Tools.KnowledgeBase, strconv.ParseBool),
		envParsed("AUTH_ENABLED", &c.Auth.Enabled, strconv.ParseBool),
		envParsed("AUTH_REQUIRE_AUTH", &c.Auth.RequireAuth, strconv.ParseBool),
		envParsed("OAUTH_TIMEOUT", &c.Auth.OAuth2.Timeout, time.ParseDuration),
		envParsed("CACHE_TTL", &c.Auth.TokenCache.TTL, time.ParseDuration),
	} {
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) validate() error {
	if c.ERPNext.BaseURL == "" {
		return fmt.Errorf("frappe instance base URL is required")
	}

	// API key and secret are optional if OAuth2 is enabled and required
	// In that case, we'll use user OAuth2 tokens for authentication
	if !c.Auth.Enabled || !c.Auth.RequireAuth {
		// If auth is not enabled or not required, we need API key/secret
		if c.ERPNext.APIKey == "" {
			return fmt.Errorf("frappe API key is required when auth is disabled")
		}
		if c.ERPNext.APISecret == "" {
			return fmt.Errorf("frappe API secret is required when auth is disabled")
		}
	} else {
		// Auth is enabled and required - API key/secret is optional but warn if missing
		if c.ERPNext.APIKey == "" || c.ERPNext.APISecret == "" {
			// through slog, not stdout: stdio's stdout is the JSON-RPC channel, and the HTTP server's log is JSON
			slog.Info("API key/secret not provided; tool calls use each user's own session or OAuth2 token")
		}
	}

	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Auth.Enabled && c.Auth.OAuth2.IssuerURL != "" && c.Auth.OAuth2.IssuerURL != c.ERPNext.BaseURL {
		slog.Warn("auth.oauth2.issuer_url is ignored; sessions are validated against erpnext.base_url",
			"issuer_url", c.Auth.OAuth2.IssuerURL, "base_url", c.ERPNext.BaseURL)
	}

	return nil
}
