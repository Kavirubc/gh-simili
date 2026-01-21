package vectordb

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/kaviruhapuarachchi/gh-simili/pkg/models"
	"github.com/qdrant/go-client/qdrant"
)

// SearchResult contains a search result with score
type SearchResult struct {
	Issue models.Issue
	Score float64
}

// Search finds similar issues in a collection
func (c *Client) Search(ctx context.Context, collection string, vector []float32, limit int, threshold float64, closedWeight float64) ([]SearchResult, error) {
	scoreThreshold := float32(threshold)

	points, err := c.qdrant.Query(ctx, &qdrant.QueryPoints{
		CollectionName: collection,
		Query:          qdrant.NewQuery(vector...),
		Limit:          qdrant.PtrOf(uint64(limit * 2)), // Fetch extra for closed weight adjustment
		ScoreThreshold: &scoreThreshold,
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	results := make([]SearchResult, 0, len(points))
	for _, point := range points {
		issue := payloadToIssue(point.Payload)
		score := float64(point.Score)

		// Apply closed issue weight adjustment
		if issue.State == "closed" && closedWeight > 0 {
			score *= closedWeight
		}

		results = append(results, SearchResult{
			Issue: issue,
			Score: score,
		})
	}

	// Re-sort after weight adjustment
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Trim to requested limit
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// SearchFiltered searches with additional filters
func (c *Client) SearchFiltered(ctx context.Context, collection string, vector []float32, limit int, threshold float64, closedWeight float64, filter *qdrant.Filter) ([]SearchResult, error) {
	scoreThreshold := float32(threshold)

	points, err := c.qdrant.Query(ctx, &qdrant.QueryPoints{
		CollectionName: collection,
		Query:          qdrant.NewQuery(vector...),
		Limit:          qdrant.PtrOf(uint64(limit * 2)),
		ScoreThreshold: &scoreThreshold,
		WithPayload:    qdrant.NewWithPayload(true),
		Filter:         filter,
	})
	if err != nil {
		return nil, fmt.Errorf("filtered search failed: %w", err)
	}

	results := make([]SearchResult, 0, len(points))
	for _, point := range points {
		issue := payloadToIssue(point.Payload)
		score := float64(point.Score)

		if issue.State == "closed" && closedWeight > 0 {
			score *= closedWeight
		}

		results = append(results, SearchResult{
			Issue: issue,
			Score: score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// payloadToIssue converts Qdrant payload to Issue
func payloadToIssue(payload map[string]*qdrant.Value) models.Issue {
	issue := models.Issue{}

	if v := payload["org"]; v != nil {
		issue.Org = v.GetStringValue()
	}
	if v := payload["repo"]; v != nil {
		issue.Repo = v.GetStringValue()
	}
	if v := payload["number"]; v != nil {
		issue.Number = int(v.GetIntegerValue())
	}
	if v := payload["title"]; v != nil {
		issue.Title = v.GetStringValue()
	}
	if v := payload["state"]; v != nil {
		issue.State = v.GetStringValue()
	}
	if v := payload["author"]; v != nil {
		issue.Author = v.GetStringValue()
	}
	if v := payload["url"]; v != nil {
		issue.URL = v.GetStringValue()
	}
	if v := payload["created_at"]; v != nil {
		issue.CreatedAt, _ = time.Parse(time.RFC3339, v.GetStringValue())
	}
	if v := payload["updated_at"]; v != nil {
		issue.UpdatedAt, _ = time.Parse(time.RFC3339, v.GetStringValue())
	}
	if v := payload["labels"]; v != nil {
		if list := v.GetListValue(); list != nil {
			for _, item := range list.Values {
				issue.Labels = append(issue.Labels, item.GetStringValue())
			}
		}
	}

	return issue
}
