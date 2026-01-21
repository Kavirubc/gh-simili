package transfer

import (
	"sort"
	"strings"

	"github.com/kaviruhapuarachchi/gh-simili/internal/config"
	"github.com/kaviruhapuarachchi/gh-simili/pkg/models"
)

// RuleMatcher evaluates transfer rules against issues
type RuleMatcher struct {
	rules []config.TransferRule
}

// NewRuleMatcher creates a matcher for a repository's transfer rules
func NewRuleMatcher(rules []config.TransferRule) *RuleMatcher {
	// Sort rules by priority (lower = higher priority)
	sorted := make([]config.TransferRule, len(rules))
	copy(sorted, rules)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})

	return &RuleMatcher{rules: sorted}
}

// Match finds the first matching rule for an issue
// Returns target repo and the matched rule, or empty string if no match
func (m *RuleMatcher) Match(issue *models.Issue) (string, *config.TransferRule) {
	for i := range m.rules {
		if m.matchesRule(issue, &m.rules[i]) {
			return m.rules[i].Target, &m.rules[i]
		}
	}
	return "", nil
}

// matchesRule checks if an issue matches a single rule
// Multiple conditions in same rule = AND logic
// Multiple values in same condition = OR logic
func (m *RuleMatcher) matchesRule(issue *models.Issue, rule *config.TransferRule) bool {
	cond := &rule.Match
	matchCount := 0
	condCount := 0

	// Check labels (OR logic within)
	if len(cond.Labels) > 0 {
		condCount++
		if m.matchesAnyLabel(issue.Labels, cond.Labels) {
			matchCount++
		}
	}

	// Check title contains (OR logic within)
	if len(cond.TitleContains) > 0 {
		condCount++
		if m.containsAny(issue.Title, cond.TitleContains) {
			matchCount++
		}
	}

	// Check body contains (OR logic within)
	if len(cond.BodyContains) > 0 {
		condCount++
		if m.containsAny(issue.Body, cond.BodyContains) {
			matchCount++
		}
	}

	// Check author (exact match)
	if cond.Author != "" {
		condCount++
		if strings.EqualFold(issue.Author, cond.Author) {
			matchCount++
		}
	}

	// AND logic: all conditions must match
	return condCount > 0 && matchCount == condCount
}

// matchesAnyLabel checks if any issue label matches any rule label
func (m *RuleMatcher) matchesAnyLabel(issueLabels, ruleLabels []string) bool {
	for _, il := range issueLabels {
		for _, rl := range ruleLabels {
			if strings.EqualFold(il, rl) {
				return true
			}
		}
	}
	return false
}

// containsAny checks if text contains any of the substrings (case-insensitive)
func (m *RuleMatcher) containsAny(text string, substrings []string) bool {
	lowerText := strings.ToLower(text)
	for _, sub := range substrings {
		if strings.Contains(lowerText, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}
