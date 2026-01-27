package github

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/Kavirubc/gh-simili/pkg/models"
)

// ListOptions configures issue listing
type ListOptions struct {
	State   string // "open", "closed", "all"
	PerPage int
	Page    int
	Since   time.Time
}

// ListIssues fetches issues from a repository
func (c *Client) ListIssues(ctx context.Context, org, repo string, opts ListOptions) ([]*models.Issue, error) {
	if opts.PerPage == 0 {
		opts.PerPage = 100
	}
	if opts.State == "" {
		opts.State = "all"
	}
	if opts.Page == 0 {
		opts.Page = 1
	}

	params := url.Values{}
	params.Set("state", opts.State)
	params.Set("per_page", strconv.Itoa(opts.PerPage))
	params.Set("page", strconv.Itoa(opts.Page))
	params.Set("sort", "updated")
	params.Set("direction", "desc")
	if !opts.Since.IsZero() {
		params.Set("since", opts.Since.Format(time.RFC3339))
	}

	endpoint := fmt.Sprintf("repos/%s/%s/issues?%s", org, repo, params.Encode())

	var apiIssues []Issue
	if err := c.rest.Get(endpoint, &apiIssues); err != nil {
		return nil, fmt.Errorf("failed to list issues: %w", err)
	}

	issues := make([]*models.Issue, 0, len(apiIssues))
	for _, ai := range apiIssues {
		// Skip pull requests (they appear in issues endpoint)
		if ai.isPullRequest() {
			continue
		}
		issues = append(issues, ai.ToModel(org, repo))
	}

	return issues, nil
}

// GetIssue fetches a single issue
func (c *Client) GetIssue(ctx context.Context, org, repo string, number int) (*models.Issue, error) {
	endpoint := fmt.Sprintf("repos/%s/%s/issues/%d", org, repo, number)

	var ai Issue
	if err := c.rest.Get(endpoint, &ai); err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}

	return ai.ToModel(org, repo), nil
}

// ListAllIssues fetches all issues using pagination
func (c *Client) ListAllIssues(ctx context.Context, org, repo string, state string, batchSize int) ([]*models.Issue, error) {
	var allIssues []*models.Issue
	page := 1

	for {
		issues, err := c.ListIssues(ctx, org, repo, ListOptions{
			State:   state,
			PerPage: batchSize,
			Page:    page,
		})
		if err != nil {
			return nil, err
		}

		if len(issues) == 0 {
			break
		}

		allIssues = append(allIssues, issues...)

		if len(issues) < batchSize {
			break
		}
		page++
	}

	return allIssues, nil
}

// isPullRequest checks if an issue is actually a pull request.
// NOTE: The GitHub /issues endpoint includes pull requests, but the go-gh Issue
// struct does not expose the "pull_request" field from the API response.
// As a result, this function always returns false and PRs will be indexed
// alongside issues. This is acceptable for similarity search purposes since
// PRs often contain relevant context about code changes.
func (i *Issue) isPullRequest() bool {
	return false
}

// ListIssuesByLabel fetches issues with a specific label with pagination
func (c *Client) ListIssuesByLabel(ctx context.Context, org, repo, label string) ([]*models.Issue, error) {
	var allIssues []*models.Issue
	page := 1
	perPage := 100

	for {
		params := url.Values{}
		params.Set("labels", label)
		params.Set("state", "open")
		params.Set("per_page", strconv.Itoa(perPage))
		params.Set("page", strconv.Itoa(page))
		params.Set("sort", "updated")
		params.Set("direction", "desc")

		endpoint := fmt.Sprintf("repos/%s/%s/issues?%s", org, repo, params.Encode())

		var apiIssues []Issue
		if err := c.rest.Get(endpoint, &apiIssues); err != nil {
			return nil, fmt.Errorf("failed to list issues by label: %w", err)
		}

		if len(apiIssues) == 0 {
			break
		}

		for _, ai := range apiIssues {
			if ai.isPullRequest() {
				continue
			}
			allIssues = append(allIssues, ai.ToModel(org, repo))
		}

		if len(apiIssues) < perPage {
			break
		}
		page++
	}

	return allIssues, nil
}
