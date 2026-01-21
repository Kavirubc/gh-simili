package config

import (
	"fmt"
	"strings"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Validate checks the configuration for errors
func Validate(cfg *Config) []error {
	var errs []error

	// Validate Qdrant config
	if cfg.Qdrant.URL == "" {
		errs = append(errs, ValidationError{"qdrant.url", "required"})
	}

	// Validate embedding config
	if cfg.Embedding.Primary.Provider == "" {
		errs = append(errs, ValidationError{"embedding.primary.provider", "required"})
	} else if cfg.Embedding.Primary.Provider != "gemini" && cfg.Embedding.Primary.Provider != "openai" {
		errs = append(errs, ValidationError{"embedding.primary.provider", "must be 'gemini' or 'openai'"})
	}

	if cfg.Embedding.Primary.APIKey == "" {
		errs = append(errs, ValidationError{"embedding.primary.api_key", "required"})
	}

	// Validate defaults
	if cfg.Defaults.SimilarityThreshold < 0 || cfg.Defaults.SimilarityThreshold > 1 {
		errs = append(errs, ValidationError{"defaults.similarity_threshold", "must be between 0 and 1"})
	}

	if cfg.Defaults.ClosedIssueWeight < 0 || cfg.Defaults.ClosedIssueWeight > 1 {
		errs = append(errs, ValidationError{"defaults.closed_issue_weight", "must be between 0 and 1"})
	}

	// Validate repositories
	for i, repo := range cfg.Repositories {
		prefix := fmt.Sprintf("repositories[%d]", i)

		if repo.Org == "" {
			errs = append(errs, ValidationError{prefix + ".org", "required"})
		}
		if repo.Repo == "" {
			errs = append(errs, ValidationError{prefix + ".repo", "required"})
		}

		// Validate transfer rules
		for j, rule := range repo.TransferRules {
			rulePrefix := fmt.Sprintf("%s.transfer_rules[%d]", prefix, j)

			if rule.Target == "" {
				errs = append(errs, ValidationError{rulePrefix + ".target", "required"})
			} else if !strings.Contains(rule.Target, "/") {
				errs = append(errs, ValidationError{rulePrefix + ".target", "must be in format 'org/repo'"})
			}

			// At least one match condition required
			if len(rule.Match.Labels) == 0 &&
				len(rule.Match.TitleContains) == 0 &&
				len(rule.Match.BodyContains) == 0 &&
				rule.Match.Author == "" {
				errs = append(errs, ValidationError{rulePrefix + ".match", "at least one condition required"})
			}
		}
	}

	return errs
}

// GetRepoConfig returns config for a specific repository
func (cfg *Config) GetRepoConfig(org, repo string) *RepositoryConfig {
	for i := range cfg.Repositories {
		if cfg.Repositories[i].Org == org && cfg.Repositories[i].Repo == repo {
			return &cfg.Repositories[i]
		}
	}
	return nil
}

// GetSimilarityThreshold returns the threshold for a repo (or default)
func (cfg *Config) GetSimilarityThreshold(org, repo string) float64 {
	if rc := cfg.GetRepoConfig(org, repo); rc != nil && rc.SimilarityThreshold > 0 {
		return rc.SimilarityThreshold
	}
	return cfg.Defaults.SimilarityThreshold
}
