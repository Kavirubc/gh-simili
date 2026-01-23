package llm

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements Provider using OpenAI's API
type OpenAIProvider struct {
	client *openai.Client
	model  string
}

// NewOpenAIProvider creates a new OpenAI chat provider
func NewOpenAIProvider(apiKey, model string) (*OpenAIProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	client := openai.NewClient(apiKey)

	if model == "" {
		model = "gpt-4o-mini"
	}

	return &OpenAIProvider{
		client: client,
		model:  model,
	}, nil
}

// Complete generates a completion for the given prompt
func (p *OpenAIProvider) Complete(ctx context.Context, prompt string) (string, error) {
	return p.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem generates a completion with a system prompt
func (p *OpenAIProvider) CompleteWithSystem(ctx context.Context, system, prompt string) (string, error) {
	messages := []openai.ChatCompletionMessage{}

	if system != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: system,
		})
	}

	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: prompt,
	})

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       p.model,
		Messages:    messages,
		MaxTokens:   1024,
		Temperature: 0.3,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create chat completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned")
	}

	return resp.Choices[0].Message.Content, nil
}

// Close releases resources
func (p *OpenAIProvider) Close() error {
	return nil
}
