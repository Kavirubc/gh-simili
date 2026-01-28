// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-01-28
// Last Modified: 2026-01-28

package steps

import (
	"fmt"
	"strings"

	"github.com/Kavirubc/gh-simili/internal/pending"
	"github.com/Kavirubc/gh-simili/internal/pipeline/core"
	"github.com/Kavirubc/gh-simili/internal/processor"
	"github.com/Kavirubc/gh-simili/internal/triage"
	"github.com/Kavirubc/gh-simili/internal/vectordb"
)

// ResponseBuilder constructs the unified comment body based on results.
type ResponseBuilder struct{}

// NewResponseBuilder creates a new response builder step
func NewResponseBuilder() *ResponseBuilder {
	return &ResponseBuilder{}
}

func (s *ResponseBuilder) Name() string {
	return "response_builder"
}

func (s *ResponseBuilder) Run(ctx *core.Context) error {
	// Logic ported from UnifiedProcessor.buildUnifiedComment
	comment := s.buildComment(ctx)
	ctx.CommentBody = comment
	return nil
}

func (s *ResponseBuilder) buildComment(ctx *core.Context) string {
	result := ctx.Result
	similarIssues := ctx.SimilarIssues
	issue := ctx.Issue

	if len(similarIssues) == 0 && result.TriageResult == nil && ctx.TransferTarget == "" {
		return ""
	}

	var sections []string

	// Header
	sections = append(sections, "## 🤖 Issue Intelligence Summary\n")
	sections = append(sections, "Thanks for opening this issue! Here's what I found:\n")

	// Similar issues section
	if len(similarIssues) > 0 {
		crossRepo := processor.HasCrossRepoResults(similarIssues, issue.Org, issue.Repo)
		sections = append(sections, s.formatSimilarIssuesSection(similarIssues, crossRepo))
	}

	// Triage results
	if result.TriageResult != nil {
		s.appendTriageSections(&sections, result.TriageResult)
	}

	// Transfer section
	if ctx.TransferTarget != "" && !(ctx.Config.Defaults.DelayedActions.Enabled && ctx.Config.Defaults.DelayedActions.OptimisticTransfers) {
		sections = append(sections, s.formatTransferSection(ctx, ctx.TransferTarget, ctx.Result.PendingAction))
	}

	// Footer
	footer := "\n---\n<sub>🤖 Powered by [Simili](https://github.com/Kavirubc/gh-simili)</sub>"
	if ctx.Result.PendingAction != nil {
		metadata, err := pending.FormatPendingActionMetadata(ctx.Result.PendingAction)
		if err == nil {
			footer = "\n\n" + metadata + footer
		}
	}
	sections = append(sections, footer)

	return strings.Join(sections, "\n\n")
}

func (s *ResponseBuilder) appendTriageSections(sections *[]string, triageResult *triage.Result) {
	// Labels section
	if len(triageResult.Labels) > 0 {
		var labelLines []string
		labelLines = append(labelLines, "### 🏷️ Suggested Labels")
		for _, l := range triageResult.Labels {
			labelLines = append(labelLines, fmt.Sprintf("- `%s` (%.0f%% confidence) - %s", l.Label, l.Confidence*100, l.Reason))
		}
		*sections = append(*sections, strings.Join(labelLines, "\n"))
	}

	// Quality section
	if triageResult.Quality != nil {
		qualityLine := fmt.Sprintf("### 📊 Quality Score: %.0f%%", triageResult.Quality.Score*100)
		if len(triageResult.Quality.Missing) > 0 {
			qualityLine += fmt.Sprintf("\n⚠️ Missing: %s", strings.Join(triageResult.Quality.Missing, ", "))
		} else {
			qualityLine += "\n✅ Issue is well-documented"
		}
		*sections = append(*sections, qualityLine)
	}

	// Duplicate section
	if triageResult.Duplicate != nil && triageResult.Duplicate.IsDuplicate {
		dupLine := fmt.Sprintf("### ⚠️ Potential Duplicate\nSimilarity: %.0f%%", triageResult.Duplicate.Similarity*100)
		if triageResult.Duplicate.Original != nil {
			dupLine += fmt.Sprintf("\nOriginal: [#%d - %s](%s)",
				triageResult.Duplicate.Original.Number,
				truncateString(triageResult.Duplicate.Original.Title, 50),
				triageResult.Duplicate.Original.URL)
		}
		*sections = append(*sections, dupLine)
	}
}

func (s *ResponseBuilder) formatSimilarIssuesSection(results []vectordb.SearchResult, crossRepo bool) string {
	if len(results) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("### 🔍 Related Issues\n\n")

	if crossRepo {
		sb.WriteString("| Issue | Repository | Similarity | Status |\n")
		sb.WriteString("|-------|------------|------------|--------|\n")
	} else {
		sb.WriteString("| Issue | Similarity | Status |\n")
		sb.WriteString("|-------|------------|--------|\n")
	}

	for _, r := range results {
		status := "🟢 Open"
		if r.Issue.State == "closed" {
			status = "🔴 Closed"
		}

		title := truncateString(r.Issue.Title, 50)
		link := fmt.Sprintf("[#%d - %s](%s)", r.Issue.Number, title, r.Issue.URL)
		similarity := fmt.Sprintf("%.0f%%", r.Score*100)

		if crossRepo {
			repo := fmt.Sprintf("%s/%s", r.Issue.Org, r.Issue.Repo)
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", link, repo, similarity, status))
		} else {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", link, similarity, status))
		}
	}

	sb.WriteString("\nIf any of these address your problem, please let us know!")
	return sb.String()
}

func (s *ResponseBuilder) formatTransferSection(ctx *core.Context, target string, action *pending.PendingAction) string {
	var sb strings.Builder
	sb.WriteString("### 🔄 Transfer Suggestion\n\n")
	sb.WriteString(fmt.Sprintf("This issue appears to belong in **%s**.\n\n", target))

	if ctx.Config.Defaults.DelayedActions.Enabled && action != nil {
		deadline := action.ExpiresAt.Format("2006-01-02 15:04 MST")
		delayHours := ctx.Config.Defaults.DelayedActions.DelayHours
		sb.WriteString(fmt.Sprintf("**This issue will be transferred in %d hours.**\n\n", delayHours))
		sb.WriteString("**React to this comment:**\n")
		sb.WriteString(fmt.Sprintf("- 👍 (%s) to approve and proceed with transfer\n", ctx.Config.Defaults.DelayedActions.ApproveReaction))
		sb.WriteString(fmt.Sprintf("- 👎 (%s) to cancel this transfer\n\n", ctx.Config.Defaults.DelayedActions.CancelReaction))
		sb.WriteString(fmt.Sprintf("**Deadline**: %s\n\n", deadline))
		sb.WriteString("If no reaction is provided, the transfer will proceed automatically.")
	} else {
		sb.WriteString("Transfer will be executed immediately.")
	}

	return sb.String()
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
