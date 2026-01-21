package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandEnvVars(t *testing.T) {
	os.Setenv("TEST_VAR", "test-value")
	defer os.Unsetenv("TEST_VAR")

	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "expands env var",
			input:  "${TEST_VAR}",
			expect: "test-value",
		},
		{
			name:   "keeps unset var",
			input:  "${UNSET_VAR}",
			expect: "${UNSET_VAR}",
		},
		{
			name:   "expands in string",
			input:  "https://${TEST_VAR}.example.com",
			expect: "https://test-value.example.com",
		},
		{
			name:   "no vars",
			input:  "plain string",
			expect: "plain string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandEnvVars(tt.input)
			if result != tt.expect {
				t.Errorf("expandEnvVars(%q) = %q, want %q", tt.input, result, tt.expect)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	content := `
qdrant:
  url: "http://localhost:6334"
  use_grpc: true

embedding:
  primary:
    provider: "gemini"
    model: "gemini-embedding-001"
    api_key: "test-key"
    dimensions: 768

repositories:
  - org: "testorg"
    repo: "testrepo"
    enabled: true
`

	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Qdrant.URL != "http://localhost:6334" {
		t.Errorf("Qdrant.URL = %v, want http://localhost:6334", cfg.Qdrant.URL)
	}

	if cfg.Embedding.Primary.Provider != "gemini" {
		t.Errorf("Embedding.Primary.Provider = %v, want gemini", cfg.Embedding.Primary.Provider)
	}

	if len(cfg.Repositories) != 1 {
		t.Errorf("len(Repositories) = %d, want 1", len(cfg.Repositories))
	}
}

func TestApplyDefaults(t *testing.T) {
	cfg := &Config{}
	applyDefaults(cfg)

	if cfg.Defaults.SimilarityThreshold != 0.82 {
		t.Errorf("SimilarityThreshold = %v, want 0.82", cfg.Defaults.SimilarityThreshold)
	}

	if cfg.Defaults.MaxSimilarToShow != 5 {
		t.Errorf("MaxSimilarToShow = %v, want 5", cfg.Defaults.MaxSimilarToShow)
	}

	if cfg.Defaults.ClosedIssueWeight != 0.9 {
		t.Errorf("ClosedIssueWeight = %v, want 0.9", cfg.Defaults.ClosedIssueWeight)
	}

	if cfg.RateLimits.GitHubRPS != 10 {
		t.Errorf("GitHubRPS = %v, want 10", cfg.RateLimits.GitHubRPS)
	}
}
