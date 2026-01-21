package processor

import (
	"context"
	"fmt"

	"github.com/kaviruhapuarachchi/gh-simili/internal/config"
	"github.com/kaviruhapuarachchi/gh-simili/internal/embedding"
	"github.com/kaviruhapuarachchi/gh-simili/internal/github"
	"github.com/kaviruhapuarachchi/gh-simili/internal/transfer"
	"github.com/kaviruhapuarachchi/gh-simili/internal/vectordb"
	"github.com/kaviruhapuarachchi/gh-simili/pkg/models"
)

// Processor handles single issue event processing
type Processor struct {
	cfg        *config.Config
	gh         *github.Client
	embedder   *embedding.FallbackProvider
	vdb        *vectordb.Client
	similarity *SimilarityFinder
	indexer    *Indexer
	dryRun     bool
}

// NewProcessor creates a new event processor
func NewProcessor(cfg *config.Config, dryRun bool) (*Processor, error) {
	gh, err := github.NewClient()
	if err != nil {
		return nil, err
	}

	embedder, err := embedding.NewFallbackProvider(&cfg.Embedding)
	if err != nil {
		return nil, err
	}

	vdb, err := vectordb.NewClient(&cfg.Qdrant)
	if err != nil {
		return nil, err
	}

	indexer, err := NewIndexer(cfg, dryRun)
	if err != nil {
		return nil, err
	}

	return &Processor{
		cfg:        cfg,
		gh:         gh,
		embedder:   embedder,
		vdb:        vdb,
		similarity: NewSimilarityFinder(cfg, embedder, vdb),
		indexer:    indexer,
		dryRun:     dryRun,
	}, nil
}

// Close releases resources
func (p *Processor) Close() error {
	p.embedder.Close()
	p.indexer.Close()
	return p.vdb.Close()
}

// ProcessEvent processes a GitHub Action event
func (p *Processor) ProcessEvent(ctx context.Context, eventPath string) (*models.ProcessResult, error) {
	event, err := github.ParseEventFile(eventPath)
	if err != nil {
		return nil, err
	}

	if !event.IsIssueEvent() {
		return &models.ProcessResult{
			Skipped:    true,
			SkipReason: "not an issue event",
		}, nil
	}

	issue := event.ToIssue()
	if issue == nil {
		return nil, fmt.Errorf("failed to parse issue from event")
	}

	// Check if repo is enabled
	repoConfig := p.cfg.GetRepoConfig(issue.Org, issue.Repo)
	if repoConfig == nil || !repoConfig.Enabled {
		return &models.ProcessResult{
			IssueNumber: issue.Number,
			Skipped:     true,
			SkipReason:  "repository not enabled",
		}, nil
	}

	// Route based on action
	switch {
	case event.IsOpenedEvent():
		return p.processOpened(ctx, issue, repoConfig)
	case event.IsEditedEvent():
		return p.processEdited(ctx, issue)
	case event.IsClosedEvent():
		return p.processClosed(ctx, issue)
	case event.IsReopenedEvent():
		return p.processReopened(ctx, issue)
	case event.IsDeletedEvent():
		return p.processDeleted(ctx, issue)
	default:
		return &models.ProcessResult{
			IssueNumber: issue.Number,
			Skipped:     true,
			SkipReason:  fmt.Sprintf("unhandled action: %s", event.Action),
		}, nil
	}
}

// processOpened handles new issues
func (p *Processor) processOpened(ctx context.Context, issue *models.Issue, repoConfig *config.RepositoryConfig) (*models.ProcessResult, error) {
	result := &models.ProcessResult{IssueNumber: issue.Number}

	// Check cooldown
	skip, err := p.gh.ShouldSkipComment(ctx, issue.Org, issue.Repo, issue.Number, p.cfg.Defaults.CommentCooldownHours)
	if err != nil {
		return nil, fmt.Errorf("failed to check cooldown: %w", err)
	}
	if skip {
		result.Skipped = true
		result.SkipReason = "cooldown active"
		return result, nil
	}

	// Ensure collection exists
	collection := vectordb.CollectionName(issue.Org)
	if !p.dryRun {
		if err := p.vdb.EnsureCollection(ctx, collection); err != nil {
			return nil, fmt.Errorf("failed to ensure collection: %w", err)
		}
	}

	// Find similar issues
	similar, err := p.similarity.FindSimilar(ctx, issue, true)
	if err != nil {
		fmt.Printf("Warning: similarity search failed: %v\n", err)
	} else if len(similar) > 0 {
		result.SimilarFound = make([]models.SearchResult, len(similar))
		for i, s := range similar {
			result.SimilarFound[i] = models.SearchResult{Issue: s.Issue, Score: s.Score}
		}

		// Post similarity comment
		if !p.dryRun {
			crossRepo := HasCrossRepoResults(similar, issue.Org, issue.Repo)
			comment := FormatSimilarityComment(similar, crossRepo)
			if err := p.gh.PostComment(ctx, issue.Org, issue.Repo, issue.Number, comment); err != nil {
				fmt.Printf("Warning: failed to post similarity comment: %v\n", err)
			} else {
				result.CommentPosted = true
			}
		}
	}

	// Check transfer rules
	if len(repoConfig.TransferRules) > 0 {
		matcher := transfer.NewRuleMatcher(repoConfig.TransferRules)
		if target, rule := matcher.Match(issue); target != "" {
			executor := transfer.NewExecutor(p.gh, p.vdb, p.dryRun)
			if err := executor.Transfer(ctx, issue, target, rule); err != nil {
				return nil, fmt.Errorf("failed to transfer issue: %w", err)
			}
			result.Transferred = true
			result.TransferTarget = target
			return result, nil // Don't index transferred issues
		}
	}

	// Index the issue
	if err := p.indexer.IndexSingleIssue(ctx, issue); err != nil {
		return nil, fmt.Errorf("failed to index issue: %w", err)
	}

	return result, nil
}

// processEdited handles edited issues
func (p *Processor) processEdited(ctx context.Context, issue *models.Issue) (*models.ProcessResult, error) {
	// Re-index with updated content
	if err := p.indexer.IndexSingleIssue(ctx, issue); err != nil {
		return nil, fmt.Errorf("failed to update index: %w", err)
	}

	return &models.ProcessResult{IssueNumber: issue.Number}, nil
}

// processClosed handles closed issues
func (p *Processor) processClosed(ctx context.Context, issue *models.Issue) (*models.ProcessResult, error) {
	// Update state in index
	issue.State = "closed"
	if err := p.indexer.IndexSingleIssue(ctx, issue); err != nil {
		return nil, fmt.Errorf("failed to update index: %w", err)
	}

	return &models.ProcessResult{IssueNumber: issue.Number}, nil
}

// processReopened handles reopened issues
func (p *Processor) processReopened(ctx context.Context, issue *models.Issue) (*models.ProcessResult, error) {
	// Update state in index
	issue.State = "open"
	if err := p.indexer.IndexSingleIssue(ctx, issue); err != nil {
		return nil, fmt.Errorf("failed to update index: %w", err)
	}

	return &models.ProcessResult{IssueNumber: issue.Number}, nil
}

// processDeleted handles deleted issues
func (p *Processor) processDeleted(ctx context.Context, issue *models.Issue) (*models.ProcessResult, error) {
	if err := p.indexer.DeleteIssue(ctx, issue.Org, issue.Repo, issue.Number); err != nil {
		return nil, fmt.Errorf("failed to delete from index: %w", err)
	}

	return &models.ProcessResult{IssueNumber: issue.Number}, nil
}
