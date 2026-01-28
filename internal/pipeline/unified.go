// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-01-28
// Last Modified: 2026-01-28

package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/Kavirubc/gh-simili/internal/config"
	"github.com/Kavirubc/gh-simili/internal/embedding"
	"github.com/Kavirubc/gh-simili/internal/github"
	"github.com/Kavirubc/gh-simili/internal/llm"
	"github.com/Kavirubc/gh-simili/internal/pending"
	"github.com/Kavirubc/gh-simili/internal/pipeline/core"
	"github.com/Kavirubc/gh-simili/internal/processor"
	"github.com/Kavirubc/gh-simili/internal/transfer"
	"github.com/Kavirubc/gh-simili/internal/triage"
	"github.com/Kavirubc/gh-simili/internal/vectordb"
	"github.com/Kavirubc/gh-simili/pkg/models"
)

// UnifiedProcessor handles the complete issue processing pipeline
type UnifiedProcessor struct {
	cfg            *config.Config
	gh             *github.Client
	transferClient *github.Client
	embedder       *embedding.FallbackProvider
	vdb            *vectordb.Client
	similarity     *processor.SimilarityFinder
	indexer        *processor.Indexer
	triageAgent    *triage.Agent
	llmProvider    llm.Provider
	dryRun         bool
	execute        bool

	// pipeline is the sequence of steps to execute for new issues
	pipeline []core.Step
}

// NewUnifiedProcessor creates a new unified processor
func NewUnifiedProcessor(cfg *config.Config, dryRun bool, execute bool) (*UnifiedProcessor, error) {
	return NewUnifiedProcessorWithTransferToken(cfg, dryRun, execute, "")
}

// NewUnifiedProcessorWithTransferToken creates a unified processor with separate transfer token
func NewUnifiedProcessorWithTransferToken(cfg *config.Config, dryRun bool, execute bool, transferToken string) (*UnifiedProcessor, error) {
	gh, err := github.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}

	// Create transfer client with separate token if provided
	var transferClient *github.Client
	if transferToken != "" {
		transferClient, err = github.NewClientWithToken(transferToken)
		if err != nil {
			return nil, fmt.Errorf("failed to create transfer client: %w", err)
		}
	} else {
		transferClient = gh
	}

	embedder, err := embedding.NewFallbackProvider(&cfg.Embedding)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding provider: %w", err)
	}

	vdb, err := vectordb.NewClient(&cfg.Qdrant)
	if err != nil {
		embedder.Close()
		return nil, fmt.Errorf("failed to create vector DB client: %w", err)
	}

	indexer, err := processor.NewIndexer(cfg, dryRun)
	if err != nil {
		embedder.Close()
		vdb.Close()
		return nil, fmt.Errorf("failed to create indexer: %w", err)
	}

	similarity := processor.NewSimilarityFinder(cfg, embedder, vdb)

	// Create LLM provider for triage (optional - only if triage is enabled)
	var llmProvider llm.Provider
	var triageAgent *triage.Agent
	if cfg.Triage.Enabled {
		llmProvider, err = createLLMProvider(&cfg.Triage.LLM)
		if err != nil {
			log.Printf("Warning: failed to create LLM provider for triage: %v", err)
		} else {
			triageAgent = triage.NewAgentWithGitHub(cfg, llmProvider, similarity, gh)
		}
	}

	up := &UnifiedProcessor{
		cfg:            cfg,
		gh:             gh,
		transferClient: transferClient,
		embedder:       embedder,
		vdb:            vdb,
		similarity:     similarity,
		indexer:        indexer,
		triageAgent:    triageAgent,
		llmProvider:    llmProvider,
		dryRun:         dryRun,
		execute:        execute,
	}

	// Initialize the pipeline
	builder := NewBuilder(cfg, gh, transferClient, vdb, similarity, indexer, triageAgent, dryRun, execute)
	pipe, err := builder.BuildFromConfig()
	if err != nil {
		// Log warning and fallback to default if config invalid
		log.Printf("Warning: invalid pipeline configuration: %v. Using default pipeline.", err)
		pipe = builder.BuildDefault()
	}
	up.pipeline = pipe

	return up, nil
}

