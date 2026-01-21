package github

import (
	"context"
	"fmt"
)

// TransferIssue transfers an issue to another repository
func (c *Client) TransferIssue(ctx context.Context, org, repo string, number int, targetRepo string) error {
	targetOrg, targetRepoName, err := ParseRepo(targetRepo)
	if err != nil {
		return err
	}

	// Use GraphQL mutation for issue transfer
	var mutation struct {
		TransferIssue struct {
			Issue struct {
				Number int
			}
		} `graphql:"transferIssue(input: $input)"`
	}

	// First, get the issue node ID
	nodeID, err := c.getIssueNodeID(ctx, org, repo, number)
	if err != nil {
		return fmt.Errorf("failed to get issue node ID: %w", err)
	}

	// Get target repo node ID
	targetRepoID, err := c.getRepoNodeID(ctx, targetOrg, targetRepoName)
	if err != nil {
		return fmt.Errorf("failed to get target repo node ID: %w", err)
	}

	query := `
		mutation TransferIssue($issueId: ID!, $repositoryId: ID!) {
			transferIssue(input: {issueId: $issueId, repositoryId: $repositoryId}) {
				issue {
					number
				}
			}
		}
	`

	variables := map[string]interface{}{
		"issueId":      nodeID,
		"repositoryId": targetRepoID,
	}

	if err := c.graphql.Do(query, variables, &mutation); err != nil {
		return fmt.Errorf("failed to transfer issue: %w", err)
	}

	return nil
}

// getIssueNodeID fetches the GraphQL node ID for an issue
func (c *Client) getIssueNodeID(ctx context.Context, org, repo string, number int) (string, error) {
	query := `
		query GetIssueID($owner: String!, $repo: String!, $number: Int!) {
			repository(owner: $owner, name: $repo) {
				issue(number: $number) {
					id
				}
			}
		}
	`

	var result struct {
		Repository struct {
			Issue struct {
				ID string
			}
		}
	}

	variables := map[string]interface{}{
		"owner":  org,
		"repo":   repo,
		"number": number,
	}

	if err := c.graphql.Do(query, variables, &result); err != nil {
		return "", err
	}

	return result.Repository.Issue.ID, nil
}

// getRepoNodeID fetches the GraphQL node ID for a repository
func (c *Client) getRepoNodeID(ctx context.Context, org, repo string) (string, error) {
	query := `
		query GetRepoID($owner: String!, $repo: String!) {
			repository(owner: $owner, name: $repo) {
				id
			}
		}
	`

	var result struct {
		Repository struct {
			ID string
		}
	}

	variables := map[string]interface{}{
		"owner": org,
		"repo":  repo,
	}

	if err := c.graphql.Do(query, variables, &result); err != nil {
		return "", err
	}

	return result.Repository.ID, nil
}
