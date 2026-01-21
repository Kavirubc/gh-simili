package embedding

import (
	"context"
	"fmt"
	"log"

	"github.com/kaviruhapuarachchi/gh-simili/internal/config"
)

// FallbackProvider wraps primary and fallback providers
type FallbackProvider struct {
	primary  Provider
	fallback Provider
}

// NewFallbackProvider creates a provider with primary and optional fallback
func NewFallbackProvider(cfg *config.EmbeddingConfig) (*FallbackProvider, error) {
	primary, err := createProvider(&cfg.Primary)
	if err != nil {
		return nil, fmt.Errorf("failed to create primary provider: %w", err)
	}

	var fallback Provider
	if cfg.Fallback.Provider != "" && cfg.Fallback.APIKey != "" {
		fallback, err = createProvider(&cfg.Fallback)
		if err != nil {
			log.Printf("Warning: failed to create fallback provider: %v", err)
		}
	}

	return &FallbackProvider{
		primary:  primary,
		fallback: fallback,
	}, nil
}

// createProvider creates a provider based on config
func createProvider(cfg *config.ProviderConfig) (Provider, error) {
	switch cfg.Provider {
	case "gemini":
		return NewGeminiProvider(cfg.APIKey, cfg.Model, cfg.Dimensions)
	case "openai":
		return NewOpenAIProvider(cfg.APIKey, cfg.Model, cfg.Dimensions)
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}

// Embed generates an embedding with fallback on failure
func (p *FallbackProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	embedding, err := p.primary.Embed(ctx, text)
	if err == nil {
		return embedding, nil
	}

	if p.fallback == nil {
		return nil, fmt.Errorf("primary embedding failed (no fallback): %w", err)
	}

	log.Printf("Primary embedding failed, trying fallback: %v", err)
	return p.fallback.Embed(ctx, text)
}

// EmbedBatch generates embeddings for multiple texts with fallback
func (p *FallbackProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings, err := p.primary.EmbedBatch(ctx, texts)
	if err == nil {
		return embeddings, nil
	}

	if p.fallback == nil {
		return nil, fmt.Errorf("primary embedding failed (no fallback): %w", err)
	}

	log.Printf("Primary batch embedding failed, trying fallback: %v", err)
	return p.fallback.EmbedBatch(ctx, texts)
}

// Close releases resources
func (p *FallbackProvider) Close() error {
	var errs []error
	if err := p.primary.Close(); err != nil {
		errs = append(errs, err)
	}
	if p.fallback != nil {
		if err := p.fallback.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
