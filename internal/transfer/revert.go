package transfer

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/Kavirubc/gh-simili/internal/config"
	"github.com/Kavirubc/gh-simili/internal/github"
	"github.com/Kavirubc/gh-simili/pkg/models"
)

const (
	revertMetadataPattern = `<!-- simili-transfer-source: ({.*}) -->`
)

var revertMetadataRegex = regexp.MustCompile(`(?s)` + revertMetadataPattern)

// RevertManager handles reverting transfers
type RevertManager struct {
	gh  *github.Client
	cfg *config.Config
}

// NewRevertManager creates a new revert manager
func NewRevertManager(gh *github.Client, cfg *config.Config) *RevertManager {
	return &RevertManager{
		gh:  gh,
		cfg: cfg,
	}
}

// RevertAction represents a revert action to be taken
type RevertAction struct {
	SourceOrg  string
	SourceRepo string
	CommentID  int
}

// CheckForRevert checks if an issue should be reverted based on comments and reactions
func (m *RevertManager) CheckForRevert(ctx context.Context, issue *models.Issue) (*RevertAction, error) {
	if !m.cfg.Defaults.DelayedActions.Enabled || !m.cfg.Defaults.DelayedActions.OptimisticTransfers {
		return nil, nil
	}

	comments, err := m.gh.ListComments(ctx, issue.Org, issue.Repo, issue.Number)
	if err != nil {
		return nil, fmt.Errorf("failed to list comments: %w", err)
	}

	for _, comment := range comments {
		matches := revertMetadataRegex.FindStringSubmatch(comment.Body)
		if len(matches) < 2 {
			continue
		}

		var metadata TransferSourceMetadata
		if err := json.Unmarshal([]byte(matches[1]), &metadata); err != nil {
			continue
		}

		// Check for cancel reaction (which triggers revert in this context)
		hasRevert, err := m.gh.HasReaction(ctx, issue.Org, issue.Repo, comment.ID, m.cfg.Defaults.DelayedActions.CancelReaction)
		if err != nil {
			continue
		}

		if hasRevert {
			return &RevertAction{
				SourceOrg:  metadata.Org,
				SourceRepo: metadata.Repo,
				CommentID:  comment.ID,
			}, nil
		}
	}

	return nil, nil
}

// Revert executes the revert transfer
// Note: We reuse Executor logic but we need to handle the "reverse" transfer
func (m *RevertManager) Revert(ctx context.Context, issue *models.Issue, action *RevertAction, executor *Executor) error {
	targetRepo := fmt.Sprintf("%s/%s", action.SourceOrg, action.SourceRepo)

	// Post revert comment
	revertMsg := fmt.Sprintf("↩️ Reverting transfer. Moving issue back to **%s** based on user request.", targetRepo)
	if err := m.gh.PostComment(ctx, issue.Org, issue.Repo, issue.Number, revertMsg); err != nil {
		return fmt.Errorf("failed to post revert comment: %w", err)
	}

	// Execute transfer using the executor (which handles auth/client)
	// We pass nil rule as this is manual revert
	// We construct a temporary issue object for the executor
	if err := executor.executeTransfer(ctx, issue, targetRepo, nil); err != nil {
		return fmt.Errorf("failed to execute revert transfer: %w", err)
	}

	return nil
}
