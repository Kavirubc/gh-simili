package processor

import (
	"context"
	"fmt"
	"time"

	"github.com/kaviruhapuarachchi/gh-simili/internal/config"
	"github.com/kaviruhapuarachchi/gh-simili/internal/embedding"
	"github.com/kaviruhapuarachchi/gh-simili/internal/github"
	"github.com/kaviruhapuarachchi/gh-simili/internal/vectordb"
	"github.com/kaviruhapuarachchi/gh-simili/pkg/models"
)

// Syncer handles syncing issue updates
type Syncer struct {
	cfg      *config.Config
	gh       *github.Client
	embedder *embedding.FallbackProvider
	vdb      *vectordb.Client
	indexer  *Indexer
	dryRun   bool
}

// NewSyncer creates a new syncer
func NewSyncer(cfg *config.Config, dryRun bool) (*Syncer, error) {
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

	return &Syncer{
		cfg:      cfg,
		gh:       gh,
		embedder: embedder,
		vdb:      vdb,
		indexer:  indexer,
		dryRun:   dryRun,
	}, nil
}

// Close releases resources
func (s *Syncer) Close() error {
	s.embedder.Close()
	s.indexer.Close()
	return s.vdb.Close()
}

// SyncRepo syncs issues updated since a given duration
func (s *Syncer) SyncRepo(ctx context.Context, fullRepo string, sinceDuration string) (*models.IndexStats, error) {
	start := time.Now()
	stats := &models.IndexStats{}

	org, repo, err := github.ParseRepo(fullRepo)
	if err != nil {
		return nil, err
	}

	// Parse duration
	since, err := parseSinceDuration(sinceDuration)
	if err != nil {
		return nil, fmt.Errorf("invalid since duration: %w", err)
	}

	// Ensure collection exists
	collection := vectordb.CollectionName(org)
	if !s.dryRun {
		if err := s.vdb.EnsureCollection(ctx, collection); err != nil {
			return nil, fmt.Errorf("failed to ensure collection: %w", err)
		}
	}

	// Fetch recently updated issues
	fmt.Printf("Fetching issues updated since %s...\n", since.Format(time.RFC3339))
	issues, err := s.gh.ListIssues(ctx, org, repo, github.ListOptions{
		State: "all",
		Since: since,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch issues: %w", err)
	}
	stats.TotalIssues = len(issues)
	fmt.Printf("Found %d updated issues\n", len(issues))

	// Process each issue
	for _, issue := range issues {
		if err := s.indexer.IndexSingleIssue(ctx, issue); err != nil {
			fmt.Printf("Warning: failed to sync issue #%d: %v\n", issue.Number, err)
			stats.Errors++
			continue
		}
		stats.Indexed++
	}

	stats.DurationMs = int(time.Since(start).Milliseconds())
	return stats, nil
}

// parseSinceDuration parses duration strings like "24h", "7d"
func parseSinceDuration(s string) (time.Time, error) {
	// Handle day suffix
	if len(s) > 1 && s[len(s)-1] == 'd' {
		days := s[:len(s)-1]
		d, err := time.ParseDuration(days + "h")
		if err != nil {
			return time.Time{}, err
		}
		return time.Now().Add(-d * 24), nil
	}

	// Standard duration
	d, err := time.ParseDuration(s)
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().Add(-d), nil
}
