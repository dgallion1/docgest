package extract

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// ClaudeClient calls the Anthropic Messages API for fact extraction.
type ClaudeClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewClaudeClient(apiKey, model string) *ClaudeClient {
	return &ClaudeClient{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// ExtractFacts calls Claude to extract facts from a chunk prompt.
func (c *ClaudeClient) ExtractFacts(ctx context.Context, prompt string) ([]Fact, error) {
	reqBody := anthropicRequest{
		Model:     c.model,
		MaxTokens: 4096,
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("claude api: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, &RetryableError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
		}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("claude api status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if apiResp.Error != nil {
		return nil, fmt.Errorf("claude error: %s: %s", apiResp.Error.Type, apiResp.Error.Message)
	}
	if len(apiResp.Content) == 0 {
		return nil, fmt.Errorf("empty response from claude")
	}

	text := apiResp.Content[0].Text
	text = stripCodeBlock(text)

	var facts []Fact
	if err := json.Unmarshal([]byte(text), &facts); err != nil {
		return nil, fmt.Errorf("parse facts json: %w (raw: %s)", err, truncate(text, 200))
	}

	return facts, nil
}

var codeBlockRe = regexp.MustCompile("(?s)^```(?:json)?\\s*(.*?)\\s*```$")

func stripCodeBlock(s string) string {
	s = strings.TrimSpace(s)
	if m := codeBlockRe.FindStringSubmatch(s); len(m) > 1 {
		return m[1]
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// RetryableError indicates a transient failure that can be retried.
type RetryableError struct {
	StatusCode int
	Message    string
}

func (e *RetryableError) Error() string {
	return fmt.Sprintf("retryable error (status %d): %s", e.StatusCode, truncate(e.Message, 200))
}

// Close releases resources.
func (c *ClaudeClient) Close() {
	c.httpClient.CloseIdleConnections()
}
