// Author: Kaviru Hapuarachchi
// GitHub: [https://github.com/kavirubc](https://github.com/kavirubc)
// Created: 2026-01-28
// Last Modified: 2026-01-28

package triage

import (
	"context"
	"strings"
	"testing"

	"github.com/Kavirubc/gh-simili/internal/config"
	"github.com/Kavirubc/gh-simili/pkg/models"
)

// mockLLM is a simple mock provider for testing
type mockLLM struct {
	response string
}

func (m *mockLLM) Complete(ctx context.Context, prompt string) (string, error) {
	return m.response, nil
}

func (m *mockLLM) CompleteWithSystem(ctx context.Context, system, prompt string) (string, error) {
	return m.response, nil
}

func (m *mockLLM) Close() error { return nil }

func TestRouter_Route(t *testing.T) {
	repos := []config.RepositoryConfig{
		{
			Org:         "similigh",
			Repo:        "nexusflow-core",
			Description: "Core engine logic",
			Enabled:     true,
		},
		{
			Org:         "similigh",
			Repo:        "nexusflow-docs",
			Description: "Documentation and guides",
			Enabled:     true,
		},
	}

	tests := []struct {
		name         string
		llmResponse  string
		issue        *models.Issue
		wantTarget   string
		wantReason   string
		wantConf     float64
	}{
		{
			name:        "routes to core",
			llmResponse: `{"target_repo": "similigh/nexusflow-core", "confidence": 0.95, "reason": "Server bug"}`,
			issue:       &models.Issue{Title: "Server crash", Body: "The server panics on startup"},
			wantTarget:  "similigh/nexusflow-core",
			wantReason:  "Server bug",
			wantConf:    0.95,
		},
		{
			name:        "routes to docs",
			llmResponse: `{"target_repo": "similigh/nexusflow-docs", "confidence": 0.9, "reason": "Typo in readme"}`,
			issue:       &models.Issue{Title: "Typo in docs", Body: "Page 3 has a typo"},
			wantTarget:  "similigh/nexusflow-docs",
			wantReason:  "Typo in readme",
			wantConf:    0.9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mockLLM{response: tt.llmResponse}
			router := NewRouter(m, repos)
			result, err := router.Route(context.Background(), tt.issue)
			if err != nil {
				t.Fatalf("Route() error = %v", err)
			}
			if result.TargetRepo != tt.wantTarget {
				t.Errorf("TargetRepo = %v, want %v", result.TargetRepo, tt.wantTarget)
			}
			if result.Reason != tt.wantReason {
				t.Errorf("Reason = %v, want %v", result.Reason, tt.wantReason)
			}
			if result.Confidence != tt.wantConf {
				t.Errorf("Confidence = %v, want %v", result.Confidence, tt.wantConf)
			}
		})
	}
}

func TestRouter_ParseRoutingResponse_EdgeCases(t *testing.T) {
	m := &mockLLM{}
	router := NewRouter(m, nil)

	tests := []struct {
		name     string
		response string
		wantErr  bool
	}{
		{
			name:     "wrapped in markdown",
			response: "```json\n{\"target_repo\": \"repo\", \"confidence\": 1.0, \"reason\": \"test\"}\n```",
			wantErr:  false,
		},
		{
			name:     "invalid json",
			response: "not json",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := router.parseRoutingResponse(tt.response)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRoutingResponse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
