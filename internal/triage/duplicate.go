package triage

import (
	"fmt"
	"strings"

	"github.com/Kavirubc/gh-simili/internal/config"
	"github.com/Kavirubc/gh-simili/internal/vectordb"
)

// DuplicateChecker handles duplicate issue detection
type DuplicateChecker struct {
	autoCloseThreshold float64
	requireConfirm     bool
}

// NewDuplicateChecker creates a new duplicate checker
func NewDuplicateChecker(cfg *config.DuplicateConfig) *DuplicateChecker {
	return &DuplicateChecker{
		autoCloseThreshold: cfg.AutoCloseThreshold,
		requireConfirm:     cfg.RequireConfirm,
	}
}

// Check analyzes similar issues to detect duplicates
func (d *DuplicateChecker) Check(similarIssues []vectordb.SearchResult) *DuplicateResult {
	if len(similarIssues) == 0 {
		return &DuplicateResult{
			IsDuplicate: false,
			Similarity:  0,
		}
	}

	// Find the highest similarity open issue
	var bestMatch *vectordb.SearchResult
	for i := range similarIssues {
		r := &similarIssues[i]
		if r.Issue.State == "open" && (bestMatch == nil || r.Score > bestMatch.Score) {
			bestMatch = r
		}
	}

	// If no open issues, check closed ones
	if bestMatch == nil {
		for i := range similarIssues {
			r := &similarIssues[i]
			if bestMatch == nil || r.Score > bestMatch.Score {
				bestMatch = r
			}
		}
	}

	if bestMatch == nil {
		return &DuplicateResult{
			IsDuplicate: false,
			Similarity:  0,
		}
	}

	isDuplicate := bestMatch.Score >= d.autoCloseThreshold
	shouldClose := isDuplicate && !d.requireConfirm

	return &DuplicateResult{
		IsDuplicate: isDuplicate,
		Similarity:  bestMatch.Score,
		Original:    &bestMatch.Issue,
		ShouldClose: shouldClose,
	}
}

// FormatDuplicateComment creates a comment for duplicate issues
func (d *DuplicateChecker) FormatDuplicateComment(result *DuplicateResult, autoClose bool) string {
	if result.Original == nil {
		return ""
	}

	var sb strings.Builder

	if autoClose {
		sb.WriteString("🔒 This issue has been automatically closed as a duplicate.\n\n")
	} else {
		sb.WriteString("⚠️ This issue appears to be a duplicate.\n\n")
	}

	sb.WriteString(fmt.Sprintf("**Original issue:** [#%d - %s](%s)\n",
		result.Original.Number,
		result.Original.Title,
		result.Original.URL))

	sb.WriteString(fmt.Sprintf("**Similarity:** %.0f%%\n\n", result.Similarity*100))

	if autoClose {
		sb.WriteString("If you believe this is not a duplicate, please comment and we will reopen it.\n\n")
	} else {
		sb.WriteString("Please review the linked issue. If it addresses your concern, ")
		sb.WriteString("consider closing this issue and following the original.\n\n")
	}

	sb.WriteString("---\n")
	sb.WriteString("<sub>🤖 Powered by [Simili](https://github.com/Kavirubc/gh-simili)</sub>")

	return sb.String()
}

// GetActions returns actions to take for a duplicate issue
func (d *DuplicateChecker) GetActions(result *DuplicateResult) []Action {
	if !result.IsDuplicate || result.Original == nil {
		return nil
	}

	actions := []Action{
		{
			Type:    ActionAddLabel,
			Label:   "duplicate",
			Reason:  fmt.Sprintf("%.0f%% similarity to #%d", result.Similarity*100, result.Original.Number),
		},
		{
			Type:    ActionComment,
			Comment: d.FormatDuplicateComment(result, result.ShouldClose),
			Reason:  "notify author of duplicate",
		},
	}

	if result.ShouldClose {
		actions = append(actions, Action{
			Type:   ActionClose,
			Reason: fmt.Sprintf("auto-close duplicate (%.0f%% similarity)", result.Similarity*100),
		})
	}

	return actions
}
