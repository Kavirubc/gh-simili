package llm

import (
	"context"
)

// Provider defines the interface for LLM chat completion
type Provider interface {
	Complete(ctx context.Context, prompt string) (string, error)
	CompleteWithSystem(ctx context.Context, system, prompt string) (string, error)
	Close() error
}

// Message represents a chat message
type Message struct {
	Role    string
	Content string
}

// CompletionRequest contains parameters for a completion request
type CompletionRequest struct {
	Messages    []Message
	MaxTokens   int
	Temperature float32
}
