package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// AddLabels adds labels to an issue
func (c *Client) AddLabels(ctx context.Context, org, repo string, number int, labels []string) error {
	if len(labels) == 0 {
		return nil
	}

	endpoint := fmt.Sprintf("repos/%s/%s/issues/%d/labels", org, repo, number)

	payload := map[string][]string{"labels": labels}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if err := c.rest.Post(endpoint, bytes.NewReader(jsonBody), nil); err != nil {
		return fmt.Errorf("failed to add labels: %w", err)
	}

	return nil
}

// RemoveLabel removes a label from an issue
func (c *Client) RemoveLabel(ctx context.Context, org, repo string, number int, label string) error {
	endpoint := fmt.Sprintf("repos/%s/%s/issues/%d/labels/%s", org, repo, number, label)

	if err := c.rest.Delete(endpoint, nil); err != nil {
		return fmt.Errorf("failed to remove label: %w", err)
	}

	return nil
}

// CloseIssue closes an issue with an optional reason
func (c *Client) CloseIssue(ctx context.Context, org, repo string, number int, reason string) error {
	endpoint := fmt.Sprintf("repos/%s/%s/issues/%d", org, repo, number)

	payload := map[string]string{"state": "closed"}
	if reason != "" {
		payload["state_reason"] = reason
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if err := c.rest.Patch(endpoint, bytes.NewReader(jsonBody), nil); err != nil {
		return fmt.Errorf("failed to close issue: %w", err)
	}

	return nil
}

// ReopenIssue reopens a closed issue
func (c *Client) ReopenIssue(ctx context.Context, org, repo string, number int) error {
	endpoint := fmt.Sprintf("repos/%s/%s/issues/%d", org, repo, number)

	payload := map[string]string{"state": "open"}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if err := c.rest.Patch(endpoint, bytes.NewReader(jsonBody), nil); err != nil {
		return fmt.Errorf("failed to reopen issue: %w", err)
	}

	return nil
}
