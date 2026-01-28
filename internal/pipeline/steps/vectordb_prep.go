// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-01-28
// Last Modified: 2026-01-28

package steps

import (
	"context"
	"fmt"

	"github.com/Kavirubc/gh-simili/internal/pipeline/core"
	"github.com/Kavirubc/gh-simili/internal/vectordb"
)

// VectorDBPrep ensures the vector database collection exists for the repo.
type VectorDBPrep struct {
	vdb    VectorDBClient
	dryRun bool
}

// VectorDBClient defines the subset of vectordb.Client needed
type VectorDBClient interface {
	EnsureCollection(ctx context.Context, name string) error
}

// NewVectorDBPrep creates a new vector db prep step
func NewVectorDBPrep(vdb VectorDBClient, dryRun bool) *VectorDBPrep {
	return &VectorDBPrep{
		vdb:    vdb,
		dryRun: dryRun,
	}
}

func (s *VectorDBPrep) Name() string {
	return "vectordb_prep"
}

func (s *VectorDBPrep) Run(ctx *core.Context) error {
	if s.dryRun {
		return nil
	}

	collection := vectordb.CollectionName(ctx.Issue.Org)
	if err := s.vdb.EnsureCollection(ctx.Ctx, collection); err != nil {
		return fmt.Errorf("failed to ensure collection: %w", err)
	}

	return nil
}
