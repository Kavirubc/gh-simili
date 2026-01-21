package cli

import (
	"context"
	"fmt"

	"github.com/kaviruhapuarachchi/gh-simili/internal/config"
	"github.com/kaviruhapuarachchi/gh-simili/internal/processor"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	var (
		repo  string
		since string
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync issue updates (closed, edited, deleted)",
		Long:  `Synchronize vector database with recent issue changes.`,
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

			syncer, err := processor.NewSyncer(cfg, dryRun)
			if err != nil {
				return fmt.Errorf("failed to create syncer: %w", err)
			}
			defer syncer.Close()

			stats, err := syncer.SyncRepo(ctx, repo, since)
			if err != nil {
				return fmt.Errorf("sync failed: %w", err)
			}

			fmt.Printf("Synced %d issues (%d updated, %d skipped) in %dms\n",
				stats.TotalIssues, stats.Indexed, stats.Skipped, stats.DurationMs)

			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "repository to sync (owner/repo)")
	cmd.Flags().StringVar(&since, "since", "24h", "sync issues updated since (e.g., 24h, 7d)")
	_ = cmd.MarkFlagRequired("repo")

	return cmd
}
