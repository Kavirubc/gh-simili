package transfer

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

// Executor handles issue transfers
type Executor struct {
	transferClient *github.Client // Client for transfer operations (may have elevated permissions)
	commentClient  *github.Client // Client for posting comments (bot identity)
	vectordb       *vectordb.Client
	pendingManager *pending.Manager
	cfg            *config.Config
	dryRun         bool
}

// NewExecutor creates a new transfer executor
// transferClient is used for the actual transfer operation (requires elevated permissions)
// commentClient is used for posting comments (can be a bot token for proper identity)
func NewExecutor(transferClient *github.Client, commentClient *github.Client, vdb *vectordb.Client, cfg *config.Config, dryRun bool) *Executor {
	return &Executor{
		transferClient: transferClient,
		commentClient:  commentClient,
		vectordb:       vdb,
		pendingManager: pending.NewManager(commentClient, cfg),
		cfg:            cfg,
		dryRun:         dryRun,
	}
}

// Transfer executes an issue transfer to target repository
// If delayed actions are enabled, schedules the transfer instead of executing immediately
func (e *Executor) Transfer(ctx context.Context, issue *models.Issue, targetRepo string, rule *config.TransferRule) error {
	targetOrg, targetRepoName, err := github.ParseRepo(targetRepo)
	if err != nil {
		return err
	}

	// Check if target repo exists (use transfer client as it may have broader access)
	exists, err := e.transferClient.RepoExists(ctx, targetOrg, targetRepoName)
	if err != nil {
		return fmt.Errorf("failed to check target repo: %w", err)
	}
	if !exists {
		return fmt.Errorf("target repo %s does not exist", targetRepo)
	}

	// Check if already transferred
	transferred, err := e.commentClient.WasAlreadyTransferred(ctx, issue.Org, issue.Repo, issue.Number)
	if err != nil {
		return fmt.Errorf("failed to check transfer status: %w", err)
	}
	if transferred {
		return nil // Idempotent - already done
	}

	// Check if delayed actions are enabled
	if e.cfg.Defaults.DelayedActions.Enabled {
		return e.ScheduleTransfer(ctx, issue, targetRepo, rule)
	}

	// Immediate transfer (original behavior)
	return e.executeTransfer(ctx, issue, targetRepo, rule)
}

// ScheduleTransfer schedules a delayed transfer
func (e *Executor) ScheduleTransfer(ctx context.Context, issue *models.Issue, targetRepo string, rule *config.TransferRule) error {
	if e.dryRun {
		return nil
	}

	delayHours := e.cfg.Defaults.DelayedActions.DelayHours
	expiresAt := time.Now().Add(time.Duration(delayHours) * time.Hour)

	// Create pending action metadata
	action := &pending.PendingAction{
		Type:        pending.ActionTypeTransfer,
		Org:         issue.Org,
		Repo:        issue.Repo,
		IssueNumber: issue.Number,
		Target:      targetRepo,
		ScheduledAt: time.Now(),
		ExpiresAt:   expiresAt,
	}

	// Post warning comment
	comment, err := formatDelayedTransferComment(targetRepo, rule, expiresAt, e.cfg.Defaults.DelayedActions, action)
	if err != nil {
		return fmt.Errorf("failed to format warning comment: %w", err)
	}
	commentID, err := e.commentClient.PostCommentWithID(ctx, issue.Org, issue.Repo, issue.Number, comment)
	if err != nil {
		return fmt.Errorf("failed to post warning comment: %w", err)
	}

	action.CommentID = commentID

	// Schedule the action
	return e.pendingManager.ScheduleTransfer(ctx, issue, targetRepo, commentID, delayHours)
}

// ProcessPendingTransfer processes a pending transfer action
func (e *Executor) ProcessPendingTransfer(ctx context.Context, action *pending.PendingAction) error {
	// Check if already transferred
	transferred, err := e.commentClient.WasAlreadyTransferred(ctx, action.Org, action.Repo, action.IssueNumber)
	if err != nil {
		return fmt.Errorf("failed to check transfer status: %w", err)
	}
	if transferred {
		// Already transferred, just remove label
		return e.pendingManager.Cancel(ctx, action)
	}

	// Check reactions
	decision, err := e.commentClient.CheckReactionDecision(
		ctx,
		action.Org,
		action.Repo,
		action.CommentID,
		e.cfg.Defaults.DelayedActions.ApproveReaction,
		e.cfg.Defaults.DelayedActions.CancelReaction,
	)
	if err != nil {
		return fmt.Errorf("failed to check reactions: %w", err)
	}

	if decision == "cancel" {
		// User cancelled, remove label and post cancellation comment
		if err := e.pendingManager.Cancel(ctx, action); err != nil {
			return err
		}
		cancelComment := formatTransferCancelledComment(action.Target)
		return e.commentClient.PostComment(ctx, action.Org, action.Repo, action.IssueNumber, cancelComment)
	}

	if decision == "approve" && e.cfg.Defaults.DelayedActions.ExecuteOnApprove {
		// User approved, execute immediately
		issue := &models.Issue{
			Org:    action.Org,
			Repo:   action.Repo,
			Number: action.IssueNumber,
		}
		return e.executeTransfer(ctx, issue, action.Target, nil)
	}

	if action.IsExpired() {
		// Expired and no cancel reaction, execute transfer
		issue := &models.Issue{
			Org:    action.Org,
			Repo:   action.Repo,
			Number: action.IssueNumber,
		}
		return e.executeTransfer(ctx, issue, action.Target, nil)
	}

	return nil // Not expired yet
}

