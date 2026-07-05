// Package provider abstracts the AI backend used for categorization. OpenAI and
// LM Studio/DeepSeek are both reached through one OpenAI-compatible client,
// selected by AI_PROVIDER.
package provider

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/elbaldfun/ghta/internal/config"
)

// Provider runs a chat completion constrained to JSON output and returns the raw
// assistant text (a JSON document).
type Provider interface {
	AnalyzeJSON(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// Embedder turns texts into vectors for similarity-based classification.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// NewEmbedder returns the configured embedder, or nil when no embedding backend
// is available — the classification pipeline then skips its embedding layer.
func NewEmbedder(cfg *config.Config, log *slog.Logger) Embedder {
	if cfg.EmbedModel == "" {
		return nil
	}
	switch cfg.AIProvider {
	case "deepseek":
		c := openai.DefaultConfig("not-needed")
		c.BaseURL = cfg.LMStudioBaseURL
		return &embedProvider{client: openai.NewClientWithConfig(c), model: cfg.EmbedModel, log: log}
	default:
		if cfg.OpenAIAPIKey == "" {
			return nil
		}
		return &embedProvider{client: openai.NewClient(cfg.OpenAIAPIKey), model: cfg.EmbedModel, log: log}
	}
}

type embedProvider struct {
	client *openai.Client
	model  string
	log    *slog.Logger
}

func (p *embedProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	resp, err := p.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Model: openai.EmbeddingModel(p.model),
		Input: texts,
	})
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	if len(resp.Data) != len(texts) {
		return nil, fmt.Errorf("embed: got %d vectors for %d texts", len(resp.Data), len(texts))
	}
	out := make([][]float32, len(texts))
	for _, d := range resp.Data {
		out[d.Index] = d.Embedding
	}
	return out, nil
}

// New builds the configured provider. DeepSeek/LM Studio point the OpenAI client
// at a local base URL; OpenAI uses the hosted API.
func New(cfg *config.Config, log *slog.Logger) Provider {
	switch cfg.AIProvider {
	case "deepseek":
		c := openai.DefaultConfig("not-needed") // local server ignores the key
		c.BaseURL = cfg.LMStudioBaseURL
		return &chatProvider{client: openai.NewClientWithConfig(c), model: cfg.LMStudioModel, log: log}
	default:
		return &chatProvider{client: openai.NewClient(cfg.OpenAIAPIKey), model: cfg.OpenAIModel, log: log}
	}
}

type chatProvider struct {
	client *openai.Client
	model  string
	log    *slog.Logger
}

const maxAttempts = 3

// AnalyzeJSON requests a JSON object response, removing the need to scrape prose
// or code fences from the reply.
func (p *chatProvider) AnalyzeJSON(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:       p.model,
			Temperature: 0.2,
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: userPrompt},
			},
		})
		if err == nil {
			if len(resp.Choices) == 0 {
				return "", fmt.Errorf("ai returned no choices")
			}
			return resp.Choices[0].Message.Content, nil
		}
		lastErr = err
		p.log.Warn("ai attempt failed", "attempt", attempt, "err", err)
		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			}
		}
	}
	return "", fmt.Errorf("ai failed after %d attempts: %w", maxAttempts, lastErr)
}
