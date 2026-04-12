package llm

import (
	"bufio"
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

// GenerateStream implements the Streamer interface.
// It requests a streaming completion (stream:true) and sends each content
// token into the returned channel, closing it when done or on error.
func (c *OpenAICompatibleClient) GenerateStream(ctx context.Context, prompt string) (<-chan string, error) {
	requestBody := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens":  c.maxTokens,
		"temperature": c.temperature,
		"stream":      true,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal stream request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create stream request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	// Use a client without a response-body timeout so we can stream.
	streamClient := &http.Client{Transport: c.client.Transport}
	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to start stream: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("stream request returned status %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan string, 64)

	go func() {
		defer close(ch)
		defer func() { _ = resp.Body.Close() }()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			// SSE format: "data: <json>" or "data: [DONE]"
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			payload := strings.TrimPrefix(line, "data: ")
			if payload == "[DONE]" {
				return
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
				continue
			}
			if len(chunk.Choices) == 0 {
				continue
			}

			token := chunk.Choices[0].Delta.Content
			if token == "" {
				continue
			}

			select {
			case ch <- token:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}
