// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-01-28
// Last Modified: 2026-01-28

package steps

import (
	"context"
	"fmt"

	"github.com/Kavirubc/gh-simili/internal/github"
	"github.com/Kavirubc/gh-simili/internal/pipeline/core"
)

// RepoGatekeeper checks if the repository is enabled configuration
// and if any cooldowns are active.
type RepoGatekeeper struct {
	gh Client
}

// Client defines the subset of github.Client needed for this step
type Client interface {
	ShouldSkipComment(ctx context.Context, org, repo string, issueNum, cooldownHours int) (bool, error)
}

// NewRepoGatekeeper creates a new gatekeeper step
func NewRepoGatekeeper(gh *github.Client) *RepoGatekeeper {
	return &RepoGatekeeper{gh: gh}
}

func (s *RepoGatekeeper) Name() string {
	return "gatekeeper"
}

func (s *RepoGatekeeper) Run(ctx *core.Context) error {
	// 1. Check if repo is enabled in config
	repoConfig := ctx.Config.GetRepoConfig(ctx.Issue.Org, ctx.Issue.Repo)
	if repoConfig == nil || !repoConfig.Enabled {
		ctx.Result.Skipped = true
		ctx.SkipReason = "repository not enabled"
		return core.ErrSkipPipeline
	}

	// 2. Check cooldown
	skip, err := s.gh.ShouldSkipComment(ctx.Ctx, ctx.Issue.Org, ctx.Issue.Repo, ctx.Issue.Number, ctx.Config.Defaults.CommentCooldownHours)
	if err != nil {
		return fmt.Errorf("failed to check cooldown: %w", err)
	}

	if skip {
		ctx.Result.Skipped = true
		ctx.SkipReason = "cooldown active"
		return core.ErrSkipPipeline
	}

	return nil
}
