package embedding

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// GeminiProvider implements Provider using Google's Gemini API
type GeminiProvider struct {
	client     *genai.Client
	model      string
	dimensions int
}

// NewGeminiProvider creates a new Gemini embedding provider
func NewGeminiProvider(apiKey, model string, dimensions int) (*GeminiProvider, error) {
	ctx := context.Background()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	if model == "" {
		model = "gemini-embedding-001"
	}
	if dimensions == 0 {
		dimensions = 768
	}

	return &GeminiProvider{
		client:     client,
		model:      model,
		dimensions: dimensions,
	}, nil
}

// Embed generates an embedding for a single text
func (p *GeminiProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts
func (p *GeminiProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	contents := make([]*genai.Content, len(texts))
	for i, text := range texts {
		contents[i] = &genai.Content{
			Parts: []*genai.Part{
				{Text: text},
			},
		}
	}

	dims := int32(p.dimensions)
	result, err := p.client.Models.EmbedContent(ctx, p.model, contents, &genai.EmbedContentConfig{
		OutputDimensionality: &dims,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate embeddings: %w", err)
	}

	embeddings := make([][]float32, len(result.Embeddings))
	for i, emb := range result.Embeddings {
		embeddings[i] = emb.Values
	}

	return embeddings, nil
}

// Close releases resources
func (p *GeminiProvider) Close() error {
	return nil
}
