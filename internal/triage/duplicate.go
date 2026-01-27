package triage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Kavirubc/gh-simili/internal/config"
	"github.com/Kavirubc/gh-simili/internal/github"
	"github.com/Kavirubc/gh-simili/internal/pending"
	"github.com/Kavirubc/gh-simili/internal/vectordb"
	"github.com/Kavirubc/gh-simili/pkg/models"
)

// DuplicateChecker handles duplicate issue detection
type DuplicateChecker struct {
	autoCloseThreshold float64
	requireConfirm     bool
	gh                 *github.Client
	pendingManager     *pending.Manager
	cfg                *config.Config
	dryRun             bool
}

// NewDuplicateChecker creates a new duplicate checker
func NewDuplicateChecker(cfg *config.DuplicateConfig) *DuplicateChecker {
	return &DuplicateChecker{
		autoCloseThreshold: cfg.AutoCloseThreshold,
		requireConfirm:     cfg.RequireConfirm,
	}
}

// NewDuplicateCheckerWithDelayedActions creates a duplicate checker with delayed action support
func NewDuplicateCheckerWithDelayedActions(cfg *config.DuplicateConfig, gh *github.Client, fullCfg *config.Config) *DuplicateChecker {
	return &DuplicateChecker{
		autoCloseThreshold: cfg.AutoCloseThreshold,
		requireConfirm:     cfg.RequireConfirm,
		gh:                 gh,
		pendingManager:     pending.NewManager(gh, fullCfg),
		cfg:                fullCfg,
		dryRun:             false,
	}
}

