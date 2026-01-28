// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-01-28
// Last Modified: 2026-01-28

package steps

import (
	"log"
	"time"

	"github.com/Kavirubc/gh-simili/internal/pending"
	"github.com/Kavirubc/gh-simili/internal/pipeline/core"
	"github.com/Kavirubc/gh-simili/internal/transfer"
)

// TransferCheck evaluates if an issue matches any transfer rules.
type TransferCheck struct{}

// NewTransferCheck creates a new transfer check step
func NewTransferCheck() *TransferCheck {
	return &TransferCheck{}
}

func (s *TransferCheck) Name() string {
	return "transfer_check"
}

func (s *TransferCheck) Run(ctx *core.Context) error {
	repoConfig := ctx.Config.GetRepoConfig(ctx.Issue.Org, ctx.Issue.Repo)
	// If no config or no rules, nothing to do
	if repoConfig == nil || len(repoConfig.TransferRules) == 0 {
		return nil
	}

	matcher := transfer.NewRuleMatcher(repoConfig.TransferRules)
	target, _ := matcher.Match(ctx.Issue)

	if target == "" {
		return nil
	}

	// Match found
	log.Printf("Transfer rule matched: %s -> %s", ctx.Issue.Repo, target)
	ctx.TransferTarget = target

	// Handle Delayed Actions Logic
	if ctx.Config.Defaults.DelayedActions.Enabled {
		delayHours := ctx.Config.Defaults.DelayedActions.DelayHours
		expiresAt := time.Now().Add(time.Duration(delayHours) * time.Hour)

		// We need to attach this to a place in Context where the Result builder can find it.
		// For now, we'll assume the Context.Result has a PendingAction field (from original UnifiedResult)
		// But wait, pipeline.Context passes a pointer to UnifiedResult.
		// Let's verify UnifiedResult definition. Assuming it matches the old one.

		// Note: We need to make sure we import 'pending' if we are using it.
		// Ideally we shouldn't construct the PendingAction here if we want to be purely functional,
		// but for now we follow the improved logic.

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
