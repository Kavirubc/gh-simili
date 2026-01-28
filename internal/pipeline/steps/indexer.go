// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-01-28
// Last Modified: 2026-01-28

package steps

import (
	"context"
	"log"

	"github.com/Kavirubc/gh-simili/internal/pipeline/core"
	"github.com/Kavirubc/gh-simili/pkg/models"
)

// Indexer adds the issue to the vector database.
type Indexer struct {
	client Interface
	dryRun bool
}

// Interface defines the indexing capability
type Interface interface {
	IndexSingleIssue(ctx context.Context, issue *models.Issue) error
}

// NewIndexer creates a new indexer step
func NewIndexer(client Interface, dryRun bool) *Indexer {
	return &Indexer{
		client: client,
		dryRun: dryRun,
	}
}

func (s *Indexer) Name() string {
	return "indexer"
}

func (s *Indexer) Run(ctx *core.Context) error {
	// Skip logic from unified.go
	if ctx.TransferTarget != "" {
		log.Printf("Skipping indexing: issue will be transferred")
		return nil
	}
	if ctx.TriageResult != nil && ctx.TriageResult.Duplicate != nil && ctx.TriageResult.Duplicate.ShouldClose {
		log.Printf("Skipping indexing: issue will be closed as duplicate")
		return nil
	}

	if s.dryRun {
		return nil
	}

	if err := s.client.IndexSingleIssue(ctx.Ctx, ctx.Issue); err != nil {
		log.Printf("Warning: failed to index issue: %v", err)
	} else {
		ctx.Result.Indexed = true
	}

	return nil
}
