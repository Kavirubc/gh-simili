package embedding

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements Provider using OpenAI's API
type OpenAIProvider struct {
	client     *openai.Client
	model      openai.EmbeddingModel
	dimensions int
}

// NewOpenAIProvider creates a new OpenAI embedding provider
func NewOpenAIProvider(apiKey, model string, dimensions int) (*OpenAIProvider, error) {
	client := openai.NewClient(apiKey)

	embModel := openai.SmallEmbedding3
	if model != "" {
		embModel = openai.EmbeddingModel(model)
	}
	if dimensions == 0 {
		dimensions = 768
	}

	return &OpenAIProvider{
		client:     client,
		model:      embModel,
		dimensions: dimensions,
	}, nil
}

// Embed generates an embedding for a single text
func (p *OpenAIProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts
func (p *OpenAIProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	req := openai.EmbeddingRequest{
		Input:      texts,
		Model:      p.model,
		Dimensions: p.dimensions,
	}

	resp, err := p.client.CreateEmbeddings(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embeddings: %w", err)
	}

	embeddings := make([][]float32, len(resp.Data))
	for i, data := range resp.Data {
		embeddings[i] = data.Embedding
	}

	return embeddings, nil
}

// Close releases resources
func (p *OpenAIProvider) Close() error {
	return nil
}
