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
	Server      ServerConfig      `yaml:"server"`
	ERPNext     ERPNextConfig     `yaml:"erpnext"`
	Logging     LoggingConfig     `yaml:"logging"`
	LLM         LLMConfig         `yaml:"llm"`
	Cache       CacheConfig       `yaml:"cache"`
	Performance PerformanceConfig `yaml:"performance"`
	Auth        AuthConfig        `yaml:"auth"`
	Tools       ToolsConfig       `yaml:"tools"`
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
	Host           string        `yaml:"host"`
	Port           int           `yaml:"port"`
	Timeout        time.Duration `yaml:"timeout"`
	MaxConnections int           `yaml:"max_connections"`
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
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type LLMConfig struct {
	// Provider type: "openai-compatible", "anthropic", "azure"
	// "openai-compatible" works with: OpenAI, Together.ai, Groq, Ollama, LocalAI, etc.
	ProviderType string `yaml:"provider_type"`

	// Generic configuration
	BaseURL     string        `yaml:"base_url"`    // API endpoint URL
	APIKey      string        `yaml:"api_key"`     // API key (can be from env)
	Model       string        `yaml:"model"`       // Model name/ID
	Timeout     time.Duration `yaml:"timeout"`     // Request timeout
	MaxTokens   int           `yaml:"max_tokens"`  // Max tokens in response
	Temperature float64       `yaml:"temperature"` // Temperature (0.0-2.0)

	// Fallback configuration (optional)
	Fallback *LLMFallbackConfig `yaml:"fallback,omitempty"` // Fallback model config

	// Azure-specific fields (only needed if provider_type is "azure")
	AzureDeployment string `yaml:"azure_deployment,omitempty"`  // Azure deployment name
	AzureAPIVersion string `yaml:"azure_api_version,omitempty"` // Azure API version
}

type LLMFallbackConfig struct {
	Enabled     bool          `yaml:"enabled"`     // Enable fallback
	BaseURL     string        `yaml:"base_url"`    // Fallback API endpoint
	APIKey      string        `yaml:"api_key"`     // Fallback API key
	Model       string        `yaml:"model"`       // Fallback model name
	Timeout     time.Duration `yaml:"timeout"`     // Fallback timeout
	MaxTokens   int           `yaml:"max_tokens"`  // Fallback max tokens
	Temperature float64       `yaml:"temperature"` // Fallback temperature
	AutoSwitch  bool          `yaml:"auto_switch"` // Auto-switch on rate limit
}

type CacheConfig struct {
	TTL     time.Duration `yaml:"ttl"`
	MaxSize int           `yaml:"max_size"`
}

