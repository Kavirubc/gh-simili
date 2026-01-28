// Author: Kaviru Hapuarachchi
// GitHub: [https://github.com/kavirubc](https://github.com/kavirubc)
// Created: 2026-01-28
// Last Modified: 2026-01-28

package triage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/Kavirubc/gh-simili/internal/config"
	"github.com/Kavirubc/gh-simili/internal/llm"
	"github.com/Kavirubc/gh-simili/pkg/models"
)

// Router handles AI-based issue routing decisions
type Router struct {
	llm          llm.Provider
	repositories []config.RepositoryConfig
}

// RoutingResult represents the decision made by the AI router
type RoutingResult struct {
	TargetRepo string  `json:"target_repo"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

// NewRouter creates a new AI issue router
func NewRouter(provider llm.Provider, repos []config.RepositoryConfig) *Router {
	return &Router{
		llm:          provider,
		repositories: repos,
	}
}

// Route analyzes an issue and suggests which repository it belongs in
func (r *Router) Route(ctx context.Context, issue *models.Issue) (*RoutingResult, error) {
	// Build a map of potential destinations
	destinations := make([]string, 0)
	for _, repo := range r.repositories {
		if repo.Enabled && repo.Description != "" {
			destinations = append(destinations, fmt.Sprintf("- %s/%s: %s", repo.Org, repo.Repo, repo.Description))
		}
	}

	if len(destinations) == 0 {
		return nil, nil // No destinations with descriptions configured
	}

	system := `You are an expert GitHub issue router. Your task is to analyze the intent of an issue and decide which repository it belongs in.
Respond ONLY with a JSON object containing:
- "target_repo": The "org/repo" string of the destination, or the current repo if it belongs here.
- "confidence": A float from 0 to 1.
- "reason": A brief explanation of why this intent matches the repository description.

If the issue clearly belongs in its current repository, "target_repo" should match the current repo.`

	prompt := fmt.Sprintf(`Current Repository: %s/%s

Issue Title: %s

Issue Description:
%s

Available Repositories and their purposes:
%s

Analyze the issue intent. Does it belong in a different repository based on the descriptions? Return JSON only.`,
		issue.Org, issue.Repo,
		issue.Title,
		truncateText(issue.Body, 3000),
		strings.Join(destinations, "\n"))

	response, err := r.llm.CompleteWithSystem(ctx, system, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI routing completion failed: %w", err)
	}

	return r.parseRoutingResponse(response)
}

// parseRoutingResponse extracts the JSON result from LLM response
func (r *Router) parseRoutingResponse(response string) (*RoutingResult, error) {
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var result RoutingResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		log.Printf("Debug: Failed to parse LLM response: %s", response)
		return nil, fmt.Errorf("failed to parse AI routing response: %w", err)
	}

	return &result, nil
}
