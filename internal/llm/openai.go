package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"frappe-mcp-server/internal/config"
)

// OpenAICompatibleClient implements the Client interface for OpenAI-compatible APIs
// This works with: OpenAI, Together.ai, Groq, Ollama, LocalAI, LM Studio, OpenRouter, Replicate, etc.
type OpenAICompatibleClient struct {
	apiKey      string
	model       string
	maxTokens   int
	temperature float64
	baseURL     string
	client      *http.Client
	providerName string
}

// NewOpenAICompatibleClient creates a new OpenAI-compatible client
func NewOpenAICompatibleClient(cfg config.LLMConfig) (*OpenAICompatibleClient, error) {
	// Detect provider name from base URL for better logging
	providerName := "openai-compatible"
	if strings.Contains(cfg.BaseURL, "together") {
		providerName = "together.ai"
	} else if strings.Contains(cfg.BaseURL, "groq") {
		providerName = "groq"
	} else if strings.Contains(cfg.BaseURL, "openrouter") {
		providerName = "openrouter"
	} else if strings.Contains(cfg.BaseURL, "replicate") {
		providerName = "replicate"
	} else if strings.Contains(cfg.BaseURL, "openai.com") {
		providerName = "openai"
	} else if strings.Contains(cfg.BaseURL, "localhost:11434") || strings.Contains(cfg.BaseURL, "ollama") {
		providerName = "ollama"
	} else if strings.Contains(cfg.BaseURL, "localhost") {
		providerName = "local"
	}
	
	return &OpenAICompatibleClient{
		apiKey:       cfg.APIKey,
		model:        cfg.Model,
		maxTokens:    cfg.MaxTokens,
		temperature:  cfg.Temperature,
		baseURL:      cfg.BaseURL,
		providerName: providerName,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

// Provider returns the provider name
func (c *OpenAICompatibleClient) Provider() string {
	return c.providerName
}

// Generate generates a completion from the given prompt
func (c *OpenAICompatibleClient) Generate(ctx context.Context, prompt string) (string, error) {
	requestBody := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"max_tokens":  c.maxTokens,
		"temperature": c.temperature,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call OpenAI API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenAI API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return result.Choices[0].Message.Content, nil
}

