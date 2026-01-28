// Author: Kaviru Hapuarachchi
// GitHub: [https://github.com/kavirubc](https://github.com/kavirubc)
// Created: 2026-01-28
// Last Modified: 2026-01-28

package steps

import (
	"log"
	"strings"
	"time"

	"github.com/Kavirubc/gh-simili/internal/github"
	"github.com/Kavirubc/gh-simili/internal/llm"
	"github.com/Kavirubc/gh-simili/internal/pending"
	"github.com/Kavirubc/gh-simili/internal/pipeline/core"
	"github.com/Kavirubc/gh-simili/internal/transfer"
	"github.com/Kavirubc/gh-simili/internal/triage"
)

// TransferCheck evaluates if an issue matches any transfer rules or AI intent routing.
type TransferCheck struct {
	llm llm.Provider
	gh  *github.Client
}

// NewTransferCheck creates a new transfer check step
func NewTransferCheck(llmProvider llm.Provider, gh *github.Client) *TransferCheck {
	return &TransferCheck{
		llm: llmProvider,
		gh:  gh,
	}
}

func (s *TransferCheck) Name() string {
	return "transfer_check"
}

func (s *TransferCheck) Run(ctx *core.Context) error {
	repoConfig := ctx.Config.GetRepoConfig(ctx.Issue.Org, ctx.Issue.Repo)
	if repoConfig == nil {
		return nil
	}

	// 1. Check for Revert Loop Prevention
	// If the issue was recently reverted, we skip automatic transfer
	if s.isReverted(ctx) {
		log.Printf("Issue #%d was recently reverted, skipping automatic transfer to prevent loops", ctx.Issue.Number)
		return nil
	}

	var target string

	// 2. Try Strict Rule Matching (Fast)
	if len(repoConfig.TransferRules) > 0 {
		matcher := transfer.NewRuleMatcher(repoConfig.TransferRules)
		matched, _ := matcher.Match(ctx.Issue)
		target = matched
	}

	// 3. Fallback to AI Intent Routing (Slow but Accurate)
	if target == "" && ctx.Config.Triage.Router.Enabled {
		router := triage.NewRouter(s.llm, ctx.Config.Repositories)
		result, err := router.Route(ctx.Ctx, ctx.Issue)
		if err != nil {
			log.Printf("Warning: AI routing failed: %v", err)
		} else if result != nil && result.Confidence >= 0.8 {
			// Ensure we don't route to the same repo
			currentRepo := strings.ToLower(ctx.Issue.Org + "/" + ctx.Issue.Repo)
			if strings.ToLower(result.TargetRepo) != currentRepo {
				log.Printf("AI Router suggested transfer: %s -> %s (Reason: %s)", currentRepo, result.TargetRepo, result.Reason)
				target = result.TargetRepo
			}
		}
	}

	if target == "" {
		return nil
	}

	// Match found
	log.Printf("Transfer target identified: %s -> %s", ctx.Issue.Repo, target)
	ctx.TransferTarget = target

	// Handle Delayed Actions Logic
	if ctx.Config.Defaults.DelayedActions.Enabled {
		delayHours := ctx.Config.Defaults.DelayedActions.DelayHours
		expiresAt := time.Now().Add(time.Duration(delayHours) * time.Hour)

		ctx.Result.PendingAction = &pending.PendingAction{
			Type:        pending.ActionTypeTransfer,
			Org:         ctx.Issue.Org,
			Repo:        ctx.Issue.Repo,
			IssueNumber: ctx.Issue.Number,
			Target:      target,
			ScheduledAt: time.Now(),
			ExpiresAt:   expiresAt,
		}
	}

	return nil
}

// isReverted checks if the issue was recently moved back via revert
func (s *TransferCheck) isReverted(ctx *core.Context) bool {
	revertMarker := "↩️ Reverting transfer"

	// 1. Check issue body
	if strings.Contains(ctx.Issue.Body, revertMarker) {
		return true
	}

	// 2. Check comment history (most reliable)
	// We only check if gh client is available
	if s.gh != nil {
		comments, err := s.gh.ListComments(ctx.Ctx, ctx.Issue.Org, ctx.Issue.Repo, ctx.Issue.Number)
		if err == nil {
			for i := len(comments) - 1; i >= 0; i-- {
				if strings.Contains(comments[i].Body, revertMarker) {
					return true
				}
				// Only look at recent comments to allow future transfers
				if i < len(comments)-5 {
					break
				}
			}
		}
	}

	return false
}