// createLLMProvider creates an LLM provider based on config
func createLLMProvider(cfg *config.LLMConfig) (llm.Provider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("LLM API key not configured")
	}
	switch cfg.Provider {
	case "gemini":
		return llm.NewGeminiProvider(cfg.APIKey, cfg.Model)
	case "openai":
		return llm.NewOpenAIProvider(cfg.APIKey, cfg.Model)
	default:
		return nil, fmt.Errorf("unknown LLM provider: %s", cfg.Provider)
	}
}

// Close releases all resources
func (up *UnifiedProcessor) Close() error {
	var errs []error

	if up.llmProvider != nil {
		up.llmProvider.Close()
	}
	if up.indexer != nil {
		if err := up.indexer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if up.embedder != nil {
		up.embedder.Close()
	}
	if up.vdb != nil {
		if err := up.vdb.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing resources: %v", errs)
	}
	return nil
}

// ProcessEvent processes a GitHub Action event through the unified pipeline
func (up *UnifiedProcessor) ProcessEvent(ctx context.Context, eventPath string) (*core.UnifiedResult, error) {
	event, err := github.ParseEventFile(eventPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse event: %w", err)
	}

	// Handle issue comment events
	if event.IsIssueCommentEvent() {
		issue := event.ToIssue()
		if issue == nil {
			return nil, fmt.Errorf("failed to parse issue from comment event")
		}
		return up.ProcessCommentEvent(ctx, issue)
	}

	if !event.IsIssueEvent() {
		return &core.UnifiedResult{
			Skipped:    true,
			SkipReason: "not an issue or comment event",
		}, nil
	}

	issue := event.ToIssue()
	if issue == nil {
		return nil, fmt.Errorf("failed to parse issue from event")
	}

	// Handle different event types
	switch {
	case event.IsOpenedEvent():
		return up.ProcessIssue(ctx, issue)
	case event.IsEditedEvent(), event.IsClosedEvent(), event.IsReopenedEvent():
		// For state changes, we just need to update the index
		// We use a simplified context just for indexing
		if err := up.indexer.IndexSingleIssue(ctx, issue); err != nil {
			return nil, fmt.Errorf("failed to update index: %w", err)
		}
		return &core.UnifiedResult{
			IssueNumber: issue.Number,
			Indexed:     true,
		}, nil
	case event.IsDeletedEvent():
		if err := up.indexer.DeleteIssue(ctx, issue.Org, issue.Repo, issue.Number); err != nil {
			return nil, fmt.Errorf("failed to delete from index: %w", err)
		}
		return &core.UnifiedResult{
			IssueNumber: issue.Number,
			Indexed:     true, // Flagging as "Indexed" (updated) effectively
		}, nil
	default:
		return &core.UnifiedResult{
			IssueNumber: issue.Number,
			Skipped:     true,
			SkipReason:  fmt.Sprintf("action '%s' not supported", event.Action),
		}, nil
	}
}

// ProcessIssue processes a single issue through the configured pipeline
func (up *UnifiedProcessor) ProcessIssue(ctx context.Context, issue *models.Issue) (*core.UnifiedResult, error) {
	// Initialize Pipeline Context
	pCtx := &core.Context{
		Ctx:    ctx,
		Issue:  issue,
		Config: up.cfg,
		Result: &core.UnifiedResult{IssueNumber: issue.Number},
	}

	// Execute Steps
	for _, step := range up.pipeline {
		if err := step.Run(pCtx); err != nil {
			if errors.Is(err, core.ErrSkipPipeline) {
				// Pipeline stopped gratefully (e.g. cooldown, disabled repo)
				break
			}
			return nil, fmt.Errorf("step %s failed: %w", step.Name(), err)
		}
	}

	return pCtx.Result, nil
}

// ProcessCommentEvent keeps the legacy logic for now, as it handles specific interactions
// TODO: Refactor this into a separate "InteractionPipeline" in future.
func (up *UnifiedProcessor) ProcessCommentEvent(ctx context.Context, issue *models.Issue) (*core.UnifiedResult, error) {
	result := &core.UnifiedResult{IssueNumber: issue.Number}

	// Create pending manager
	pendingMgr := pending.NewManager(up.gh, up.cfg)

	// Check if this issue has a pending action
	action, err := pendingMgr.GetPendingAction(ctx, issue)
	if err != nil {
		log.Printf("Error checking pending action: %v", err)
		result.Skipped = true
		result.SkipReason = "error checking pending action"
		return result, nil
	}

	// Check for Revert (Optimistic Transfer Undo)
	revertMgr := transfer.NewRevertManager(up.gh, up.cfg)
	revertAction, err := revertMgr.CheckForRevert(ctx, issue)
	if err != nil {
		log.Printf("Error checking for revert: %v", err)
	}

	if revertAction != nil {
		log.Printf("Found revert action for issue #%d, executing...", issue.Number)
		executor := transfer.NewExecutor(up.transferClient, up.gh, up.vdb, up.cfg, up.dryRun)
		if err := revertMgr.Revert(ctx, issue, revertAction, executor); err != nil {
			return nil, fmt.Errorf("failed to execute revert: %w", err)
		}
		result.Transferred = true
		result.ActionsExecuted = 1
		return result, nil
	}

	if action == nil {
		result.Skipped = true
		result.SkipReason = "no pending action or revert found"
		return result, nil
	}

	// Action found! Check if we should execute it
	log.Printf("Found pending %s action for issue #%d, checking status...", action.Type, issue.Number)

	switch action.Type {
	case pending.ActionTypeTransfer:
		executor := transfer.NewExecutor(up.transferClient, up.gh, up.vdb, up.cfg, up.dryRun)
		if err := executor.ProcessPendingTransfer(ctx, action); err != nil {
			return nil, fmt.Errorf("failed to process pending transfer: %w", err)
		}
		result.Transferred = true
		result.ActionsExecuted = 1

	case pending.ActionTypeClose:
		dChecker := triage.NewDuplicateCheckerWithDelayedActionsAndDryRun(&up.cfg.Triage.Duplicate, up.gh, up.cfg, up.dryRun)
		if err := dChecker.ProcessPendingClose(ctx, action); err != nil {
			return nil, fmt.Errorf("failed to process pending close: %w", err)
		}
		result.ActionsExecuted = 1
	}

	return result, nil
}

// PrintUnifiedResult outputs the processing result to stdout
// Helper method for CLI visualization
func PrintUnifiedResult(result *core.UnifiedResult) {
	fmt.Println("\n=== Unified Processing Result ===")
	fmt.Printf("Issue: #%d\n", result.IssueNumber)

	if result.Skipped {
		fmt.Printf("Skipped: %s\n", result.SkipReason)
		return
	}

	if len(result.SimilarFound) > 0 {
		fmt.Printf("Similar Issues Found: %d\n", len(result.SimilarFound))
	}

	if result.TransferTarget != "" {
		status := "scheduled"
		if result.Transferred {
			status = "executed"
		}
		fmt.Printf("Transfer to %s: %s\n", result.TransferTarget, status)
	}

	if result.TriageResult != nil {
		if len(result.TriageResult.Labels) > 0 {
			fmt.Println("Labels:")
			for _, l := range result.TriageResult.Labels {
				fmt.Printf("  - %s (%.0f%%)\n", l.Label, l.Confidence*100)
			}
		}
		if result.TriageResult.Quality != nil {
			fmt.Printf("Quality Score: %.0f%%\n", result.TriageResult.Quality.Score*100)
		}
		if result.TriageResult.Duplicate != nil && result.TriageResult.Duplicate.IsDuplicate {
			fmt.Printf("Duplicate: %.0f%% similar to #%d\n",
				result.TriageResult.Duplicate.Similarity*100,
				result.TriageResult.Duplicate.Original.Number)
		}
	}

	if result.CommentPosted {
		fmt.Println("Comment: posted")
	}

	if result.Indexed {
		fmt.Println("Index: updated")
	}

	if result.ActionsExecuted > 0 {
		fmt.Printf("Actions Executed: %d\n", result.ActionsExecuted)
	}
}
