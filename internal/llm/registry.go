package llm

import "fmt"

// PresetModel represents a preset model configuration
type PresetModel struct {
	Name        string      `json:"name"`
	Provider    string      `json:"provider"`
	Model       string      `json:"model"`
	BaseURL     string      `json:"base_url"`
	Description string      `json:"description"`
	Cost        string      `json:"cost"`
	RamRequired string      `json:"ram_required,omitempty"`
	Config      ModelConfig `json:"config"`
}

// ModelRegistry holds all available preset models
var ModelRegistry = map[string]PresetModel{
	"groq-llama-70b": {
		Name:        "groq-llama-70b",
		Provider:    "groq",
		Model:       "llama-3.3-70b-versatile",
		BaseURL:     "https://api.groq.com/openai/v1",
		Description: "Ultra-fast 70B model with excellent accuracy",
		Cost:        "free-tier-100k-tokens-daily",
		Config: ModelConfig{
			Provider:    "groq",
			Model:       "llama-3.3-70b-versatile",
			BaseURL:     "https://api.groq.com/openai/v1",
			Temperature: 0.3,
			MaxTokens:   1000,
			Timeout:     "30s",
		},
	},
	"groq-llama-8b": {
		Name:        "groq-llama-8b",
		Provider:    "groq",
		Model:       "llama-3.1-8b-instant",
		BaseURL:     "https://api.groq.com/openai/v1",
		Description: "Fast 8B model for simple queries",
		Cost:        "free-tier-100k-tokens-daily",
		Config: ModelConfig{
			Provider:    "groq",
			Model:       "llama-3.1-8b-instant",
			BaseURL:     "https://api.groq.com/openai/v1",
			Temperature: 0.3,
			MaxTokens:   1000,
			Timeout:     "30s",
		},
	},
	"ollama-llama-8b": {
		Name:        "ollama-llama-8b",
		Provider:    "ollama",
		Model:       "llama3.1:latest",
		BaseURL:     "http://ollama:11434/v1",
		Description: "Local 8B model, unlimited usage",
		Cost:        "free-unlimited",
		RamRequired: "~5GB",
		Config: ModelConfig{
			Provider:    "ollama",
			Model:       "llama3.1:latest",
			BaseURL:     "http://ollama:11434/v1",
			APIKey:      "",
			Temperature: 0.3,
			MaxTokens:   1000,
			Timeout:     "60s",
		},
	},
	"ollama-llama-1b": {
		Name:        "ollama-llama-1b",
		Provider:    "ollama",
		Model:       "llama3.2:1b",
		BaseURL:     "http://ollama:11434/v1",
		Description: "Very small local model, fast but less accurate",
		Cost:        "free-unlimited",
		RamRequired: "~1GB",
		Config: ModelConfig{
			Provider:    "ollama",
			Model:       "llama3.2:1b",
			BaseURL:     "http://ollama:11434/v1",
			APIKey:      "",
			Temperature: 0.3,
			MaxTokens:   1000,
			Timeout:     "60s",
		},
	},
	"ollama-mixtral": {
		Name:        "ollama-mixtral",
		Provider:    "ollama",
		Model:       "mixtral:8x7b",
		BaseURL:     "http://ollama:11434/v1",
		Description: "Large local MoE model, high accuracy",
		Cost:        "free-unlimited",
		RamRequired: "~26GB",
		Config: ModelConfig{
			Provider:    "ollama",
			Model:       "mixtral:8x7b",
			BaseURL:     "http://ollama:11434/v1",
			APIKey:      "",
			Temperature: 0.3,
			MaxTokens:   1000,
			Timeout:     "120s",
		},
	},
}

// GetPresetModel returns a preset model configuration by name
func GetPresetModel(name string) (PresetModel, bool) {
	model, exists := ModelRegistry[name]
	return model, exists
}

// ListPresetModels returns all available preset models
func ListPresetModels() []PresetModel {
	models := make([]PresetModel, 0, len(ModelRegistry))
	for _, model := range ModelRegistry{
		models = append(models, model)
	}
	return models
}

// ValidateModelConfig validates a model configuration
func ValidateModelConfig(config ModelConfig) error {
	if config.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if config.Model == "" {
		return fmt.Errorf("model is required")
	}
	if config.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}
	if config.Temperature < 0 || config.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2")
	}
	if config.MaxTokens <= 0 {
		return fmt.Errorf("max_tokens must be positive")
	}
	return nil
}

