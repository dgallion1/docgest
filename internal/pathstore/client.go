package pathstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client communicates with the pathstore HTTP API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NodeRequest is the body for PUT /kv/{key}.
type NodeRequest struct {
	Value      any     `json:"value"`
	MergeMode  string  `json:"merge_mode,omitempty"`
	MemoryType string  `json:"memory_type,omitempty"`
	Salience   float64 `json:"salience,omitempty"`
	Source     string  `json:"source,omitempty"`
	ExpiresAt  string  `json:"expires_at,omitempty"`
}

// NodeResponse is the response from GET /kv/{key}.
type NodeResponse struct {
	Key        string  `json:"key_path"`
	Value      any     `json:"value"`
	OrigKey    string  `json:"metadata.original_key,omitempty"`
	MemoryType string  `json:"memory_type,omitempty"`
	Salience   float64 `json:"salience,omitempty"`
}

// LinkRequest is the body for PUT /links.
type LinkRequest struct {
	From          string  `json:"from_key"`
	To            string  `json:"to_key"`
	Weight        float64 `json:"weight"`
	Summary       string  `json:"summary,omitempty"`
	Bidirectional bool    `json:"bidirectional,omitempty"`
}

// PutNode stores or updates a node at the given path.
func (c *Client) PutNode(ctx context.Context, key string, req NodeRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal node: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/kv/"+key, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("put node: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("put node %s: status %d: %s", key, resp.StatusCode, string(respBody))
	}
	return nil
}

// GetNode retrieves a node by key.
func (c *Client) GetNode(ctx context.Context, key string) (*NodeResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/kv/"+key, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("get node: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("get node %s: status %d: %s", key, resp.StatusCode, string(respBody))
	}

	var node NodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&node); err != nil {
		return nil, fmt.Errorf("decode node: %w", err)
	}
	return &node, nil
}

// DeleteNode deletes a node and optionally its children.
func (c *Client) DeleteNode(ctx context.Context, key string, recursive bool) error {
	u := c.baseURL + "/kv/" + key
	if recursive {
		u += "?children=true"
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("delete node: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("delete node %s: status %d: %s", key, resp.StatusCode, string(respBody))
	}
	return nil
}

// ListChildrenResponse is a single node from a prefix scan.
type ListChildrenResponse struct {
	Key   string `json:"key_path"`
	Value any    `json:"value"`
}

// ListChildren does a prefix scan under the given key.
func (c *Client) ListChildren(ctx context.Context, key string, limit int) ([]ListChildrenResponse, error) {
	u := c.baseURL + "/kv/" + key + "/*"
	if limit > 0 {
		u += "?limit=" + url.QueryEscape(fmt.Sprintf("%d", limit))
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("list children: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("list children %s: status %d: %s", key, resp.StatusCode, string(respBody))
	}

	var result struct {
		Nodes []ListChildrenResponse `json:"nodes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode children: %w", err)
	}
	return result.Nodes, nil
}

// PutLink creates or updates an edge between two nodes.
func (c *Client) PutLink(ctx context.Context, req LinkRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal link: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/links", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("put link: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("put link: status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// Close releases any resources (currently a no-op).
func (c *Client) Close() {
	c.httpClient.CloseIdleConnections()
}
