package llm

import (
	"context"
	"fmt"
	"time"

	"frappe-mcp-server/internal/config"
)

// Client is the interface for LLM providers
type Client interface {
	// Generate generates a completion from the given prompt
	Generate(ctx context.Context, prompt string) (string, error)
	
	// Provider returns the provider name
	Provider() string
}

// NewClient creates an LLM client based on configuration
func NewClient(cfg config.LLMConfig) (Client, error) {
	// Validate common fields
	if cfg.Model == "" {
		return nil, fmt.Errorf("LLM model is required")
	}
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("LLM base_url is required")
	}
	
	// Set defaults
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 500
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.7
	}
	
	// Determine provider type (default to openai-compatible)
	providerType := cfg.ProviderType
	if providerType == "" {
		providerType = "openai-compatible"
	}
	
	switch providerType {
	case "openai-compatible", "openai", "ollama":
		// OpenAI-compatible API (works with OpenAI, Together.ai, Groq, Ollama, etc.)
		return NewOpenAICompatibleClient(cfg)
		
	case "anthropic":
		// Anthropic has a different API format
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("API key is required for Anthropic")
		}
		return NewAnthropicClient(cfg)
		
	case "azure":
		// Azure OpenAI has special URL format
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("API key is required for Azure")
		}
		if cfg.AzureDeployment == "" {
			return nil, fmt.Errorf("Azure deployment is required")
		}
		return NewAzureClient(cfg)
		
	default:
		return nil, fmt.Errorf("unsupported provider_type: %s (use: openai-compatible, anthropic, or azure)", providerType)
	}
}

