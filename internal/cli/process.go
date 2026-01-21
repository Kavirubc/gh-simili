package cli

import (
	"context"
	"fmt"

	"github.com/kaviruhapuarachchi/gh-simili/internal/config"
	"github.com/kaviruhapuarachchi/gh-simili/internal/processor"
	"github.com/spf13/cobra"
)

func newProcessCmd() *cobra.Command {
	var eventPath string

	cmd := &cobra.Command{
		Use:   "process",
		Short: "Process a single issue from GitHub Action event",
		Long:  `Process a single issue event (opened, edited, closed) for similarity detection and transfer rules.`,
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

			proc, err := processor.NewProcessor(cfg, dryRun)
			if err != nil {
				return fmt.Errorf("failed to create processor: %w", err)
			}
			defer proc.Close()

			result, err := proc.ProcessEvent(ctx, eventPath)
			if err != nil {
				return fmt.Errorf("processing failed: %w", err)
			}

			if result.Skipped {
				fmt.Printf("Skipped: %s\n", result.SkipReason)
				return nil
			}

			if len(result.SimilarFound) > 0 {
				fmt.Printf("Found %d similar issues\n", len(result.SimilarFound))
			}

			if result.CommentPosted {
				fmt.Println("Posted similarity comment")
			}

			if result.Transferred {
				fmt.Printf("Transferred to %s\n", result.TransferTarget)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&eventPath, "event-path", "", "path to GitHub event JSON file")
	_ = cmd.MarkFlagRequired("event-path")

	return cmd
}
