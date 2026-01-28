// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-01-28
// Last Modified: 2026-01-28

package steps

import (
	"context"
	"log"

	"github.com/Kavirubc/gh-simili/internal/pipeline/core"
	"github.com/Kavirubc/gh-simili/internal/vectordb"
	"github.com/Kavirubc/gh-simili/pkg/models"
)

// SimilaritySearch performs vector search to find similar issues.
type SimilaritySearch struct {
	finder SimilarityFinder
}

// SimilarityFinder defines the interface for similarity search
type SimilarityFinder interface {
	FindSimilar(ctx context.Context, issue *models.Issue, includeClosed bool) ([]vectordb.SearchResult, error)
}

// NewSimilaritySearch creates a new similarity search step
func NewSimilaritySearch(finder SimilarityFinder) *SimilaritySearch {
	return &SimilaritySearch{finder: finder}
}

func (s *SimilaritySearch) Name() string {
	return "similarity_search"
}

func (s *SimilaritySearch) Run(ctx *core.Context) error {
	// Optimization: If the issue is already marked for transfer, we don't need to search here.
	if ctx.TransferTarget != "" {
		log.Printf("Skipping similarity search: issue marked for transfer to %s", ctx.TransferTarget)
		return nil
	}

	similar, err := s.finder.FindSimilar(ctx.Ctx, ctx.Issue, true)
	if err != nil {
		// We log warning but don't fail the pipeline for search failure (resilience)
		// Or should we fail? The old code logged warning.
		log.Printf("Warning: similarity search failed: %v", err)
		return nil
	}

	if len(similar) > 0 {
		ctx.SimilarIssues = similar
		ctx.Result.SimilarFound = similar
	}

	return nil
}
