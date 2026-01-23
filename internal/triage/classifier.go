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

// Classifier handles issue label classification
type Classifier struct {
	llm           llm.Provider
	labels        []config.LabelConfig
	minConfidence float64
}

// NewClassifier creates a new label classifier
func NewClassifier(provider llm.Provider, cfg *config.ClassifierConfig) *Classifier {
	return &Classifier{
		llm:           provider,
		labels:        cfg.Labels,
		minConfidence: cfg.MinConfidence,
	}
}

// Classify analyzes an issue and suggests labels
func (c *Classifier) Classify(ctx context.Context, issue *models.Issue) ([]LabelResult, error) {
	// First try rule-based classification
	ruleResults := c.classifyByRules(issue)

	// Then use LLM for remaining labels
	llmResults, err := c.classifyByLLM(ctx, issue, ruleResults)
	if err != nil {
		// Fall back to rule-based only on LLM error
		return ruleResults, nil
	}

	return c.mergeResults(ruleResults, llmResults), nil
}

// classifyByRules applies keyword-based rules
func (c *Classifier) classifyByRules(issue *models.Issue) []LabelResult {
	var results []LabelResult
	text := strings.ToLower(issue.Title + " " + issue.Body)

	for _, label := range c.labels {
		if len(label.Keywords) == 0 {
			continue
		}

		matchCount := 0
		for _, kw := range label.Keywords {
			if strings.Contains(text, strings.ToLower(kw)) {
				matchCount++
			}
		}

		if matchCount > 0 {
			confidence := float64(matchCount) / float64(len(label.Keywords))
			if confidence > 1.0 {
				confidence = 1.0
			}
			results = append(results, LabelResult{
				Label:      label.Name,
				Confidence: confidence,
				Reason:     "keyword match",
			})
		}
	}

	return results
}

// classifyByLLM uses the LLM to classify labels
func (c *Classifier) classifyByLLM(ctx context.Context, issue *models.Issue, existingResults []LabelResult) ([]LabelResult, error) {
	// Build list of labels not yet classified by rules
	classifiedLabels := make(map[string]bool)
	for _, r := range existingResults {
		classifiedLabels[r.Label] = true
	}

	var labelsToClassify []string
	for _, label := range c.labels {
		if !classifiedLabels[label.Name] {
			labelsToClassify = append(labelsToClassify, label.Name)
		}
	}

	if len(labelsToClassify) == 0 {
		return nil, nil
	}

	system := `You are an issue classification assistant. Analyze the GitHub issue and determine which labels apply.
Respond with a JSON array of objects with "label", "confidence" (0-1), and "reason" fields.
Only include labels that are relevant. Be conservative - only assign labels you are confident about.`

	prompt := fmt.Sprintf(`Issue Title: %s

Issue Body:
%s

Available Labels: %s

Classify this issue. Return JSON array only, no other text.`,
		issue.Title,
		truncateText(issue.Body, 2000),
		strings.Join(labelsToClassify, ", "))

	response, err := c.llm.CompleteWithSystem(ctx, system, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM classification failed: %w", err)
	}

	return c.parseClassificationResponse(response, labelsToClassify)
}

// parseClassificationResponse parses the LLM response
func (c *Classifier) parseClassificationResponse(response string, validLabels []string) ([]LabelResult, error) {
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var results []LabelResult
	if err := json.Unmarshal([]byte(response), &results); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Filter to only valid labels
	validSet := make(map[string]bool)
	for _, l := range validLabels {
		validSet[l] = true
	}

	var filtered []LabelResult
	for _, r := range results {
		if validSet[r.Label] {
			r.Reason = "LLM classification"
			filtered = append(filtered, r)
		}
	}

	return filtered, nil
}

// mergeResults combines rule-based and LLM results
func (c *Classifier) mergeResults(ruleResults, llmResults []LabelResult) []LabelResult {
	resultMap := make(map[string]LabelResult)

	// LLM results first (lower priority)
	for _, r := range llmResults {
		resultMap[r.Label] = r
	}

	// Rule results override (higher priority for keyword matches)
	for _, r := range ruleResults {
		if existing, ok := resultMap[r.Label]; ok {
			// Take higher confidence
			if r.Confidence > existing.Confidence {
				resultMap[r.Label] = r
			}
		} else {
			resultMap[r.Label] = r
		}
	}

	var results []LabelResult
	for _, r := range resultMap {
		if r.Confidence >= c.minConfidence {
			results = append(results, r)
		}
	}

	return results
}

// truncateText limits text length
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}
