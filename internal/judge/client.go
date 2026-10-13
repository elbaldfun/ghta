package judge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to an OpenAI-compatible chat endpoint (LM Studio) with a JSON
// Schema attached to every request.
//
// The schema is what makes a local reasoning model usable here: with
// response_format json_schema the server constrains decoding, the model skips
// its thinking phase (seconds instead of minutes per batch) and cannot invent
// a category or drop an item. LM Studio then files the answer under
// reasoning_content and leaves content empty, so both are read.
type Client struct {
	BaseURL     string // e.g. http://192.168.50.182:1234/v1
	Model       string
	APIKey      string
	Temperature float64
	HTTP        *http.Client
}

// NewClient returns a client for baseURL. A model on a private or loopback
// address is reached directly even when the environment configures a proxy.
func NewClient(baseURL, model, apiKey string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"), Model: model, APIKey: apiKey, Temperature: 0.2,
		HTTP: &http.Client{Timeout: 15 * time.Minute, Transport: &http.Transport{Proxy: proxyUnlessLocal}},
	}
}

func proxyUnlessLocal(r *http.Request) (*url.URL, error) {
	host := r.URL.Hostname()
	if ip := net.ParseIP(host); host == "localhost" || (ip != nil && (ip.IsLoopback() || ip.IsPrivate())) {
		return nil, nil
	}
	return http.ProxyFromEnvironment(r)
}

type chatRequest struct {
	Model          string        `json:"model"`
	Temperature    float64       `json:"temperature"`
	MaxTokens      int           `json:"max_tokens"`
	Messages       []chatMessage `json:"messages"`
	ResponseFormat any           `json:"response_format"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete sends one chat completion constrained to schema and returns the
// JSON text of the answer.
func (c *Client) Complete(ctx context.Context, system, user string, schema json.RawMessage, maxTokens int) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model: c.Model, Temperature: c.Temperature, MaxTokens: maxTokens,
		Messages: []chatMessage{{Role: "system", Content: system}, {Role: "user", Content: user}},
		ResponseFormat: map[string]any{"type": "json_schema",
			"json_schema": map[string]any{"name": "verdicts", "strict": true, "schema": schema}},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", fmt.Errorf("llm response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm http %d: %s", resp.StatusCode, clip(string(raw), 300))
	}
	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("llm response: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("llm error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("llm response has no choices")
	}
	msg := parsed.Choices[0].Message
	text := strings.TrimSpace(msg.Content)
	if text == "" {
		text = strings.TrimSpace(msg.ReasoningContent)
	}
	if text == "" {
		return "", fmt.Errorf("llm returned an empty answer (finish reason %q)", parsed.Choices[0].FinishReason)
	}
	return text, nil
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
