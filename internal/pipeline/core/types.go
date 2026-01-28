// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-01-28
// Last Modified: 2026-01-28

package core

import (
	"context"
	"errors"

	"github.com/Kavirubc/gh-simili/internal/config"
	"github.com/Kavirubc/gh-simili/internal/pending"
	"github.com/Kavirubc/gh-simili/internal/triage"
	"github.com/Kavirubc/gh-simili/internal/vectordb"
	"github.com/Kavirubc/gh-simili/pkg/models"
)

// ErrSkipPipeline indicates that the rest of the pipeline should be skipped purely for logic reasons
// (e.g. repo disabled, cooldown active). It is not an error condition.
var ErrSkipPipeline = errors.New("skip pipeline")

// UnifiedResult contains the complete result of unified processing
type UnifiedResult struct {
	IssueNumber     int                     `json:"issue_number"`
	Skipped         bool                    `json:"skipped,omitempty"`
	SkipReason      string                  `json:"skip_reason,omitempty"`
	SimilarFound    []vectordb.SearchResult `json:"similar_found,omitempty"`
	TriageResult    *triage.Result          `json:"triage_result,omitempty"`
	Transferred     bool                    `json:"transferred,omitempty"`
	TransferTarget  string                  `json:"transfer_target,omitempty"`
	CommentPosted   bool                    `json:"comment_posted,omitempty"`
	Indexed         bool                    `json:"indexed,omitempty"`
	ActionsExecuted int                     `json:"actions_executed,omitempty"`
	PendingAction   *pending.PendingAction  `json:"pending_action,omitempty"`
}

// Context carries state through the pipeline steps.
// It follows "Effective Go" by using direct field access for simplicity within the package.
type Context struct {
	// Base Inputs
	Ctx    context.Context
	Issue  *models.Issue
	Config *config.Config

	// Mutable State
	// Result accumulates the final output structure
	Result *UnifiedResult

	// SimilarIssues holds vector search results
	SimilarIssues []vectordb.SearchResult

	// TransferTarget holds the matched transfer target repo name (if any)
	TransferTarget string

	// TriageResult holds the output of the LLM/Rule-based triage
	TriageResult *triage.Result

	// CommentBody holds the generated comment text (if any)
	CommentBody string

	// SkipReason is set when ErrSkipPipeline is returned to explain why
	SkipReason string
}

// Step defines a single unit of work in the pipeline.
type Step interface {
	// Name returns the unique identifier for this step (used in config/logs)
	Name() string
	// Run executes the step logic.
	// Returning ErrSkipPipeline gracefully stops execution.
	// Returning any other error halts execution and is treated as a failure.
	Run(ctx *Context) error
}
