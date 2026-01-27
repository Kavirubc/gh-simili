package cli

import (
	"context"
	"fmt"

	"github.com/Kavirubc/gh-simili/internal/config"
	"github.com/Kavirubc/gh-simili/internal/github"
	"github.com/Kavirubc/gh-simili/internal/pending"
	"github.com/Kavirubc/gh-simili/internal/transfer"
	"github.com/Kavirubc/gh-simili/internal/triage"
	"github.com/Kavirubc/gh-simili/internal/vectordb"
	"github.com/spf13/cobra"
)

func newProcessPendingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "process-pending",
		Short: "Process expired pending actions (transfers and closes)",
		Long:  `Processes pending actions that have expired and checks for user reactions to determine if actions should execute or be cancelled.`,
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

			if !cfg.Defaults.DelayedActions.Enabled {
				fmt.Println("Delayed actions are disabled in config")
				return nil
			}

			// Create clients
			gh, err := github.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create GitHub client: %w", err)
			}

			vdb, err := vectordb.NewClient(&cfg.Qdrant)
			if err != nil {
				return fmt.Errorf("failed to create vector DB client: %w", err)
			}
			defer vdb.Close()

			// Create pending manager once (reused for all repos)
			pendingMgr := pending.NewManager(gh, cfg)

			// Process each repository
			processedCount := 0
			for _, repoConfig := range cfg.Repositories {
				if !repoConfig.Enabled {
					continue
				}

				fmt.Printf("Processing pending actions for %s/%s...\n", repoConfig.Org, repoConfig.Repo)

				// Find pending actions
				actions, err := pendingMgr.FindPendingActions(ctx, repoConfig.Org, repoConfig.Repo)
				if err != nil {
					fmt.Printf("Warning: failed to find pending actions: %v\n", err)
					continue
				}

				// Process each action
				for _, action := range actions {
					if !action.IsExpired() {
						continue // Not expired yet
					}

					fmt.Printf("Processing %s action for issue #%d...\n", action.Type, action.IssueNumber)

					switch action.Type {
					case pending.ActionTypeTransfer:
						executor := transfer.NewExecutor(gh, gh, vdb, cfg, dryRun)
						if err := executor.ProcessPendingTransfer(ctx, action); err != nil {
							fmt.Printf("Error processing transfer: %v\n", err)
							continue
						}
						processedCount++

					case pending.ActionTypeClose:
						duplicateChecker := triage.NewDuplicateCheckerWithDelayedActionsAndDryRun(&cfg.Triage.Duplicate, gh, cfg, dryRun)
						if err := duplicateChecker.ProcessPendingClose(ctx, action); err != nil {
							fmt.Printf("Error processing close: %v\n", err)
							continue
						}
						processedCount++
					}
				}
			}

			fmt.Printf("Processed %d pending actions\n", processedCount)
			return nil
		},
	}

	return cmd
}