type PerformanceConfig struct {
	WorkerPoolSize    int  `yaml:"worker_pool_size"`
	BatchSize         int  `yaml:"batch_size"`
	EnableCompression bool `yaml:"enable_compression"`
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
	TTL             time.Duration `yaml:"ttl"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
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

	// Set default rate limits if not provided
	if config.ERPNext.RateLimit.RequestsPerSecond == 0 {
		config.ERPNext.RateLimit.RequestsPerSecond = 10
	}
	if config.ERPNext.RateLimit.Burst == 0 {
		config.ERPNext.RateLimit.Burst = 20
	}
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}
	// an omitted timeout would leave the server's reads and writes unbounded; two minutes covers sid validation and a tool
	// call's 60 s deadline
	if config.Server.Timeout == 0 {
		config.Server.Timeout = 2 * time.Minute
	}
	// an omitted delay would make every retry immediate
	if config.ERPNext.Retry.InitialDelay == 0 {
		config.ERPNext.Retry.InitialDelay = 500 * time.Millisecond
	}
	if config.ERPNext.Retry.MaxDelay == 0 {
		config.ERPNext.Retry.MaxDelay = 5 * time.Second
	}
	if config.Tools.ConfirmationRedeemMethod == "" {
		config.Tools.ConfirmationRedeemMethod = DefaultConfirmationRedeemMethod
	}
	// one site URL has to point the whole server at a site: Frappe serves its own OAuth2 introspection
	if config.Auth.OAuth2.TokenInfoURL == "" && config.ERPNext.BaseURL != "" {
		config.Auth.OAuth2.TokenInfoURL = strings.TrimSuffix(config.ERPNext.BaseURL, "/") + frappeUserinfoPath
	}

	// Validate configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

func (c *Config) loadFromEnv() error {
	// ERPNEXT_* is the deprecated fallback that keeps upgraded deployments starting; drop it no earlier than 2026-10-01.
	if baseURL := os.Getenv("FRAPPE_BASE_URL"); baseURL != "" {
		c.ERPNext.BaseURL = baseURL
	} else if legacy := os.Getenv("ERPNEXT_BASE_URL"); legacy != "" {
		c.ERPNext.BaseURL = legacy
		slog.Warn("ERPNEXT_BASE_URL is deprecated; rename to FRAPPE_BASE_URL")
	}
	if apiKey := os.Getenv("FRAPPE_API_KEY"); apiKey != "" {
		c.ERPNext.APIKey = apiKey
	} else if legacy := os.Getenv("ERPNEXT_API_KEY"); legacy != "" {
		c.ERPNext.APIKey = legacy
		slog.Warn("ERPNEXT_API_KEY is deprecated; rename to FRAPPE_API_KEY")
	}
	if apiSecret := os.Getenv("FRAPPE_API_SECRET"); apiSecret != "" {
		c.ERPNext.APISecret = apiSecret
	} else if legacy := os.Getenv("ERPNEXT_API_SECRET"); legacy != "" {
		c.ERPNext.APISecret = legacy
		slog.Warn("ERPNEXT_API_SECRET is deprecated; rename to FRAPPE_API_SECRET")
	}

	// Server configuration
	if host := os.Getenv("SERVER_HOST"); host != "" {
		c.Server.Host = host
	}
	if port := os.Getenv("SERVER_PORT"); port != "" {
		// Parse port from string
		var portInt int
		if _, err := fmt.Sscanf(port, "%d", &portInt); err == nil {
			c.Server.Port = portInt
		}
	}

	// Logging configuration
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		c.Logging.Level = level
	}
	// a deployment whose site has no rag turns the knowledge base off without a config file of its own
	if kb := os.Getenv("TOOLS_KNOWLEDGE_BASE"); kb != "" {
		on, err := strconv.ParseBool(kb)
		if err != nil {
			return fmt.Errorf("TOOLS_KNOWLEDGE_BASE: %w", err)
		}
		c.Tools.KnowledgeBase = on
	}

	// LLM Provider configuration
	if providerType := os.Getenv("LLM_PROVIDER_TYPE"); providerType != "" {
		c.LLM.ProviderType = providerType
	}
	if baseURL := os.Getenv("LLM_BASE_URL"); baseURL != "" {
		c.LLM.BaseURL = baseURL
	}
	if apiKey := os.Getenv("LLM_API_KEY"); apiKey != "" {
		c.LLM.APIKey = apiKey
	}
	if model := os.Getenv("LLM_MODEL"); model != "" {
		c.LLM.Model = model
	}
	if azureDeployment := os.Getenv("LLM_AZURE_DEPLOYMENT"); azureDeployment != "" {
		c.LLM.AzureDeployment = azureDeployment
	}
	if azureAPIVersion := os.Getenv("LLM_AZURE_API_VERSION"); azureAPIVersion != "" {
		c.LLM.AzureAPIVersion = azureAPIVersion
	}

	// Auth configuration
	if enabled := os.Getenv("AUTH_ENABLED"); enabled != "" {
		c.Auth.Enabled = enabled == "true"
	}
	if requireAuth := os.Getenv("AUTH_REQUIRE_AUTH"); requireAuth != "" {
		c.Auth.RequireAuth = requireAuth == "true"
	}
	if tokenInfoURL := os.Getenv("OAUTH_TOKEN_INFO_URL"); tokenInfoURL != "" {
		c.Auth.OAuth2.TokenInfoURL = tokenInfoURL
	}
	if issuerURL := os.Getenv("OAUTH_ISSUER_URL"); issuerURL != "" {
		c.Auth.OAuth2.IssuerURL = issuerURL
	}
	if timeout := os.Getenv("OAUTH_TIMEOUT"); timeout != "" {
		if duration, err := time.ParseDuration(timeout); err == nil {
			c.Auth.OAuth2.Timeout = duration
		}
	}
	if cacheTTL := os.Getenv("CACHE_TTL"); cacheTTL != "" {
		if duration, err := time.ParseDuration(cacheTTL); err == nil {
			c.Auth.TokenCache.TTL = duration
		}
	}
	if cleanupInterval := os.Getenv("CACHE_CLEANUP_INTERVAL"); cleanupInterval != "" {
		if duration, err := time.ParseDuration(cleanupInterval); err == nil {
			c.Auth.TokenCache.CleanupInterval = duration
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
