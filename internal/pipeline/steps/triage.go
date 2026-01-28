// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-01-28
// Last Modified: 2026-01-28

package steps

import (
	"context"
	"log"
	"time"

	"github.com/Kavirubc/gh-simili/internal/pending"
	"github.com/Kavirubc/gh-simili/internal/pipeline/core"
	"github.com/Kavirubc/gh-simili/internal/triage"
	"github.com/Kavirubc/gh-simili/internal/vectordb"
	"github.com/Kavirubc/gh-simili/pkg/models"
)

// TriageAnalysis runs the AI triage agent to determine labels, quality, and duplicates.
type TriageAnalysis struct {
	agent TriageAgent
}

// TriageAgent defines the interface for the triage agent
type TriageAgent interface {
	TriageWithSimilar(ctx context.Context, issue *models.Issue, similar []vectordb.SearchResult) (*triage.Result, error)
	TriageWithoutDuplicates(ctx context.Context, issue *models.Issue, similar []vectordb.SearchResult) (*triage.Result, error)
}

// NewTriageAnalysis creates a new triage step
func NewTriageAnalysis(agent TriageAgent) *TriageAnalysis {
	return &TriageAnalysis{agent: agent}
}

func (s *TriageAnalysis) Name() string {
	return "triage"
}

func (s *TriageAnalysis) Run(ctx *core.Context) error {
	if s.agent == nil {
		return nil
	}

	var result *triage.Result
	var err error

	// If transferring, skip duplicate check to avoid confusion
	skipDuplicateCheck := ctx.TransferTarget != ""

	if skipDuplicateCheck {
		result, err = s.agent.TriageWithoutDuplicates(ctx.Ctx, ctx.Issue, ctx.SimilarIssues)
	} else {
		result, err = s.agent.TriageWithSimilar(ctx.Ctx, ctx.Issue, ctx.SimilarIssues)
	}

	if err != nil {
		log.Printf("Warning: triage failed: %v", err)
		return nil
	}

	ctx.TriageResult = result
	ctx.Result.TriageResult = result

	// Prepare pending close if duplicate found
	s.checkForPendingClose(ctx, result)

	return nil
}

func (s *TriageAnalysis) checkForPendingClose(ctx *core.Context, result *triage.Result) {
	if result.Duplicate != nil && result.Duplicate.IsDuplicate && result.Duplicate.ShouldClose &&
		ctx.Config.Defaults.DelayedActions.Enabled && ctx.Result.PendingAction == nil {

		delayHours := ctx.Config.Defaults.DelayedActions.DelayHours
		expiresAt := time.Now().Add(time.Duration(delayHours) * time.Hour)

		ctx.Result.PendingAction = &pending.PendingAction{
			Type:        pending.ActionTypeClose,
			Org:         ctx.Issue.Org,
			Repo:        ctx.Issue.Repo,
			IssueNumber: ctx.Issue.Number,
			Target:      result.Duplicate.Original.URL,
			ScheduledAt: time.Now(),
			ExpiresAt:   expiresAt,
		}
	}
}
