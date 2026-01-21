package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const botSignature = "gh-simili Issue Intelligence"

// ListComments fetches comments on an issue
func (c *Client) ListComments(ctx context.Context, org, repo string, number int) ([]Comment, error) {
	endpoint := fmt.Sprintf("repos/%s/%s/issues/%d/comments", org, repo, number)

	var comments []Comment
	if err := c.rest.Get(endpoint, &comments); err != nil {
		return nil, fmt.Errorf("failed to list comments: %w", err)
	}

	return comments, nil
}

// PostComment adds a comment to an issue
func (c *Client) PostComment(ctx context.Context, org, repo string, number int, body string) error {
	endpoint := fmt.Sprintf("repos/%s/%s/issues/%d/comments", org, repo, number)

	payload := map[string]string{"body": body}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if err := c.rest.Post(endpoint, bytes.NewReader(jsonBody), nil); err != nil {
		return fmt.Errorf("failed to post comment: %w", err)
	}

	return nil
}

// ShouldSkipComment checks if bot recently commented (within cooldown period)
func (c *Client) ShouldSkipComment(ctx context.Context, org, repo string, number int, cooldownHours int) (bool, error) {
	comments, err := c.ListComments(ctx, org, repo, number)
	if err != nil {
		return false, err
	}

	cutoff := time.Now().Add(-time.Duration(cooldownHours) * time.Hour)

	for _, comment := range comments {
		if strings.Contains(comment.Body, botSignature) && comment.CreatedAt.After(cutoff) {
			return true, nil
		}
	}

	return false, nil
}

// WasAlreadyTransferred checks if issue was already transferred by bot
func (c *Client) WasAlreadyTransferred(ctx context.Context, org, repo string, number int) (bool, error) {
	comments, err := c.ListComments(ctx, org, repo, number)
	if err != nil {
		return false, err
	}

	for _, comment := range comments {
		if strings.Contains(comment.Body, "automatically transferred to") {
			return true, nil
		}
	}

	return false, nil
}
