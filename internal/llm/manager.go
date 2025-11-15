package llm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	
	"frappe-mcp-server/internal/config"
)

// ModelConfig represents a complete LLM model configuration
type ModelConfig struct {
	Provider    string  `json:"provider"`     // "ollama", "groq", "openai"
	Model       string  `json:"model"`        // Model name
	BaseURL     string  `json:"base_url"`     // API endpoint
	APIKey      string  `json:"api_key"`      // API key (optional for local)
	Temperature float64 `json:"temperature"`  // Generation temperature
	MaxTokens   int     `json:"max_tokens"`   // Max tokens per request
	Timeout     string  `json:"timeout"`      // Request timeout
}

// toConfigLLM converts ModelConfig to config.LLMConfig
func (mc ModelConfig) toConfigLLM() (config.LLMConfig, error) {
	timeout, err := time.ParseDuration(mc.Timeout)
	if err != nil {
		return config.LLMConfig{}, fmt.Errorf("invalid timeout: %w", err)
	}
	
	return config.LLMConfig{
		ProviderType: "openai-compatible",
		BaseURL:      mc.BaseURL,
		APIKey:       mc.APIKey,
		Model:        mc.Model,
		Timeout:      timeout,
		MaxTokens:    mc.MaxTokens,
		Temperature:  mc.Temperature,
	}, nil
}

// ModelStatus represents the current status of a model
type ModelStatus struct {
	Provider         string    `json:"provider"`
	Model            string    `json:"model"`
	BaseURL          string    `json:"base_url"`
	Status           string    `json:"status"` // "active", "rate_limited", "error", "unavailable"
	LastUsed         time.Time `json:"last_used"`
	RequestCount     int64     `json:"request_count"`
	SuccessCount     int64     `json:"success_count"`
	ErrorCount       int64     `json:"error_count"`
	RateLimitCount   int64     `json:"rate_limit_count"`
	AvgResponseTime  int64     `json:"avg_response_time_ms"`
	FallbackCount    int64     `json:"fallback_count"`
}

// Manager handles dynamic LLM model switching and fallback
type Manager struct {
	primaryClient      Client
	fallbackClient     Client
	primaryConfig      ModelConfig
	fallbackConfig     ModelConfig
	autoFallback       bool
	fallbackEnabled    bool
	metrics            *ModelStatus
	fallbackMetrics    *ModelStatus
	mutex              sync.RWMutex
	revertTimer        *time.Timer
	revertDuration     time.Duration
	rateLimitUntil     time.Time     // When the rate limit expires
	rateLimitDuration  time.Duration // How long to wait after rate limit
}

// NewManager creates a new LLM manager with primary and optional fallback
func NewManager(primaryConfig ModelConfig, fallbackConfig *ModelConfig) (*Manager, error) {
	// Convert to config.LLMConfig
	primaryCfg, err := primaryConfig.toConfigLLM()
	if err != nil {
		return nil, fmt.Errorf("invalid primary config: %w", err)
	}
	
	// Create primary client
	primaryClient, err := NewOpenAICompatibleClient(primaryCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create primary client: %w", err)
	}

	m := &Manager{
		primaryClient:   primaryClient,
		primaryConfig:   primaryConfig,
		autoFallback:    false,
		fallbackEnabled: false,
		metrics: &ModelStatus{
			Provider: primaryConfig.Provider,
			Model:    primaryConfig.Model,
			BaseURL:  primaryConfig.BaseURL,
			Status:   "active",
		},
		revertDuration: 5 * time.Minute, // Default: try primary again after 5 min
	}

	// Create fallback client if configured
	if fallbackConfig != nil && fallbackConfig.Provider != "" {
		fallbackCfg, err := fallbackConfig.toConfigLLM()
		if err != nil {
			slog.Warn("Invalid fallback config, continuing without fallback", "error", err)
		} else {
			fallbackClient, err := NewOpenAICompatibleClient(fallbackCfg)
			if err != nil {
				slog.Warn("Failed to create fallback client, continuing without fallback", "error", err)
			} else {
				m.fallbackClient = fallbackClient
				m.fallbackConfig = *fallbackConfig
				m.fallbackEnabled = true
				m.fallbackMetrics = &ModelStatus{
					Provider: fallbackConfig.Provider,
					Model:    fallbackConfig.Model,
					BaseURL:  fallbackConfig.BaseURL,
					Status:   "available",
				}
				slog.Info("Fallback LLM configured", "provider", fallbackConfig.Provider, "model", fallbackConfig.Model)
			}
		}
	}

	return m, nil
}

