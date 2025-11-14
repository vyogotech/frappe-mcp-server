package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	// Create a temporary config file
	configContent := `
server:
  host: "localhost"
  port: 8080
  timeout: "30s"
  max_connections: 100

erpnext:
  base_url: "https://test.erpnext.com"
  api_key: "test_key"
  api_secret: "test_secret"
  timeout: "30s"
  rate_limit:
    requests_per_second: 10
    burst: 20
  retry:
    max_attempts: 3
    initial_delay: "1s"
    max_delay: "10s"

logging:
  level: "info"
  format: "json"

cache:
  ttl: "300s"
  max_size: 1000

performance:
  worker_pool_size: 10
  batch_size: 50
  enable_compression: true
`

	// Write config to temporary file
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString(configContent)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Set environment variable to use our test config
	originalConfigFile := os.Getenv("CONFIG_FILE")
	_ = os.Setenv("CONFIG_FILE", tmpFile.Name())
	defer func() {
		if originalConfigFile == "" {
			_ = os.Unsetenv("CONFIG_FILE")
		} else {
			_ = os.Setenv("CONFIG_FILE", originalConfigFile)
		}
	}()

	// Load configuration
	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Test server configuration
	assert.Equal(t, "localhost", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, 30*time.Second, cfg.Server.Timeout)
	assert.Equal(t, 100, cfg.Server.MaxConnections)

	// Test ERPNext configuration
	assert.Equal(t, "https://test.erpnext.com", cfg.ERPNext.BaseURL)
	assert.Equal(t, "test_key", cfg.ERPNext.APIKey)
	assert.Equal(t, "test_secret", cfg.ERPNext.APISecret)
	assert.Equal(t, 30*time.Second, cfg.ERPNext.Timeout)

	// Test rate limiting
	assert.Equal(t, 10, cfg.ERPNext.RateLimit.RequestsPerSecond)
	assert.Equal(t, 20, cfg.ERPNext.RateLimit.Burst)

	// Test retry configuration
	assert.Equal(t, 3, cfg.ERPNext.Retry.MaxAttempts)
	assert.Equal(t, 1*time.Second, cfg.ERPNext.Retry.InitialDelay)
	assert.Equal(t, 10*time.Second, cfg.ERPNext.Retry.MaxDelay)

	// Test logging configuration
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)

	// Test cache configuration
	assert.Equal(t, 300*time.Second, cfg.Cache.TTL)
	assert.Equal(t, 1000, cfg.Cache.MaxSize)

	// Test performance configuration
	assert.Equal(t, 10, cfg.Performance.WorkerPoolSize)
	assert.Equal(t, 50, cfg.Performance.BatchSize)
	assert.True(t, cfg.Performance.EnableCompression)
}

func TestLoadFromEnv(t *testing.T) {
	// Create minimal config file
	configContent := `
server:
  host: "default"
  port: 3000
erpnext:
  base_url: "https://default.com"
  api_key: "default_key"
  api_secret: "default_secret"
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString(configContent)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Set environment variables
	originalVars := map[string]string{
		"CONFIG_FILE":        os.Getenv("CONFIG_FILE"),
		"FRAPPE_BASE_URL":    os.Getenv("FRAPPE_BASE_URL"),
		"FRAPPE_API_KEY":     os.Getenv("FRAPPE_API_KEY"),
		"FRAPPE_API_SECRET":  os.Getenv("FRAPPE_API_SECRET"),
		"SERVER_HOST":        os.Getenv("SERVER_HOST"),
		"SERVER_PORT":        os.Getenv("SERVER_PORT"),
		"LOG_LEVEL":          os.Getenv("LOG_LEVEL"),
	}

	// Set test environment variables
	_ = os.Setenv("CONFIG_FILE", tmpFile.Name())
	_ = os.Setenv("FRAPPE_BASE_URL", "https://env.erpnext.com")
	_ = os.Setenv("FRAPPE_API_KEY", "env_key")
	_ = os.Setenv("FRAPPE_API_SECRET", "env_secret")
	_ = os.Setenv("SERVER_HOST", "env_host")
	_ = os.Setenv("SERVER_PORT", "9090")
	_ = os.Setenv("LOG_LEVEL", "debug")

	// Restore environment variables after test
	defer func() {
		for key, value := range originalVars {
			if value == "" {
				_ = os.Unsetenv(key)
			} else {
				_ = os.Setenv(key, value)
			}
		}
	}()

	// Load configuration
	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify environment variables override config file values
	assert.Equal(t, "https://env.erpnext.com", cfg.ERPNext.BaseURL)
	assert.Equal(t, "env_key", cfg.ERPNext.APIKey)
	assert.Equal(t, "env_secret", cfg.ERPNext.APISecret)
	assert.Equal(t, "env_host", cfg.Server.Host)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "debug", cfg.Logging.Level)
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid config",
			config: &Config{
				Server: ServerConfig{
					Host: "localhost",
					Port: 8080,
				},
				ERPNext: ERPNextConfig{
					BaseURL:   "https://test.erpnext.com",
					APIKey:    "test_key",
					APISecret: "test_secret",
				},
			},
			expectError: false,
		},
		{
			name: "missing base URL",
			config: &Config{
				Server: ServerConfig{
					Host: "localhost",
					Port: 8080,
				},
				ERPNext: ERPNextConfig{
					BaseURL:   "",
					APIKey:    "test_key",
					APISecret: "test_secret",
				},
			},
			expectError: true,
		},
		{
			name: "missing API key",
			config: &Config{
				Server: ServerConfig{
					Host: "localhost",
					Port: 8080,
				},
				ERPNext: ERPNextConfig{
					BaseURL:   "https://test.erpnext.com",
					APIKey:    "",
					APISecret: "test_secret",
				},
			},
			expectError: true,
		},
		{
			name: "missing API secret",
			config: &Config{
				Server: ServerConfig{
					Host: "localhost",
					Port: 8080,
				},
				ERPNext: ERPNextConfig{
					BaseURL:   "https://test.erpnext.com",
					APIKey:    "test_key",
					APISecret: "",
				},
			},
			expectError: true,
		},
		{
			name: "invalid port",
			config: &Config{
				Server: ServerConfig{
					Host: "localhost",
					Port: 70000,
				},
				ERPNext: ERPNextConfig{
					BaseURL:   "https://test.erpnext.com",
					APIKey:    "test_key",
					APISecret: "test_secret",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	// Set environment variable to non-existent file
	originalConfigFile := os.Getenv("CONFIG_FILE")
	_ = os.Setenv("CONFIG_FILE", "/non/existent/file.yaml")
	defer func() {
		if originalConfigFile == "" {
			_ = os.Unsetenv("CONFIG_FILE")
		} else {
			_ = os.Setenv("CONFIG_FILE", originalConfigFile)
		}
	}()

	// Attempt to load configuration
	cfg, err := Load()
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func BenchmarkLoad(b *testing.B) {
	// Create a test config file
	configContent := `
server:
  host: "localhost"
  port: 8080
  timeout: "30s"
erpnext:
  base_url: "https://test.erpnext.com"
  api_key: "test_key"
  api_secret: "test_secret"
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(b, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString(configContent)
	require.NoError(b, err)
	_ = tmpFile.Close()

	originalConfigFile := os.Getenv("CONFIG_FILE")
	_ = os.Setenv("CONFIG_FILE", tmpFile.Name())
	defer func() {
		if originalConfigFile == "" {
			_ = os.Unsetenv("CONFIG_FILE")
		} else {
			_ = os.Setenv("CONFIG_FILE", originalConfigFile)
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Load()
		if err != nil {
			b.Fatal(err)
		}
	}
}
