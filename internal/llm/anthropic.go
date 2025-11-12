package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"frappe-mcp-server/internal/config"
)

// AnthropicClient implements the Client interface for Anthropic Claude
type AnthropicClient struct {
	apiKey      string
	model       string
	maxTokens   int
	temperature float64
	client      *http.Client
}

// NewAnthropicClient creates a new Anthropic client
func NewAnthropicClient(cfg config.LLMConfig) (*AnthropicClient, error) {
	return &AnthropicClient{
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		maxTokens:   cfg.MaxTokens,
		temperature: cfg.Temperature,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

// Provider returns the provider name
func (c *AnthropicClient) Provider() string {
	return "anthropic"
}

// Generate generates a completion from the given prompt
func (c *AnthropicClient) Generate(ctx context.Context, prompt string) (string, error) {
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

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call Anthropic API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("anthropic API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("no response from Anthropic")
	}

	return result.Content[0].Text, nil
}

