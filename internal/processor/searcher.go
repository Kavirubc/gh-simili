package processor

import (
	"context"

	"github.com/kaviruhapuarachchi/gh-simili/internal/config"
	"github.com/kaviruhapuarachchi/gh-simili/internal/embedding"
	"github.com/kaviruhapuarachchi/gh-simili/internal/vectordb"
	"github.com/kaviruhapuarachchi/gh-simili/pkg/models"
)

// Searcher handles interactive similarity searches
type Searcher struct {
	cfg      *config.Config
	embedder *embedding.FallbackProvider
	vdb      *vectordb.Client
}

// NewSearcher creates a new searcher
func NewSearcher(cfg *config.Config) (*Searcher, error) {
	embedder, err := embedding.NewFallbackProvider(&cfg.Embedding)
	if err != nil {
		return nil, err
	}

	vdb, err := vectordb.NewClient(&cfg.Qdrant)
	if err != nil {
		return nil, err
	}

	return &Searcher{
		cfg:      cfg,
		embedder: embedder,
		vdb:      vdb,
	}, nil
}

// Close releases resources
func (s *Searcher) Close() error {
	s.embedder.Close()
	return s.vdb.Close()
}

// Search finds similar issues for a query
func (s *Searcher) Search(ctx context.Context, query string, org string, limit int) ([]models.SearchResult, error) {
	// If no org specified, use first configured repo's org
	if org == "" && len(s.cfg.Repositories) > 0 {
		org = s.cfg.Repositories[0].Org
	}

	finder := NewSimilarityFinder(s.cfg, s.embedder, s.vdb)
	results, err := finder.FindSimilarByText(ctx, query, org, limit)
	if err != nil {
		return nil, err
	}

	// Convert to models.SearchResult
	modelResults := make([]models.SearchResult, len(results))
	for i, r := range results {
		modelResults[i] = models.SearchResult{
			Issue: r.Issue,
			Score: r.Score,
		}
	}

	return modelResults, nil
}