// Generate generates text using the current model with auto-fallback support
func (m *Manager) Generate(ctx context.Context, prompt string) (string, error) {
	m.mutex.RLock()
	client := m.primaryClient
	config := m.primaryConfig
	metrics := m.metrics
	isRateLimited := time.Now().Before(m.rateLimitUntil)
	m.mutex.RUnlock()

	// If primary is rate-limited, skip directly to fallback
	if isRateLimited {
		waitTime := time.Until(m.rateLimitUntil)
		slog.Warn("Primary LLM is rate-limited, using fallback immediately",
			"primary_provider", config.Provider,
			"wait_remaining", waitTime.Round(time.Second))
		
		if m.fallbackEnabled && m.fallbackClient != nil {
			start := time.Now()
			result, err := m.fallbackClient.Generate(ctx, prompt)
			duration := time.Since(start)
			m.updateMetrics(m.fallbackMetrics, err, duration)
			
			if err == nil {
				slog.Info("Successfully used fallback LLM (primary rate-limited)",
					"fallback_provider", m.fallbackConfig.Provider,
					"primary_available_in", waitTime.Round(time.Second))
				return result, nil
			}
			slog.Error("Fallback LLM also failed", "error", err)
		}
		
		return "", fmt.Errorf("primary LLM rate-limited (available in %v), fallback unavailable or failed", waitTime.Round(time.Second))
	}

	// Try primary
	start := time.Now()
	result, err := client.Generate(ctx, prompt)
	duration := time.Since(start)

	// Update metrics
	m.updateMetrics(metrics, err, duration)

	// Check if we should fallback
	if err != nil && m.shouldFallback(err) {
		// Parse retry-after time from error if available
		retryAfter := m.parseRetryAfter(err)
		if retryAfter > 0 {
			m.mutex.Lock()
			m.rateLimitUntil = time.Now().Add(retryAfter)
			m.rateLimitDuration = retryAfter
			m.mutex.Unlock()
			
			slog.Warn("Primary LLM rate-limited, setting cooldown",
				"primary_provider", config.Provider,
				"retry_after", retryAfter.Round(time.Second))
		}
		
		slog.Warn("Primary LLM failed, attempting fallback",
			"primary_provider", config.Provider,
			"primary_model", config.Model,
			"error", err)

		// Try fallback
		if m.fallbackEnabled && m.fallbackClient != nil {
			m.mutex.Lock()
			m.metrics.Status = "rate_limited"
			m.metrics.FallbackCount++
			m.mutex.Unlock()

			start = time.Now()
			result, err = m.fallbackClient.Generate(ctx, prompt)
			duration = time.Since(start)

			m.updateMetrics(m.fallbackMetrics, err, duration)

			if err == nil {
				slog.Info("Successfully used fallback LLM",
					"fallback_provider", m.fallbackConfig.Provider,
					"fallback_model", m.fallbackConfig.Model)

				// Schedule revert to primary after rate limit expires
				if retryAfter > 0 {
					m.scheduleRevertAfterRateLimit(retryAfter)
				} else {
					m.scheduleRevertToPrimary()
				}
				return result, nil
			}

			slog.Error("Fallback LLM also failed", "error", err)
		}
	}

	return result, err
}

