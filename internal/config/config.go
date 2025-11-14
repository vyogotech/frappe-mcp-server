package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	ERPNext     ERPNextConfig     `yaml:"erpnext"`
	Logging     LoggingConfig     `yaml:"logging"`
	LLM         LLMConfig         `yaml:"llm"`
	Cache       CacheConfig       `yaml:"cache"`
	Performance PerformanceConfig `yaml:"performance"`
	Auth        AuthConfig        `yaml:"auth"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Host           string        `yaml:"host"`
	Port           int           `yaml:"port"`
	Timeout        time.Duration `yaml:"timeout"`
	MaxConnections int           `yaml:"max_connections"`
}

// ERPNextConfig represents Frappe instance client configuration
// Named ERPNextConfig for backward compatibility, but works with any Frappe app
type ERPNextConfig struct {
	BaseURL   string          `yaml:"base_url"`
	APIKey    string          `yaml:"api_key"`
	APISecret string          `yaml:"api_secret"`
	Timeout   time.Duration   `yaml:"timeout"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Retry     RetryConfig     `yaml:"retry"`
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	RequestsPerSecond int `yaml:"requests_per_second"`
	Burst             int `yaml:"burst"`
}

// RetryConfig represents retry configuration
type RetryConfig struct {
	MaxAttempts  int           `yaml:"max_attempts"`
	InitialDelay time.Duration `yaml:"initial_delay"`
	MaxDelay     time.Duration `yaml:"max_delay"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// LLMConfig represents generic LLM provider configuration
type LLMConfig struct {
	// Provider type: "openai-compatible", "anthropic", "azure"
	// "openai-compatible" works with: OpenAI, Together.ai, Groq, Ollama, LocalAI, etc.
	ProviderType string `yaml:"provider_type"`
	
	// Generic configuration
	BaseURL     string        `yaml:"base_url"`      // API endpoint URL
	APIKey      string        `yaml:"api_key"`       // API key (can be from env)
	Model       string        `yaml:"model"`         // Model name/ID
	Timeout     time.Duration `yaml:"timeout"`       // Request timeout
	MaxTokens   int           `yaml:"max_tokens"`    // Max tokens in response
	Temperature float64       `yaml:"temperature"`   // Temperature (0.0-2.0)
	
	// Azure-specific fields (only needed if provider_type is "azure")
	AzureDeployment string `yaml:"azure_deployment,omitempty"` // Azure deployment name
	AzureAPIVersion string `yaml:"azure_api_version,omitempty"` // Azure API version
}

// CacheConfig represents caching configuration
type CacheConfig struct {
	TTL     time.Duration `yaml:"ttl"`
	MaxSize int           `yaml:"max_size"`
}

// PerformanceConfig represents performance tuning configuration
type PerformanceConfig struct {
	WorkerPoolSize    int  `yaml:"worker_pool_size"`
	BatchSize         int  `yaml:"batch_size"`
	EnableCompression bool `yaml:"enable_compression"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Enabled     bool         `yaml:"enabled"`
	RequireAuth bool         `yaml:"require_auth"`
	OAuth2      OAuth2Config `yaml:"oauth2"`
	TokenCache  TokenCacheConfig `yaml:"token_cache"`
}

// OAuth2Config represents OAuth2 configuration
type OAuth2Config struct {
	// Frappe OAuth endpoints
	TokenInfoURL string `yaml:"token_info_url"`
	IssuerURL    string `yaml:"issuer_url"`
	
	// Trusted backend clients (can provide user context headers)
	TrustedClients []string `yaml:"trusted_clients"`
	
	// Token validation
	ValidateRemote bool `yaml:"validate_remote"`
	
	// HTTP client timeout
	Timeout time.Duration `yaml:"timeout"`
}

// TokenCacheConfig represents token cache configuration
type TokenCacheConfig struct {
	TTL             time.Duration `yaml:"ttl"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
}

// Load loads configuration from config.yaml and environment variables
func Load() (*Config, error) {
	// Load .env file if it exists (ignore errors as .env file might not exist)
	_ = godotenv.Load()

	// Load from YAML file
	configFile := "config.yaml"
	if envFile := os.Getenv("CONFIG_FILE"); envFile != "" {
		configFile = envFile
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Override with environment variables
	if err := config.loadFromEnv(); err != nil {
		return nil, fmt.Errorf("failed to load from environment: %w", err)
	}

	// Validate configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// loadFromEnv loads configuration from environment variables
func (c *Config) loadFromEnv() error {
	// Frappe instance configuration
	if baseURL := os.Getenv("FRAPPE_BASE_URL"); baseURL != "" {
		c.ERPNext.BaseURL = baseURL
	}
	if apiKey := os.Getenv("FRAPPE_API_KEY"); apiKey != "" {
		c.ERPNext.APIKey = apiKey
	}
	if apiSecret := os.Getenv("FRAPPE_API_SECRET"); apiSecret != "" {
		c.ERPNext.APISecret = apiSecret
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

// validate validates the configuration
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
			// This is valid - we'll use user OAuth2 tokens
			// But log a warning for clarity
			fmt.Println("INFO: API key/secret not provided. Will use OAuth2 token pass-through for user-level permissions.")
		}
	}
	
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	
	// Validate OAuth2 config if auth is enabled
	if c.Auth.Enabled {
		if c.Auth.OAuth2.TokenInfoURL == "" {
			return fmt.Errorf("OAuth2 token_info_url is required when auth is enabled")
		}
		if c.Auth.OAuth2.IssuerURL == "" {
			return fmt.Errorf("OAuth2 issuer_url is required when auth is enabled")
		}
	}
	
	return nil
}
