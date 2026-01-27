package pending

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/Kavirubc/gh-simili/internal/config"
	"github.com/Kavirubc/gh-simili/internal/github"
	"github.com/Kavirubc/gh-simili/pkg/models"
)

const (
	LabelPendingTransfer = "pending-transfer"
	LabelPendingClose    = "pending-close"
	metadataPattern      = `<!-- simili-pending-action: ({.*}) -->`
)

var metadataRegex = regexp.MustCompile(`(?s)` + metadataPattern)

// ActionType represents the type of pending action
type ActionType string

const (
	ActionTypeTransfer ActionType = "transfer"
	ActionTypeClose    ActionType = "close"
)

// PendingAction represents a scheduled action
type PendingAction struct {
	Type        ActionType        `json:"type"`
	Org         string            `json:"org"`
	Repo        string            `json:"repo"`
	IssueNumber int               `json:"issue_number"`
	Target      string            `json:"target"` // target repo for transfer, or original issue URL for close
	CommentID   int               `json:"comment_id"`
	ScheduledAt time.Time         `json:"scheduled_at"`
	ExpiresAt   time.Time         `json:"expires_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Manager handles pending actions
type Manager struct {
	gh  *github.Client
	cfg *config.Config
}

// NewManager creates a new pending action manager
func NewManager(gh *github.Client, cfg *config.Config) *Manager {
	return &Manager{
		gh:  gh,
		cfg: cfg,
	}
}

// ScheduleTransfer schedules a transfer action
func (m *Manager) ScheduleTransfer(ctx context.Context, issue *models.Issue, targetRepo string, commentID int, delayHours int) error {
	// Add label (metadata is already in comment)
	return m.gh.AddLabels(ctx, issue.Org, issue.Repo, issue.Number, []string{LabelPendingTransfer})
}

// ScheduleClose schedules a close action
func (m *Manager) ScheduleClose(ctx context.Context, issue *models.Issue, originalIssueURL string, commentID int, delayHours int) error {
	// Add label (metadata is already in comment)
	return m.gh.AddLabels(ctx, issue.Org, issue.Repo, issue.Number, []string{LabelPendingClose})
}

// FindPendingActions finds all pending actions for issues with pending labels
func (m *Manager) FindPendingActions(ctx context.Context, org, repo string) ([]*PendingAction, error) {
	var actions []*PendingAction

	// Find issues with pending-transfer label
	transferIssues, err := m.gh.ListIssuesByLabel(ctx, org, repo, LabelPendingTransfer)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending transfer issues: %w", err)
	}

	for _, issue := range transferIssues {
		action, err := m.extractPendingAction(ctx, issue, ActionTypeTransfer)
		if err == nil && action != nil {
			actions = append(actions, action)
		}
	}

	// Find issues with pending-close label
	closeIssues, err := m.gh.ListIssuesByLabel(ctx, org, repo, LabelPendingClose)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending close issues: %w", err)
	}

	for _, issue := range closeIssues {
		action, err := m.extractPendingAction(ctx, issue, ActionTypeClose)
		if err == nil && action != nil {
			actions = append(actions, action)
		}
	}

	return actions, nil
}

// extractPendingAction extracts pending action from issue comments
func (m *Manager) extractPendingAction(ctx context.Context, issue *models.Issue, actionType ActionType) (*PendingAction, error) {
	comments, err := m.gh.ListComments(ctx, issue.Org, issue.Repo, issue.Number)
	if err != nil {
		return nil, err
	}

	for _, comment := range comments {
		matches := metadataRegex.FindStringSubmatch(comment.Body)
		if len(matches) < 2 {
			continue
		}

		var action PendingAction
		if err := json.Unmarshal([]byte(matches[1]), &action); err != nil {
			continue
		}

		if action.Type == actionType && action.IssueNumber == issue.Number {
			action.Org = issue.Org
			action.Repo = issue.Repo
			return &action, nil
		}
	}

	return nil, fmt.Errorf("pending action not found")
}

// FormatPendingActionMetadata formats action metadata as HTML comment
func FormatPendingActionMetadata(action *PendingAction) (string, error) {
	data, err := json.Marshal(action)
	if err != nil {
		return "", fmt.Errorf("failed to marshal pending action: %w", err)
	}
	return fmt.Sprintf("<!-- simili-pending-action: %s -->", string(data)), nil
}

// ParsePendingActionMetadata parses action metadata from comment body
func ParsePendingActionMetadata(commentBody string) (*PendingAction, error) {
	matches := metadataRegex.FindStringSubmatch(commentBody)
	if len(matches) < 2 {
		return nil, fmt.Errorf("metadata not found")
	}

	var action PendingAction
	if err := json.Unmarshal([]byte(matches[1]), &action); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &action, nil
}

// IsExpired checks if action has expired
func (a *PendingAction) IsExpired() bool {
	return time.Now().After(a.ExpiresAt)
}

// Cancel removes pending label and cancels the action
func (m *Manager) Cancel(ctx context.Context, action *PendingAction) error {
	var label string
	switch action.Type {
	case ActionTypeTransfer:
		label = LabelPendingTransfer
	case ActionTypeClose:
		label = LabelPendingClose
	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}

	return m.gh.RemoveLabel(ctx, action.Org, action.Repo, action.IssueNumber, label)
}
