package llm

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// GeminiProvider implements Provider using Google's Gemini API
type GeminiProvider struct {
	client *genai.Client
	model  string
}

// NewGeminiProvider creates a new Gemini chat provider
func NewGeminiProvider(apiKey, model string) (*GeminiProvider, error) {
	ctx := context.Background()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	if model == "" {
		model = "gemini-1.5-flash"
	}

	return &GeminiProvider{
		client: client,
		model:  model,
	}, nil
}

// Complete generates a completion for the given prompt
func (p *GeminiProvider) Complete(ctx context.Context, prompt string) (string, error) {
	return p.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem generates a completion with a system prompt
func (p *GeminiProvider) CompleteWithSystem(ctx context.Context, system, prompt string) (string, error) {
	config := &genai.GenerateContentConfig{
		MaxOutputTokens: genai.Ptr(int32(1024)),
		Temperature:     genai.Ptr(float32(0.3)),
	}

	if system != "" {
		config.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{{Text: system}},
		}
	}

	result, err := p.client.Models.GenerateContent(ctx, p.model, []*genai.Content{
		{
			Role:  "user",
			Parts: []*genai.Part{{Text: prompt}},
		},
	}, config)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content generated")
	}

	return result.Candidates[0].Content.Parts[0].Text, nil
}

// Close releases resources
func (p *GeminiProvider) Close() error {
	return nil
}
