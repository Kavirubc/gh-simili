package cli

import (
	"context"
	"fmt"

	"github.com/kaviruhapuarachchi/gh-simili/internal/config"
	"github.com/kaviruhapuarachchi/gh-simili/internal/processor"
	"github.com/spf13/cobra"
)

func newIndexCmd() *cobra.Command {
	var (
		repo      string
		batchSize int
	)

	cmd := &cobra.Command{
		Use:   "index",
		Short: "Bulk index existing issues from a repository",
		Long:  `Index all existing issues from a repository into the vector database for similarity search.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			cfgPath := config.FindConfigPath(cfgFile)
			if cfgPath == "" {
				return fmt.Errorf("config file not found")
			}

			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if errs := config.Validate(cfg); len(errs) > 0 {
				for _, e := range errs {
					fmt.Printf("config error: %v\n", e)
				}
				return fmt.Errorf("invalid configuration")
			}

			indexer, err := processor.NewIndexer(cfg, dryRun)
			if err != nil {
				return fmt.Errorf("failed to create indexer: %w", err)
			}
			defer indexer.Close()

			stats, err := indexer.IndexRepo(ctx, repo, batchSize)
			if err != nil {
				return fmt.Errorf("indexing failed: %w", err)
			}

			fmt.Printf("Indexed %d/%d issues (%d skipped, %d errors) in %dms\n",
				stats.Indexed, stats.TotalIssues, stats.Skipped, stats.Errors, stats.DurationMs)

			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "repository to index (owner/repo)")
	cmd.Flags().IntVar(&batchSize, "batch-size", 100, "number of issues to fetch per batch")
	_ = cmd.MarkFlagRequired("repo")

	return cmd
}
