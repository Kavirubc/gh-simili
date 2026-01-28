// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-01-28
// Last Modified: 2026-01-28

package steps

import (
	"log"

	"github.com/Kavirubc/gh-simili/internal/github"
	"github.com/Kavirubc/gh-simili/internal/pipeline/core"
	"github.com/Kavirubc/gh-simili/internal/transfer"
	"github.com/Kavirubc/gh-simili/internal/triage"
	"github.com/Kavirubc/gh-simili/internal/vectordb"
)

type ActionExecutor struct {
	gh             *github.Client
	transferClient *github.Client
	vdb            *vectordb.Client
	dryRun         bool
	runActions     bool // "execute" flag in old unified.go
}

func NewActionExecutor(gh *github.Client, transferClient *github.Client, vdb *vectordb.Client, dryRun bool, runActions bool) *ActionExecutor {
	return &ActionExecutor{
		gh:             gh,
		transferClient: transferClient,
		vdb:            vdb,
		dryRun:         dryRun,
		runActions:     runActions,
	}
}

func (s *ActionExecutor) Name() string {
	return "action_executor"
}

func (s *ActionExecutor) Run(ctx *core.Context) error {
	if s.dryRun || !s.runActions {
		log.Println("Dry run or execute=false, skipping side effects")
		return nil
	}

	// 1. Post Comment
	commentID := 0
	if ctx.CommentBody != "" {
		id, err := s.gh.PostCommentWithID(ctx.Ctx, ctx.Issue.Org, ctx.Issue.Repo, ctx.Issue.Number, ctx.CommentBody)
		if err != nil {
			log.Printf("Warning: failed to post unified comment: %v", err)
		} else {
			ctx.Result.CommentPosted = true
			commentID = id
		}
	}

	// 2. Execute Transfer
	if ctx.TransferTarget != "" {
		s.executeTransfer(ctx, commentID)
	}

	// 3. Execute Triage Actions
	if ctx.TriageResult != nil {
		s.executeTriageRequest(ctx, commentID)
	}

	return nil
}

func (s *ActionExecutor) executeTransfer(ctx *core.Context, commentID int) {
	executor := transfer.NewExecutor(s.transferClient, s.gh, s.vdb, ctx.Config, s.dryRun)

	// Optimistic?
	if ctx.Config.Defaults.DelayedActions.Enabled && ctx.Config.Defaults.DelayedActions.OptimisticTransfers {
		if err := executor.Transfer(ctx.Ctx, ctx.Issue, ctx.TransferTarget, nil); err != nil { // nil rule? we lost the rule obj in Context, but maybe Transfer doesn't NEED it if target is set?
			// Checking transfer.go: Transfer(ctx, issue, target, rule). The rule is used for logging priority.
			// Currently we didn't store the rule in Context, only the target.
			// That's acceptable for now.
			log.Printf("Warning: failed to execute optimistic transfer: %v", err)
		} else {
			ctx.Result.Transferred = true
			ctx.Result.ActionsExecuted++
		}
	} else if ctx.Result.CommentPosted {
		// Delayed Silent
		if err := executor.ScheduleTransferSilent(ctx.Ctx, ctx.Issue, ctx.TransferTarget, commentID); err != nil {
			log.Printf("Warning: failed to schedule transfer: %v", err)
		}
	} else {
		// Fallback
		if err := executor.Transfer(ctx.Ctx, ctx.Issue, ctx.TransferTarget, nil); err != nil {
			log.Printf("Warning: failed to transfer: %v", err)
		} else {
			ctx.Result.Transferred = true
			ctx.Result.ActionsExecuted++
		}
	}
}

func (s *ActionExecutor) executeTriageRequest(ctx *core.Context, commentID int) {
	// Filter comment actions since we already posted unified comment
	actions := filterNonCommentActions(ctx.TriageResult.Actions)

	var executor *triage.Executor
	if ctx.Config.Defaults.DelayedActions.Enabled {
		dupChecker := triage.NewDuplicateCheckerWithDelayedActions(&ctx.Config.Triage.Duplicate, s.gh, ctx.Config)

		// Silent close scheduling
		if ctx.TriageResult.Duplicate != nil && ctx.TriageResult.Duplicate.IsDuplicate &&
			ctx.TriageResult.Duplicate.ShouldClose && ctx.Result.CommentPosted {

			if err := dupChecker.ScheduleCloseSilent(ctx.Ctx, ctx.Issue, ctx.TriageResult.Duplicate.Original.URL, commentID); err != nil {
				log.Printf("Warning: failed to schedule close: %v", err)
			}
			actions = filterCloseActions(actions)
		}
		executor = triage.NewExecutorWithDelayedActions(s.gh, ctx.Config, dupChecker, s.dryRun)
	} else {
		executor = triage.NewExecutor(s.gh, s.dryRun)
	}

	filteredResult := *ctx.TriageResult // Copy
	filteredResult.Actions = actions

	if err := executor.Execute(ctx.Ctx, ctx.Issue, &filteredResult); err != nil {
		log.Printf("Warning: failed to execute triage actions: %v", err)
	} else {
		ctx.Result.ActionsExecuted += len(actions)
	}
}

// Helpers copied from unified.go (or we should export them there? No, better copy or put in triage package)

func filterNonCommentActions(actions []triage.Action) []triage.Action {
	filtered := make([]triage.Action, 0, len(actions))
	for _, a := range actions {
		if a.Type != triage.ActionComment {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

func filterCloseActions(actions []triage.Action) []triage.Action {
	filtered := make([]triage.Action, 0, len(actions))
	for _, a := range actions {
		if a.Type != triage.ActionClose {
			filtered = append(filtered, a)
		}
	}
	return filtered
}