// SwitchModel switches to a new model configuration at runtime
func (m *Manager) SwitchModel(config ModelConfig, persist bool) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Convert to config.LLMConfig
	cfg, err := config.toConfigLLM()
	if err != nil {
		return fmt.Errorf("invalid model config: %w", err)
	}

	// Create new client
	newClient, err := NewOpenAICompatibleClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create new client: %w", err)
	}

	// Test the new client
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = newClient.Generate(ctx, "test")
	if err != nil {
		slog.Warn("New model test failed, but switching anyway", "error", err)
	}

	// Save old config for logging
	oldConfig := m.primaryConfig

	// Switch
	m.primaryClient = newClient
	m.primaryConfig = config
	m.metrics = &ModelStatus{
		Provider: config.Provider,
		Model:    config.Model,
		BaseURL:  config.BaseURL,
		Status:   "active",
	}

	slog.Info("Successfully switched LLM model",
		"old_provider", oldConfig.Provider,
		"old_model", oldConfig.Model,
		"new_provider", config.Provider,
		"new_model", config.Model,
		"persist", persist)

	// TODO: If persist is true, update config.yaml

	return nil
}

// SetFallback sets or updates the fallback model
func (m *Manager) SetFallback(config ModelConfig, autoEnable bool) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Convert to config.LLMConfig
	cfg, err := config.toConfigLLM()
	if err != nil {
		return fmt.Errorf("invalid fallback config: %w", err)
	}

	fallbackClient, err := NewOpenAICompatibleClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create fallback client: %w", err)
	}

	m.fallbackClient = fallbackClient
	m.fallbackConfig = config
	m.fallbackEnabled = true
	m.autoFallback = autoEnable
	m.fallbackMetrics = &ModelStatus{
		Provider: config.Provider,
		Model:    config.Model,
		BaseURL:  config.BaseURL,
		Status:   "available",
	}

	slog.Info("Fallback model configured",
		"provider", config.Provider,
		"model", config.Model,
		"auto_enabled", autoEnable)

	return nil
}

// EnableAutoFallback enables or disables automatic fallback
func (m *Manager) EnableAutoFallback(enabled bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.autoFallback = enabled
	slog.Info("Auto-fallback setting changed", "enabled", enabled)
}

// GetStatus returns the current status of primary and fallback models
func (m *Manager) GetStatus() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	status := map[string]interface{}{
		"primary": map[string]interface{}{
			"provider":           m.primaryConfig.Provider,
			"model":              m.primaryConfig.Model,
			"base_url":           m.primaryConfig.BaseURL,
			"status":             m.metrics.Status,
			"request_count":      m.metrics.RequestCount,
			"success_count":      m.metrics.SuccessCount,
			"error_count":        m.metrics.ErrorCount,
			"rate_limit_count":   m.metrics.RateLimitCount,
			"avg_response_time":  m.metrics.AvgResponseTime,
			"fallback_count":     m.metrics.FallbackCount,
		},
		"auto_fallback_enabled": m.autoFallback,
		"fallback_available":    m.fallbackEnabled,
	}

	if m.fallbackEnabled {
		status["fallback"] = map[string]interface{}{
			"provider":           m.fallbackConfig.Provider,
			"model":              m.fallbackConfig.Model,
			"base_url":           m.fallbackConfig.BaseURL,
			"status":             m.fallbackMetrics.Status,
			"request_count":      m.fallbackMetrics.RequestCount,
			"success_count":      m.fallbackMetrics.SuccessCount,
			"error_count":        m.fallbackMetrics.ErrorCount,
		}
	}

	return status
}

// shouldFallback determines if we should fallback based on the error
func (m *Manager) shouldFallback(err error) bool {
	if !m.autoFallback || !m.fallbackEnabled {
		return false
	}

	errStr := err.Error()
	// Check for rate limit (429)
	if contains(errStr, "429") || contains(errStr, "rate limit") {
		return true
	}
	// Check for timeout
	if contains(errStr, "timeout") || contains(errStr, "deadline exceeded") {
		return true
	}
	// Check for server errors (5xx)
	if contains(errStr, "500") || contains(errStr, "502") || contains(errStr, "503") {
		return true
	}

	return false
}

