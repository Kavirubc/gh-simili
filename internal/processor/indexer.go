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

// Indexer handles bulk indexing of issues
type Indexer struct {
	cfg      *config.Config
	gh       *github.Client
	embedder *embedding.FallbackProvider
	vdb      *vectordb.Client
	dryRun   bool
}

// NewIndexer creates a new bulk indexer
func NewIndexer(cfg *config.Config, dryRun bool) (*Indexer, error) {
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

	return &Indexer{
		cfg:      cfg,
		gh:       gh,
		embedder: embedder,
		vdb:      vdb,
		dryRun:   dryRun,
	}, nil
}

// Close releases resources
func (idx *Indexer) Close() error {
	idx.embedder.Close()
	return idx.vdb.Close()
}

// IndexRepo indexes all issues from a repository
func (idx *Indexer) IndexRepo(ctx context.Context, fullRepo string, batchSize int) (*models.IndexStats, error) {
	start := time.Now()
	stats := &models.IndexStats{}

	org, repo, err := github.ParseRepo(fullRepo)
	if err != nil {
		return nil, err
	}

	// Ensure collection exists
	collection := vectordb.CollectionName(org)
	if !idx.dryRun {
		if err := idx.vdb.EnsureCollection(ctx, collection); err != nil {
			return nil, fmt.Errorf("failed to ensure collection: %w", err)
		}
	}

	// Fetch all issues
	fmt.Printf("Fetching issues from %s...\n", fullRepo)
	issues, err := idx.gh.ListAllIssues(ctx, org, repo, "all", batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch issues: %w", err)
	}
	stats.TotalIssues = len(issues)
	fmt.Printf("Found %d issues\n", len(issues))

	// Process in batches
	for i := 0; i < len(issues); i += batchSize {
		end := i + batchSize
		if end > len(issues) {
			end = len(issues)
		}
		batch := issues[i:end]

		if err := idx.indexBatch(ctx, collection, batch); err != nil {
			fmt.Printf("Warning: batch %d-%d failed: %v\n", i, end, err)
			stats.Errors += len(batch)
			continue
		}

		stats.Indexed += len(batch)
		fmt.Printf("Indexed %d/%d issues\n", stats.Indexed, stats.TotalIssues)
	}

	stats.DurationMs = int(time.Since(start).Milliseconds())
	return stats, nil
}

// indexBatch processes and indexes a batch of issues
func (idx *Indexer) indexBatch(ctx context.Context, collection string, issues []*models.Issue) error {
	// Prepare texts for embedding
	texts := make([]string, len(issues))
	for i, issue := range issues {
		texts[i] = embedding.PrepareIssueText(issue.Title, issue.Body)
	}

	// Generate embeddings
	vectors, err := idx.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}

	if idx.dryRun {
		return nil
	}

	// Upsert to Qdrant
	if err := idx.vdb.UpsertBatch(ctx, collection, issues, vectors); err != nil {
		return fmt.Errorf("failed to upsert batch: %w", err)
	}

	return nil
}

// IndexSingleIssue indexes a single issue
func (idx *Indexer) IndexSingleIssue(ctx context.Context, issue *models.Issue) error {
	collection := vectordb.CollectionName(issue.Org)

	text := embedding.PrepareIssueText(issue.Title, issue.Body)
	vector, err := idx.embedder.Embed(ctx, text)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	if idx.dryRun {
		return nil
	}

	if err := idx.vdb.Upsert(ctx, collection, issue, vector); err != nil {
		return fmt.Errorf("failed to upsert issue: %w", err)
	}

	return nil
}

// DeleteIssue removes an issue from the index
func (idx *Indexer) DeleteIssue(ctx context.Context, org, repo string, number int) error {
	if idx.dryRun {
		return nil
	}

	collection := vectordb.CollectionName(org)
	id := models.IssueUUID(org, repo, number)
	return idx.vdb.Delete(ctx, collection, id)
}
