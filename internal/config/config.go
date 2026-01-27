package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the full application configuration
type Config struct {
	Qdrant       QdrantConfig       `yaml:"qdrant"`
	Embedding    EmbeddingConfig    `yaml:"embedding"`
	Triage       TriageConfig       `yaml:"triage"`
	Defaults     DefaultsConfig     `yaml:"defaults"`
	Repositories []RepositoryConfig `yaml:"repositories"`
	RateLimits   RateLimitsConfig   `yaml:"rate_limits"`
}

// TriageConfig contains issue triage settings
type TriageConfig struct {
	Enabled    bool             `yaml:"enabled"`
	LLM        LLMConfig        `yaml:"llm"`
	Classifier ClassifierConfig `yaml:"classifier"`
	Quality    QualityConfig    `yaml:"quality"`
	Duplicate  DuplicateConfig  `yaml:"duplicate"`
}

// LLMConfig contains LLM provider settings for triage
type LLMConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	APIKey   string `yaml:"api_key"`
}

// ClassifierConfig contains label classification settings
type ClassifierConfig struct {
	Enabled       bool          `yaml:"enabled"`
	Labels        []LabelConfig `yaml:"labels"`
	MinConfidence float64       `yaml:"min_confidence"`
}

// LabelConfig defines a label with optional matching keywords
type LabelConfig struct {
	Name     string   `yaml:"name"`
	Keywords []string `yaml:"keywords,omitempty"`
}

// QualityConfig contains quality detection settings
type QualityConfig struct {
	Enabled        bool    `yaml:"enabled"`
	MinScore       float64 `yaml:"min_score"`
	NeedsInfoLabel string  `yaml:"needs_info_label"`
}

// DuplicateConfig contains duplicate detection settings
type DuplicateConfig struct {
	Enabled            bool    `yaml:"enabled"`
	AutoCloseThreshold float64 `yaml:"auto_close_threshold"`
	RequireConfirm     bool    `yaml:"require_confirmation"`
}

// QdrantConfig contains Qdrant connection settings
type QdrantConfig struct {
	URL     string `yaml:"url"`
	APIKey  string `yaml:"api_key"`
	UseGRPC bool   `yaml:"use_grpc"`
}

// EmbeddingConfig contains embedding provider settings
type EmbeddingConfig struct {
	Primary  ProviderConfig `yaml:"primary"`
	Fallback ProviderConfig `yaml:"fallback"`
}

// ProviderConfig contains settings for an embedding provider
type ProviderConfig struct {
	Provider   string `yaml:"provider"`
	Model      string `yaml:"model"`
	APIKey     string `yaml:"api_key"`
	Dimensions int    `yaml:"dimensions"`
}

// DefaultsConfig contains default behavior settings
type DefaultsConfig struct {
	SimilarityThreshold  float64              `yaml:"similarity_threshold"`
	MaxSimilarToShow     int                  `yaml:"max_similar_to_show"`
	IncludeClosedIssues  bool                 `yaml:"include_closed_issues"`
	ClosedIssueWeight    float64              `yaml:"closed_issue_weight"`
	CrossRepoSearch      bool                 `yaml:"cross_repo_search"`
	CommentCooldownHours int                  `yaml:"comment_cooldown_hours"`
	DelayedActions       DelayedActionsConfig `yaml:"delayed_actions"`
}

// DelayedActionsConfig contains settings for delayed actions
type DelayedActionsConfig struct {
	Enabled          bool   `yaml:"enabled"`
	DelayHours       int    `yaml:"delay_hours"`
	ApproveReaction  string `yaml:"approve_reaction"`
	CancelReaction   string `yaml:"cancel_reaction"`
	ExecuteOnApprove bool   `yaml:"execute_on_approve"`
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
	Target   string         `yaml:"target"`
	Priority int            `yaml:"priority"`
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

// Load reads and parses config from the given path
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	expandConfigEnvVars(&cfg)
	applyDefaults(&cfg)

	return &cfg, nil
}

// FindConfigPath looks for config in common locations
func FindConfigPath(explicit string) string {
	if explicit != "" {
		return explicit
	}

	// Check common locations
	paths := []string{
		".github/simili.yaml",
		".github/simili.yml",
		"simili.yaml",
		"simili.yml",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Check home directory
	if home, err := os.UserHomeDir(); err == nil {
		homePath := filepath.Join(home, ".config", "gh-simili", "config.yaml")
		if _, err := os.Stat(homePath); err == nil {
			return homePath
		}
	}

	return ""
}

// applyDefaults sets default values for unset fields
func applyDefaults(cfg *Config) {
	if cfg.Defaults.SimilarityThreshold == 0 {
		cfg.Defaults.SimilarityThreshold = 0.82
	}
	if cfg.Defaults.MaxSimilarToShow == 0 {
		cfg.Defaults.MaxSimilarToShow = 5
	}
	if cfg.Defaults.ClosedIssueWeight == 0 {
		cfg.Defaults.ClosedIssueWeight = 0.9
	}
	if cfg.Defaults.CommentCooldownHours == 0 {
		cfg.Defaults.CommentCooldownHours = 1
	}
	if cfg.RateLimits.GitHubRPS == 0 {
		cfg.RateLimits.GitHubRPS = 10
	}
	if cfg.RateLimits.EmbeddingRPS == 0 {
		cfg.RateLimits.EmbeddingRPS = 5
	}
	if cfg.RateLimits.QdrantRPS == 0 {
		cfg.RateLimits.QdrantRPS = 50
	}
	if cfg.Embedding.Primary.Dimensions == 0 {
		cfg.Embedding.Primary.Dimensions = 768
	}
	if cfg.Embedding.Fallback.Dimensions == 0 {
		cfg.Embedding.Fallback.Dimensions = 768
	}

	// Triage defaults
	if cfg.Triage.Classifier.MinConfidence == 0 {
		cfg.Triage.Classifier.MinConfidence = 0.7
	}
	if cfg.Triage.Quality.MinScore == 0 {
		cfg.Triage.Quality.MinScore = 0.5
	}
	if cfg.Triage.Quality.NeedsInfoLabel == "" {
		cfg.Triage.Quality.NeedsInfoLabel = "needs-info"
	}
	if cfg.Triage.Duplicate.AutoCloseThreshold == 0 {
		cfg.Triage.Duplicate.AutoCloseThreshold = 0.95
	}

	// Delayed actions defaults
	if cfg.Defaults.DelayedActions.DelayHours == 0 {
		cfg.Defaults.DelayedActions.DelayHours = 24
	}
	if cfg.Defaults.DelayedActions.ApproveReaction == "" {
		cfg.Defaults.DelayedActions.ApproveReaction = "+1"
	}
	if cfg.Defaults.DelayedActions.CancelReaction == "" {
		cfg.Defaults.DelayedActions.CancelReaction = "-1"
	}
	// Enabled defaults to false (zero value) - must be explicitly enabled
}
