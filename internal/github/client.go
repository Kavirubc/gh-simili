package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/kaviruhapuarachchi/gh-simili/pkg/models"
)

// Client wraps GitHub API operations
type Client struct {
	rest    *api.RESTClient
	graphql *api.GraphQLClient
}

// NewClient creates a new GitHub client
func NewClient() (*Client, error) {
	rest, err := api.DefaultRESTClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create REST client: %w", err)
	}

	graphql, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create GraphQL client: %w", err)
	}

	return &Client{
		rest:    rest,
		graphql: graphql,
	}, nil
}

// Close releases resources
func (c *Client) Close() error {
	return nil
}

// ParseRepo splits "owner/repo" into owner and repo
func ParseRepo(fullRepo string) (string, string, error) {
	parts := strings.Split(fullRepo, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo format: %s (expected owner/repo)", fullRepo)
	}
	return parts[0], parts[1], nil
}

// Issue represents a GitHub issue from the API
type Issue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	State     string    `json:"state"`
	HTMLURL   string    `json:"html_url"`
	User      User      `json:"user"`
	Labels    []Label   `json:"labels"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// User represents a GitHub user
type User struct {
	Login string `json:"login"`
}

// Label represents a GitHub label
type Label struct {
	Name string `json:"name"`
}

// Comment represents a GitHub comment
type Comment struct {
	ID        int       `json:"id"`
	Body      string    `json:"body"`
	User      User      `json:"user"`
	CreatedAt time.Time `json:"created_at"`
}

// ToModel converts API Issue to models.Issue
func (i *Issue) ToModel(org, repo string) *models.Issue {
	labels := make([]string, len(i.Labels))
	for j, l := range i.Labels {
		labels[j] = l.Name
	}

	return &models.Issue{
		Org:       org,
		Repo:      repo,
		Number:    i.Number,
		Title:     i.Title,
		Body:      i.Body,
		State:     i.State,
		Labels:    labels,
		Author:    i.User.Login,
		URL:       i.HTMLURL,
		CreatedAt: i.CreatedAt,
		UpdatedAt: i.UpdatedAt,
	}
}

// RepoExists checks if a repository exists
func (c *Client) RepoExists(ctx context.Context, org, repo string) (bool, error) {
	var result struct{}
	err := c.rest.Get(fmt.Sprintf("repos/%s/%s", org, repo), &result)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
