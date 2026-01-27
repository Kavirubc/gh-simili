package github

import (
	"context"
	"fmt"
)

// Reaction represents a GitHub reaction
type Reaction struct {
	Content string `json:"content"` // "+1", "-1", "laugh", "confused", "heart", "hooray", "rocket", "eyes"
	User    User   `json:"user"`
}

// ListCommentReactions fetches reactions on a comment with pagination
func (c *Client) ListCommentReactions(ctx context.Context, org, repo string, commentID int) ([]Reaction, error) {
	var allReactions []Reaction
	page := 1
	perPage := 100

	for {
		endpoint := fmt.Sprintf("repos/%s/%s/issues/comments/%d/reactions?per_page=%d&page=%d", org, repo, commentID, perPage, page)

		var reactions []Reaction
		if err := c.rest.Get(endpoint, &reactions); err != nil {
			return nil, fmt.Errorf("failed to list comment reactions: %w", err)
		}

		if len(reactions) == 0 {
			break
		}

		allReactions = append(allReactions, reactions...)

		if len(reactions) < perPage {
			break
		}
		page++
	}

	return allReactions, nil
}

// HasReaction checks if a comment has a specific reaction type from any user
func (c *Client) HasReaction(ctx context.Context, org, repo string, commentID int, reactionType string) (bool, error) {
	reactions, err := c.ListCommentReactions(ctx, org, repo, commentID)
	if err != nil {
		return false, err
	}

	for _, r := range reactions {
		if r.Content == reactionType {
			return true, nil
		}
	}

	return false, nil
}

// GetReactionUsers returns all users who reacted with a specific reaction type
func (c *Client) GetReactionUsers(ctx context.Context, org, repo string, commentID int, reactionType string) ([]string, error) {
	reactions, err := c.ListCommentReactions(ctx, org, repo, commentID)
	if err != nil {
		return nil, err
	}

	var users []string
	for _, r := range reactions {
		if r.Content == reactionType {
			users = append(users, r.User.Login)
		}
	}

	return users, nil
}

// CheckReactionDecision checks reactions and returns decision: "approve", "cancel", or "none"
// approveReaction is typically "+1" (thumbs up)
// cancelReaction is typically "-1" (thumbs down)
func (c *Client) CheckReactionDecision(ctx context.Context, org, repo string, commentID int, approveReaction, cancelReaction string) (string, error) {
	reactions, err := c.ListCommentReactions(ctx, org, repo, commentID)
	if err != nil {
		return "", err
	}

	hasApprove := false
	hasCancel := false

	for _, r := range reactions {
		if r.Content == approveReaction {
			hasApprove = true
		}
		if r.Content == cancelReaction {
			hasCancel = true
		}
	}

	// Cancel takes precedence
	if hasCancel {
		return "cancel", nil
	}
	if hasApprove {
		return "approve", nil
	}

	return "none", nil
}
