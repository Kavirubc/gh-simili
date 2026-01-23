package triage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Kavirubc/gh-simili/internal/config"
	"github.com/Kavirubc/gh-simili/internal/llm"
	"github.com/Kavirubc/gh-simili/pkg/models"
)

// QualityChecker assesses issue quality
type QualityChecker struct {
	llm            llm.Provider
	minScore       float64
	needsInfoLabel string
}

// NewQualityChecker creates a new quality checker
func NewQualityChecker(provider llm.Provider, cfg *config.QualityConfig) *QualityChecker {
	return &QualityChecker{
		llm:            provider,
		minScore:       cfg.MinScore,
		needsInfoLabel: cfg.NeedsInfoLabel,
	}
}

// Check assesses the quality of an issue
func (q *QualityChecker) Check(ctx context.Context, issue *models.Issue) (*QualityResult, error) {
	// Basic checks first
	basicResult := q.basicQualityCheck(issue)

	// Use LLM for deeper analysis
	llmResult, err := q.llmQualityCheck(ctx, issue)
	if err != nil {
		// Return basic result on LLM failure
		return basicResult, nil
	}

	return q.mergeResults(basicResult, llmResult), nil
}

// basicQualityCheck performs rule-based quality assessment
func (q *QualityChecker) basicQualityCheck(issue *models.Issue) *QualityResult {
	result := &QualityResult{
		Score:   1.0,
		Missing: []string{},
	}

	// Check for minimum body length
	bodyLen := len(strings.TrimSpace(issue.Body))
	if bodyLen < 50 {
		result.Score -= 0.3
		result.Missing = append(result.Missing, "detailed description")
	}

	// Check for title quality
	titleLen := len(strings.TrimSpace(issue.Title))
	if titleLen < 10 {
		result.Score -= 0.2
		result.Missing = append(result.Missing, "descriptive title")
	}

	// Common quality indicators
	bodyLower := strings.ToLower(issue.Body)

	// Check for reproduction steps (for bugs)
	if containsAny(bodyLower, []string{"bug", "error", "crash", "broken", "not working"}) {
		if !containsAny(bodyLower, []string{"steps to reproduce", "reproduction", "to reproduce", "how to reproduce"}) {
			result.Score -= 0.2
			result.Missing = append(result.Missing, "reproduction steps")
		}
	}

	// Normalize score
	if result.Score < 0 {
		result.Score = 0
	}

	return result
}

// llmQualityCheck uses LLM for quality assessment
func (q *QualityChecker) llmQualityCheck(ctx context.Context, issue *models.Issue) (*QualityResult, error) {
	system := `You are an issue quality assessor. Analyze the GitHub issue and assess its quality.
Respond with JSON containing:
- "score": 0-1 quality score
- "missing": array of missing information (e.g., "reproduction steps", "version info", "expected behavior")
- "feedback": constructive feedback message for the author

Be helpful and constructive. Focus on what would help maintainers understand and address the issue.`

	prompt := fmt.Sprintf(`Issue Title: %s

Issue Body:
%s

Existing Labels: %s

Assess this issue's quality. Return JSON only.`,
		issue.Title,
		truncateText(issue.Body, 2000),
		strings.Join(issue.Labels, ", "))

	response, err := q.llm.CompleteWithSystem(ctx, system, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM quality check failed: %w", err)
	}

	return q.parseQualityResponse(response)
}

// parseQualityResponse parses the LLM response
func (q *QualityChecker) parseQualityResponse(response string) (*QualityResult, error) {
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var result QualityResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Clamp score to valid range
	if result.Score < 0 {
		result.Score = 0
	}
	if result.Score > 1 {
		result.Score = 1
	}

	return &result, nil
}

// mergeResults combines basic and LLM results
func (q *QualityChecker) mergeResults(basic, llm *QualityResult) *QualityResult {
	// Average the scores
	score := (basic.Score + llm.Score) / 2

	// Combine missing items (deduplicated)
	missingSet := make(map[string]bool)
	for _, m := range basic.Missing {
		missingSet[m] = true
	}
	for _, m := range llm.Missing {
		missingSet[m] = true
	}

	var missing []string
	for m := range missingSet {
		missing = append(missing, m)
	}

	return &QualityResult{
		Score:    score,
		Missing:  missing,
		Feedback: llm.Feedback,
	}
}

// NeedsInfo returns true if the issue needs more information
func (q *QualityChecker) NeedsInfo(result *QualityResult) bool {
	return result.Score < q.minScore
}

// GetNeedsInfoLabel returns the label to apply for low quality issues
func (q *QualityChecker) GetNeedsInfoLabel() string {
	return q.needsInfoLabel
}

// containsAny checks if text contains any of the keywords
func containsAny(text string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}