// executeTransfer performs the actual transfer
func (e *Executor) executeTransfer(ctx context.Context, issue *models.Issue, targetRepo string, rule *config.TransferRule) error {
	if e.dryRun {
		return nil
	}

	// Post transfer comment
	comment := formatTransferComment(targetRepo, rule)
	if err := e.commentClient.PostComment(ctx, issue.Org, issue.Repo, issue.Number, comment); err != nil {
		return fmt.Errorf("failed to post transfer comment: %w", err)
	}

	// Execute transfer
	if err := e.transferClient.TransferIssue(ctx, issue.Org, issue.Repo, issue.Number, targetRepo); err != nil {
		return fmt.Errorf("failed to transfer issue: %w", err)
	}

	// Remove pending label if exists
	if err := e.commentClient.RemoveLabel(ctx, issue.Org, issue.Repo, issue.Number, pending.LabelPendingTransfer); err != nil {
		fmt.Printf("Warning: failed to remove pending-transfer label from %s/%s#%d: %v\n", issue.Org, issue.Repo, issue.Number, err)
	}

	// Delete old vector
	collection := vectordb.CollectionName(issue.Org)
	if err := e.vectordb.Delete(ctx, collection, issue.UUID()); err != nil {
		fmt.Printf("Warning: failed to delete old vector: %v\n", err)
	}

	return nil
}

// formatTransferComment creates the transfer notification comment
func formatTransferComment(targetRepo string, rule *config.TransferRule) string {
	matchDesc := formatMatchDescription(rule)

	return fmt.Sprintf(`🚚 This issue has been automatically transferred to **%s** because it matches our routing rules.

**Matched rule:** %s

The discussion will continue there. Thanks for your report!

---
<sub>🤖 Powered by [Simili](https://github.com/Kavirubc/gh-simili)</sub>`, targetRepo, matchDesc)
}

// formatDelayedTransferComment creates a warning comment for delayed transfer
func formatDelayedTransferComment(targetRepo string, rule *config.TransferRule, expiresAt time.Time, cfg config.DelayedActionsConfig, action *pending.PendingAction) (string, error) {
	matchDesc := formatMatchDescription(rule)
	deadline := expiresAt.Format("2006-01-02 15:04 MST")

	metadata, err := pending.FormatPendingActionMetadata(action)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`⚠️ **This issue will be transferred to %s in %d hours**

**Matched rule:** %s

**React to this comment:**
- 👍 (%s) to approve and proceed with this transfer
- 👎 (%s) to cancel this transfer

**Deadline**: %s

If no reaction is provided, the transfer will proceed automatically.

%s

---
<sub>🤖 Powered by [Simili](https://github.com/Kavirubc/gh-simili)</sub>`,
		targetRepo,
		cfg.DelayHours,
		matchDesc,
		cfg.ApproveReaction,
		cfg.CancelReaction,
		deadline,
		metadata,
	), nil
}

// formatTransferCancelledComment creates a cancellation comment
func formatTransferCancelledComment(targetRepo string) string {
	return fmt.Sprintf(`✅ Transfer to **%s** has been cancelled based on your reaction.

The issue will remain in this repository.

---
<sub>🤖 Powered by [Simili](https://github.com/Kavirubc/gh-simili)</sub>`, targetRepo)
}

// formatMatchDescription creates a human-readable match description
func formatMatchDescription(rule *config.TransferRule) string {
	if rule == nil {
		return "routing rules"
	}

	var parts []string

	if len(rule.Match.Labels) > 0 {
		parts = append(parts, fmt.Sprintf("`labels: [%s]`", strings.Join(rule.Match.Labels, ", ")))
	}
	if len(rule.Match.TitleContains) > 0 {
		parts = append(parts, fmt.Sprintf("`title_contains: [%s]`", strings.Join(rule.Match.TitleContains, ", ")))
	}
	if len(rule.Match.BodyContains) > 0 {
		parts = append(parts, fmt.Sprintf("`body_contains: [%s]`", strings.Join(rule.Match.BodyContains, ", ")))
	}
	if rule.Match.Author != "" {
		parts = append(parts, fmt.Sprintf("`author: %s`", rule.Match.Author))
	}

	if len(parts) == 0 {
		return "routing rules"
	}
	return strings.Join(parts, " + ")
}
