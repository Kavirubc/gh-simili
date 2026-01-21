package processor

import (
	"context"
	"fmt"
	"strings"

	"github.com/kaviruhapuarachchi/gh-simili/internal/config"
	"github.com/kaviruhapuarachchi/gh-simili/internal/embedding"
	"github.com/kaviruhapuarachchi/gh-simili/internal/vectordb"
	"github.com/kaviruhapuarachchi/gh-simili/pkg/models"
	"github.com/qdrant/go-client/qdrant"
)

// SimilarityFinder searches for similar issues
type SimilarityFinder struct {
	cfg      *config.Config
	embedder *embedding.FallbackProvider
	vdb      *vectordb.Client
}

// NewSimilarityFinder creates a new similarity finder
func NewSimilarityFinder(cfg *config.Config, embedder *embedding.FallbackProvider, vdb *vectordb.Client) *SimilarityFinder {
	return &SimilarityFinder{
		cfg:      cfg,
		embedder: embedder,
		vdb:      vdb,
	}
}

// FindSimilar finds similar issues for a given issue
func (sf *SimilarityFinder) FindSimilar(ctx context.Context, issue *models.Issue, excludeSelf bool) ([]vectordb.SearchResult, error) {
	text := embedding.PrepareIssueText(issue.Title, issue.Body)
	vector, err := sf.embedder.Embed(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	collection := vectordb.CollectionName(issue.Org)
	threshold := sf.cfg.GetSimilarityThreshold(issue.Org, issue.Repo)
	limit := sf.cfg.Defaults.MaxSimilarToShow
	closedWeight := sf.cfg.Defaults.ClosedIssueWeight

	var filter *qdrant.Filter
	if excludeSelf {
		// Exclude the issue itself from results (must match all: org, repo, and number)
		filter = &qdrant.Filter{
			MustNot: []*qdrant.Condition{
				{
					ConditionOneOf: &qdrant.Condition_Filter{
						Filter: &qdrant.Filter{
							Must: []*qdrant.Condition{
								qdrant.NewMatchKeyword("org", issue.Org),
								qdrant.NewMatchKeyword("repo", issue.Repo),
								qdrant.NewMatchInt("number", int64(issue.Number)),
							},
						},
					},
				},
			},
		}
	}

	var results []vectordb.SearchResult
	if filter != nil {
		results, err = sf.vdb.SearchFiltered(ctx, collection, vector, limit+1, threshold, closedWeight, filter)
	} else {
		results, err = sf.vdb.Search(ctx, collection, vector, limit+1, threshold, closedWeight)
	}

	if err != nil {
		return nil, err
	}

	// Filter out self if present (backup check)
	if excludeSelf {
		filtered := make([]vectordb.SearchResult, 0, len(results))
		for _, r := range results {
			if r.Issue.Org == issue.Org && r.Issue.Repo == issue.Repo && r.Issue.Number == issue.Number {
				continue
			}
			filtered = append(filtered, r)
		}
		results = filtered
	}

	// Trim to limit
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// FindSimilarByText finds similar issues for a text query
func (sf *SimilarityFinder) FindSimilarByText(ctx context.Context, text string, org string, limit int) ([]vectordb.SearchResult, error) {
	vector, err := sf.embedder.Embed(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	collection := vectordb.CollectionName(org)
	threshold := sf.cfg.Defaults.SimilarityThreshold
	closedWeight := sf.cfg.Defaults.ClosedIssueWeight

	return sf.vdb.Search(ctx, collection, vector, limit, threshold, closedWeight)
}

// FormatSimilarityComment creates the similarity comment for posting
func FormatSimilarityComment(results []vectordb.SearchResult, crossRepo bool) string {
	if len(results) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("👋 Thanks for opening this issue!\n\n")
	sb.WriteString("I found some potentially related issues that might be helpful:\n\n")

	if crossRepo {
		sb.WriteString("| Issue | Repository | Similarity | Status |\n")
		sb.WriteString("|-------|------------|------------|--------|\n")
	} else {
		sb.WriteString("| Issue | Similarity | Status |\n")
		sb.WriteString("|-------|------------|--------|\n")
	}

	for _, r := range results {
		status := "🟢 Open"
		if r.Issue.State == "closed" {
			status = "🔴 Closed"
		}

		title := truncateString(r.Issue.Title, 50)
		link := fmt.Sprintf("[#%d - %s](%s)", r.Issue.Number, title, r.Issue.URL)
		similarity := fmt.Sprintf("%.0f%%", r.Score*100)

		if crossRepo {
			repo := fmt.Sprintf("%s/%s", r.Issue.Org, r.Issue.Repo)
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", link, repo, similarity, status))
		} else {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", link, similarity, status))
		}
	}

	sb.WriteString("\nIf any of these address your problem, please let us know and we can close this as a duplicate.\n\n")
	sb.WriteString("---\n")
	sb.WriteString("<sub>🤖 gh-simili Issue Intelligence</sub>")

	return sb.String()
}

// truncateString truncates a string to maxLen with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// HasCrossRepoResults checks if results span multiple repos
func HasCrossRepoResults(results []vectordb.SearchResult, sourceOrg, sourceRepo string) bool {
	for _, r := range results {
		if r.Issue.Org != sourceOrg || r.Issue.Repo != sourceRepo {
			return true
		}
	}
	return false
}