// updateMetrics updates the metrics for a model
func (m *Manager) updateMetrics(metrics *ModelStatus, err error, duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	metrics.RequestCount++
	metrics.LastUsed = time.Now()

	if err == nil {
		metrics.SuccessCount++
		metrics.Status = "active"
	} else {
		metrics.ErrorCount++
		if contains(err.Error(), "429") || contains(err.Error(), "rate limit") {
			metrics.RateLimitCount++
			metrics.Status = "rate_limited"
		} else {
			metrics.Status = "error"
		}
	}

	// Update average response time
	if metrics.RequestCount > 0 {
		metrics.AvgResponseTime = (metrics.AvgResponseTime*(metrics.RequestCount-1) + duration.Milliseconds()) / metrics.RequestCount
	}
}

// scheduleRevertToPrimary schedules a revert back to the primary model
func (m *Manager) scheduleRevertToPrimary() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Cancel existing timer if any
	if m.revertTimer != nil {
		m.revertTimer.Stop()
	}

	// Schedule revert
	m.revertTimer = time.AfterFunc(m.revertDuration, func() {
		m.mutex.Lock()
		defer m.mutex.Unlock()
		m.metrics.Status = "active"
		m.rateLimitUntil = time.Time{} // Clear rate limit
		slog.Info("Reverted to primary LLM after cooldown", "duration", m.revertDuration)
	})

	slog.Info("Scheduled revert to primary LLM", "after", m.revertDuration)
}

// scheduleRevertAfterRateLimit schedules a revert after the rate limit expires
func (m *Manager) scheduleRevertAfterRateLimit(retryAfter time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Cancel existing timer if any
	if m.revertTimer != nil {
		m.revertTimer.Stop()
	}

	// Schedule revert after rate limit expires
	m.revertTimer = time.AfterFunc(retryAfter, func() {
		m.mutex.Lock()
		defer m.mutex.Unlock()
		m.metrics.Status = "active"
		m.rateLimitUntil = time.Time{} // Clear rate limit
		slog.Info("Rate limit expired, primary LLM now available",
			"was_limited_for", retryAfter.Round(time.Second))
	})

	slog.Info("Scheduled rate limit recovery", "after", retryAfter.Round(time.Second))
}

// parseRetryAfter extracts retry-after duration from error message
// Groq format: "Please try again in 9m38.016s"
// Returns 0 if not found
func (m *Manager) parseRetryAfter(err error) time.Duration {
	if err == nil {
		return 0
	}

	errStr := err.Error()
	
	// Look for patterns like "try again in 9m38s" or "try again in 9m38.016s"
	patterns := []string{
		"try again in ",
		"retry after ",
		"available in ",
	}
	
	for _, pattern := range patterns {
		idx := strings.Index(strings.ToLower(errStr), pattern)
		if idx == -1 {
			continue
		}
		
		// Extract the time string after the pattern
		timeStr := errStr[idx+len(pattern):]
		
		// Find the end of the duration (next non-duration character)
		endIdx := 0
		for i, ch := range timeStr {
			if !isDurationChar(ch) {
				endIdx = i
				break
			}
		}
		
		if endIdx > 0 {
			timeStr = timeStr[:endIdx]
			
			// Try to parse as duration
			duration, err := time.ParseDuration(timeStr)
			if err == nil && duration > 0 {
				slog.Info("Parsed retry-after from error", "duration", duration.Round(time.Second), "raw", timeStr)
				return duration
			}
		}
	}
	
	// Fallback: if rate limit but no duration found, use a default (10 minutes)
	if strings.Contains(strings.ToLower(errStr), "rate limit") {
		return 10 * time.Minute
	}
	
	return 0
}

// isDurationChar checks if a character is valid in a duration string
func isDurationChar(ch rune) bool {
	return (ch >= '0' && ch <= '9') || ch == 'h' || ch == 'm' || ch == 's' || ch == '.' || ch == 'µ' || ch == 'n'
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

