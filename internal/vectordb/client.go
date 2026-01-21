package vectordb

import (
	"fmt"
	"strings"

	"github.com/kaviruhapuarachchi/gh-simili/internal/config"
	"github.com/qdrant/go-client/qdrant"
)

// Client wraps Qdrant operations
type Client struct {
	qdrant *qdrant.Client
}

// NewClient creates a new Qdrant client
func NewClient(cfg *config.QdrantConfig) (*Client, error) {
	host, port := parseHostPort(cfg.URL)

	// Determine if TLS should be used (cloud.qdrant.io requires TLS)
	useTLS := strings.Contains(host, "qdrant.io") || strings.Contains(host, "qdrant.cloud")

	client, err := qdrant.NewClient(&qdrant.Config{
		Host:   host,
		Port:   port,
		APIKey: cfg.APIKey,
		UseTLS: useTLS,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Qdrant: %w", err)
	}

	return &Client{qdrant: client}, nil
}

// parseHostPort extracts host and port from URL string
func parseHostPort(url string) (string, int) {
	// Remove protocol prefix if present
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Check for port
	if idx := strings.LastIndex(url, ":"); idx != -1 {
		host := url[:idx]
		var port int
		_, _ = fmt.Sscanf(url[idx+1:], "%d", &port)
		if port == 0 {
			port = 6334
		}
		return host, port
	}

	return url, 6334
}

// Close closes the connection
func (c *Client) Close() error {
	if c.qdrant != nil {
		return c.qdrant.Close()
	}
	return nil
}

// CollectionName returns the collection name for an org
func CollectionName(org string) string {
	return fmt.Sprintf("%s_issues", org)
}
