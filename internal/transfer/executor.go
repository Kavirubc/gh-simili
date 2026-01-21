package transfer

import (
	"context"
	"fmt"
	"strings"

	"github.com/kaviruhapuarachchi/gh-simili/internal/config"
	"github.com/kaviruhapuarachchi/gh-simili/internal/github"
	"github.com/kaviruhapuarachchi/gh-simili/internal/vectordb"
	"github.com/kaviruhapuarachchi/gh-simili/pkg/models"
)

// Executor handles issue transfers
type Executor struct {
	gh       *github.Client
	vectordb *vectordb.Client
	dryRun   bool
}

// NewExecutor creates a new transfer executor
func NewExecutor(gh *github.Client, vdb *vectordb.Client, dryRun bool) *Executor {
	return &Executor{
		gh:       gh,
		vectordb: vdb,
		dryRun:   dryRun,
	}
}

// Transfer executes an issue transfer to target repository
func (e *Executor) Transfer(ctx context.Context, issue *models.Issue, targetRepo string, rule *config.TransferRule) error {
	targetOrg, targetRepoName, err := github.ParseRepo(targetRepo)
	if err != nil {
		return err
	}

	// Check if target repo exists
	exists, err := e.gh.RepoExists(ctx, targetOrg, targetRepoName)
	if err != nil {
		return fmt.Errorf("failed to check target repo: %w", err)
	}
	if !exists {
		return fmt.Errorf("target repo %s does not exist", targetRepo)
	}

	// Check if already transferred
	transferred, err := e.gh.WasAlreadyTransferred(ctx, issue.Org, issue.Repo, issue.Number)
	if err != nil {
		return fmt.Errorf("failed to check transfer status: %w", err)
	}
	if transferred {
		return nil // Idempotent - already done
	}

	if e.dryRun {
		return nil
	}

	// Post pre-transfer comment
	comment := formatTransferComment(targetRepo, rule)
	if err := e.gh.PostComment(ctx, issue.Org, issue.Repo, issue.Number, comment); err != nil {
		return fmt.Errorf("failed to post transfer comment: %w", err)
	}

	// Execute transfer
	if err := e.gh.TransferIssue(ctx, issue.Org, issue.Repo, issue.Number, targetRepo); err != nil {
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
<sub>🤖 gh-simili Issue Intelligence</sub>`, targetRepo, matchDesc)
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
