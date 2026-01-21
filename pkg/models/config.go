package models

// Config represents the full application configuration
type Config struct {
	Qdrant       QdrantConfig       `yaml:"qdrant"`
	Embedding    EmbeddingConfig    `yaml:"embedding"`
	Defaults     DefaultsConfig     `yaml:"defaults"`
	Repositories []RepositoryConfig `yaml:"repositories"`
	RateLimits   RateLimitsConfig   `yaml:"rate_limits"`
}

// QdrantConfig contains Qdrant connection settings
type QdrantConfig struct {
	URL    string `yaml:"url"`
	APIKey string `yaml:"api_key"`
	UseGRPC bool   `yaml:"use_grpc"`
}

// EmbeddingConfig contains embedding provider settings
type EmbeddingConfig struct {
	Primary  ProviderConfig `yaml:"primary"`
	Fallback ProviderConfig `yaml:"fallback"`
}

// ProviderConfig contains settings for an embedding provider
type ProviderConfig struct {
	Provider   string `yaml:"provider"` // "gemini" or "openai"
	Model      string `yaml:"model"`
	APIKey     string `yaml:"api_key"`
	Dimensions int    `yaml:"dimensions"`
}

// DefaultsConfig contains default behavior settings
type DefaultsConfig struct {
	SimilarityThreshold  float64 `yaml:"similarity_threshold"`
	MaxSimilarToShow     int     `yaml:"max_similar_to_show"`
	IncludeClosedIssues  bool    `yaml:"include_closed_issues"`
	ClosedIssueWeight    float64 `yaml:"closed_issue_weight"`
	CrossRepoSearch      bool    `yaml:"cross_repo_search"`
	CommentCooldownHours int     `yaml:"comment_cooldown_hours"`
}

// RepositoryConfig contains settings for a specific repository
type RepositoryConfig struct {
	Org                 string         `yaml:"org"`
	Repo                string         `yaml:"repo"`
	Enabled             bool           `yaml:"enabled"`
	SimilarityThreshold float64        `yaml:"similarity_threshold,omitempty"`
	TransferRules       []TransferRule `yaml:"transfer_rules,omitempty"`
}

// TransferRule defines when to transfer an issue to another repo
type TransferRule struct {
	Match    MatchCondition `yaml:"match"`
	Target   string         `yaml:"target"`   // "org/repo"
	Priority int            `yaml:"priority"` // Lower = higher priority
}

// MatchCondition defines conditions for matching issues
type MatchCondition struct {
	Labels        []string `yaml:"labels,omitempty"`
	TitleContains []string `yaml:"title_contains,omitempty"`
	BodyContains  []string `yaml:"body_contains,omitempty"`
	Author        string   `yaml:"author,omitempty"`
}

// RateLimitsConfig contains rate limiting settings
type RateLimitsConfig struct {
	GitHubRPS    int `yaml:"github_requests_per_second"`
	EmbeddingRPS int `yaml:"embedding_requests_per_second"`
	QdrantRPS    int `yaml:"qdrant_requests_per_second"`
}
