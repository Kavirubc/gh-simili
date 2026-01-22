package transfer

import (
	"context"
	"fmt"
	"strings"

	"github.com/Kavirubc/gh-simili/internal/config"
	"github.com/Kavirubc/gh-simili/internal/github"
	"github.com/Kavirubc/gh-simili/internal/vectordb"
	"github.com/Kavirubc/gh-simili/pkg/models"
)

// Executor handles issue transfers
type Executor struct {
	transferClient *github.Client // Client for transfer operations (may have elevated permissions)
	commentClient  *github.Client // Client for posting comments (bot identity)
	vectordb       *vectordb.Client
	dryRun         bool
}

// NewExecutor creates a new transfer executor
// transferClient is used for the actual transfer operation (requires elevated permissions)
// commentClient is used for posting comments (can be a bot token for proper identity)
func NewExecutor(transferClient *github.Client, commentClient *github.Client, vdb *vectordb.Client, dryRun bool) *Executor {
	return &Executor{
		transferClient: transferClient,
		commentClient:  commentClient,
		vectordb:       vdb,
		dryRun:         dryRun,
	}
}

// Transfer executes an issue transfer to target repository
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

	if e.dryRun {
		return nil
	}

	// Post pre-transfer comment using comment client (for bot identity)
	comment := formatTransferComment(targetRepo, rule)
	if err := e.commentClient.PostComment(ctx, issue.Org, issue.Repo, issue.Number, comment); err != nil {
		return fmt.Errorf("failed to post transfer comment: %w", err)
	}

	// Execute transfer using transfer client (requires elevated permissions)
	if err := e.transferClient.TransferIssue(ctx, issue.Org, issue.Repo, issue.Number, targetRepo); err != nil {
		return fmt.Errorf("failed to transfer issue: %w", err)
	}

	// Delete old vector (will be re-indexed in new repo on next sync)
	collection := vectordb.CollectionName(issue.Org)
	if err := e.vectordb.Delete(ctx, collection, issue.UUID()); err != nil {
		// Log but don't fail - vector will be cleaned up eventually
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

// formatMatchDescription creates a human-readable match description
func formatMatchDescription(rule *config.TransferRule) string {
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
