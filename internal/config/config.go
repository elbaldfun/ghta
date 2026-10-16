// Package config loads and validates runtime configuration from the environment.
// Missing or invalid required values cause startup to fail with the offending
// field named — the app never silently runs with a bad config.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port     int
	LogLevel string

	MongoURI string
	MongoDB  string

	// Bearer token guarding the admin surface (/internal/*). Empty means those
	// routes refuse every request: a misconfigured deploy fails closed.
	AdminToken string

	// AI
	AIProvider      string // "openai" | "deepseek"
	OpenAIAPIKey    string
	OpenAIModel     string
	LMStudioBaseURL string
	LMStudioModel   string
	LMStudioAPIKey  string // auth for a hosted OpenAI-compatible relay (e.g. grok); empty for local LM Studio

	// Embedding classification layer (skipped when EmbedModel is empty or the
	// provider has no credentials).
	EmbedModel        string
	EmbedSimThreshold float64
}

// Load reads .env (if present) then the environment, validates, and returns the
// config or an aggregated error listing every invalid field.
func Load() (*Config, error) {
	_ = godotenv.Load() // .env is optional; real env always wins

	var errs []string

	cfg := &Config{
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		MongoURI:        os.Getenv("MONGODB_URI"),
		MongoDB:         getEnv("MONGODB_DB", "github-trend"),
		AdminToken:      os.Getenv("ADMIN_API_TOKEN"),
		AIProvider:      getEnv("AI_PROVIDER", "openai"),
		OpenAIAPIKey:    os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:     getEnv("OPENAI_MODEL", "gpt-4o-mini"),
		LMStudioBaseURL: getEnv("LMSTUDIO_BASE_URL", "http://localhost:1234/v1"),
		LMStudioModel:   os.Getenv("LMSTUDIO_LOCAL_MODULE_NAME"),
		LMStudioAPIKey:  os.Getenv("LMSTUDIO_API_KEY"),
	}

	// PORT
	port, err := strconv.Atoi(getEnv("PORT", "3000"))
	if err != nil {
		errs = append(errs, "PORT must be a number")
	}
	cfg.Port = port

	// Embedding layer
	cfg.EmbedModel = getEnv("EMBED_MODEL", "text-embedding-3-small")
	thr, err := strconv.ParseFloat(getEnv("EMBED_SIM_THRESHOLD", "0.35"), 64)
	if err != nil || thr <= 0 || thr >= 1 {
		errs = append(errs, "EMBED_SIM_THRESHOLD must be a number between 0 and 1")
	}
	cfg.EmbedSimThreshold = thr

	// Required
	if cfg.MongoURI == "" {
		errs = append(errs, "MONGODB_URI is required")
	}

	// Enum
	switch cfg.AIProvider {
	case "openai", "deepseek":
	default:
		errs = append(errs, "AI_PROVIDER must be one of: openai, deepseek")
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("invalid configuration:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