// NewDuplicateCheckerWithDelayedActionsAndDryRun creates a duplicate checker with delayed action support and dry run
func NewDuplicateCheckerWithDelayedActionsAndDryRun(cfg *config.DuplicateConfig, gh *github.Client, fullCfg *config.Config, dryRun bool) *DuplicateChecker {
	return &DuplicateChecker{
		autoCloseThreshold: cfg.AutoCloseThreshold,
		requireConfirm:     cfg.RequireConfirm,
		gh:                 gh,
		pendingManager:     pending.NewManager(gh, fullCfg),
		cfg:                fullCfg,
		dryRun:             dryRun,
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
			Type:   ActionAddLabel,
			Label:  "duplicate",
			Reason: fmt.Sprintf("%.0f%% similarity to #%d", result.Similarity*100, result.Original.Number),
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

// ScheduleClose schedules a delayed close action
func (d *DuplicateChecker) ScheduleClose(ctx context.Context, issue *models.Issue, result *DuplicateResult) error {
	if d.pendingManager == nil || d.cfg == nil {
		return fmt.Errorf("delayed actions not configured")
	}

	if !d.cfg.Defaults.DelayedActions.Enabled {
		return fmt.Errorf("delayed actions disabled")
	}

	if result.Original == nil {
		return fmt.Errorf("cannot schedule close: original issue is nil")
	}

	delayHours := d.cfg.Defaults.DelayedActions.DelayHours
	expiresAt := time.Now().Add(time.Duration(delayHours) * time.Hour)

	// Create pending action metadata
	action := &pending.PendingAction{
		Type:        pending.ActionTypeClose,
		Org:         issue.Org,
		Repo:        issue.Repo,
		IssueNumber: issue.Number,
		Target:      result.Original.URL,
		ScheduledAt: time.Now(),
		ExpiresAt:   expiresAt,
	}

	// Post warning comment
	comment, err := d.formatDelayedCloseComment(result, expiresAt, d.cfg.Defaults.DelayedActions, action)
	if err != nil {
		return fmt.Errorf("failed to format warning comment: %w", err)
	}
	commentID, err := d.gh.PostCommentWithID(ctx, issue.Org, issue.Repo, issue.Number, comment)
	if err != nil {
		return fmt.Errorf("failed to post warning comment: %w", err)
	}

	action.CommentID = commentID

	// Schedule the action
	return d.pendingManager.ScheduleClose(ctx, issue, result.Original.URL, commentID, delayHours)
}

// ProcessPendingClose processes a pending close action
func (d *DuplicateChecker) ProcessPendingClose(ctx context.Context, action *pending.PendingAction) error {
	if d.pendingManager == nil || d.cfg == nil {
		return fmt.Errorf("delayed actions not configured")
	}

	// Check if already closed
	issue, err := d.gh.GetIssue(ctx, action.Org, action.Repo, action.IssueNumber)
	if err != nil {
		return fmt.Errorf("failed to get issue: %w", err)
	}
	if issue.State == "closed" {
		// Already closed, just remove label
		return d.pendingManager.Cancel(ctx, action)
	}

	// Check reactions
	decision, err := d.gh.CheckReactionDecision(
		ctx,
		action.Org,
		action.Repo,
		action.CommentID,
		d.cfg.Defaults.DelayedActions.ApproveReaction,
		d.cfg.Defaults.DelayedActions.CancelReaction,
	)
	if err != nil {
		return fmt.Errorf("failed to check reactions: %w", err)
	}

	if d.dryRun {
		return nil
	}

	if decision == "cancel" {
		// User cancelled, add potential-duplicate label instead
		if err := d.pendingManager.Cancel(ctx, action); err != nil {
			return err
		}
		if err := d.gh.AddLabels(ctx, action.Org, action.Repo, action.IssueNumber, []string{"potential-duplicate"}); err != nil {
			return err
		}
		cancelComment := formatCloseCancelledComment()
		return d.gh.PostComment(ctx, action.Org, action.Repo, action.IssueNumber, cancelComment)
	}

	if decision == "approve" && d.cfg.Defaults.DelayedActions.ExecuteOnApprove {
		// User approved, close immediately
		return d.executeClose(ctx, action)
	}

	if action.IsExpired() {
		// Expired and no cancel reaction, close issue
		return d.executeClose(ctx, action)
	}

	return nil // Not expired yet
}

// executeClose performs the actual close
func (d *DuplicateChecker) executeClose(ctx context.Context, action *pending.PendingAction) error {
	if d.dryRun {
		return nil
	}

	// Add duplicate label
	if err := d.gh.AddLabels(ctx, action.Org, action.Repo, action.IssueNumber, []string{"duplicate"}); err != nil {
		return err
	}

	// Close issue
	if err := d.gh.CloseIssue(ctx, action.Org, action.Repo, action.IssueNumber, "not_planned"); err != nil {
		return err
	}

	// Remove pending label
	if err := d.pendingManager.Cancel(ctx, action); err != nil {
		fmt.Printf("Warning: failed to remove pending-close label from %s/%s#%d: %v\n", action.Org, action.Repo, action.IssueNumber, err)
	}

	return nil
}

// formatDelayedCloseComment creates a warning comment for delayed close
func (d *DuplicateChecker) formatDelayedCloseComment(result *DuplicateResult, expiresAt time.Time, cfg config.DelayedActionsConfig, action *pending.PendingAction) (string, error) {
	deadline := expiresAt.Format("2006-01-02 15:04 MST")

	metadata, err := pending.FormatPendingActionMetadata(action)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`⚠️ **This issue will be closed as a duplicate in %d hours**

**Original issue:** [#%d - %s](%s)
**Similarity:** %.0f%%

**React to this comment:**
- 👍 (%s) to approve and proceed with closing
- 👎 (%s) to cancel and add potential-duplicate label instead

**Deadline**: %s

If no reaction is provided, the issue will be closed automatically.

%s

---
<sub>🤖 Powered by [Simili](https://github.com/Kavirubc/gh-simili)</sub>`,
		cfg.DelayHours,
		result.Original.Number,
		result.Original.Title,
		result.Original.URL,
		result.Similarity*100,
		cfg.ApproveReaction,
		cfg.CancelReaction,
		deadline,
		metadata,
	), nil
}

// formatCloseCancelledComment creates a cancellation comment
func formatCloseCancelledComment() string {
	return `✅ Auto-close has been cancelled based on your reaction.

The issue will remain open and has been labeled as ` + "`potential-duplicate`" + ` for maintainer review.

---
<sub>🤖 Powered by [Simili](https://github.com/Kavirubc/gh-simili)</sub>`
}
