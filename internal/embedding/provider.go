package embedding

import (
	"context"
	"fmt"
	"strings"
)

// Provider defines the interface for embedding generation
type Provider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Close() error
}

// PrepareIssueText combines title and body for embedding
func PrepareIssueText(title, body string) string {
	text := fmt.Sprintf("Title: %s\n\nBody: %s", title, body)

	// Truncate to ~6000 chars (~1500 tokens) to stay within limits
	if len(text) > 6000 {
		text = text[:6000] + "..."
	}

	return text
}

// TruncateText truncates text to maxLen characters
func TruncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

// CleanText removes excessive whitespace from text
func CleanText(text string) string {
	// Replace multiple newlines with double newline
	text = strings.TrimSpace(text)
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}
