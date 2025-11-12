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

// AzureClient implements the Client interface for Azure OpenAI
type AzureClient struct {
	apiKey      string
	endpoint    string
	deployment  string
	apiVersion  string
	maxTokens   int
	temperature float64
	client      *http.Client
}

// NewAzureClient creates a new Azure OpenAI client
func NewAzureClient(cfg config.LLMConfig) (*AzureClient, error) {
	apiVersion := cfg.AzureAPIVersion
	if apiVersion == "" {
		apiVersion = "2024-02-01"
	}
	
	return &AzureClient{
		apiKey:      cfg.APIKey,
		endpoint:    cfg.BaseURL,
		deployment:  cfg.AzureDeployment,
		apiVersion:  apiVersion,
		maxTokens:   cfg.MaxTokens,
		temperature: cfg.Temperature,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

// Provider returns the provider name
func (c *AzureClient) Provider() string {
	return "azure"
}

// Generate generates a completion from the given prompt
func (c *AzureClient) Generate(ctx context.Context, prompt string) (string, error) {
	requestBody := map[string]interface{}{
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

	// Azure OpenAI endpoint format
	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		c.endpoint, c.deployment, c.apiVersion)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call Azure OpenAI API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("azure OpenAI API returned status %d: %s", resp.StatusCode, string(body))
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
		return "", fmt.Errorf("no response from Azure OpenAI")
	}

	return result.Choices[0].Message.Content, nil
}

