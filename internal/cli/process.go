package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/Kavirubc/gh-simili/internal/config"
	"github.com/Kavirubc/gh-simili/internal/pipeline"
	"github.com/spf13/cobra"
)

func newProcessCmd() *cobra.Command {
	var execute bool
	cmd := &cobra.Command{
		Use:   "process",
		Short: "Process a single issue from GitHub Action event",
		Long:  `Process a single issue event (opened, edited, closed) using the unified pipeline.`,
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

			// Use separate transfer token if provided (for elevated permissions)
			transferToken := os.Getenv("TRANSFER_TOKEN")

			// Process command defaults to execute=true for backward compatibility if typical usage implies it,
			// BUT 'full-process' required --execute flag.
			// The old 'process' command didn't have --execute flag, it just ran.
			// However, in the legacy code `NewProcessor` accepted `dryRun`.
			// `execute` param in UnifiedProcessor controls side-effects.
			// To match old behavior where it DID post comments/transfers:
			// We should set execute = true unless dry-run is set?
			// Wait, old `process.go` didn't have `execute` flag. It relied on `dryRun`.
			// So we should set execute=true (implied) but dryRun handles the safety.

			execute = true // Implicit execution for legacy command compatibility

			proc, err := pipeline.NewUnifiedProcessorWithTransferToken(cfg, dryRun, execute, transferToken)
			if err != nil {
				return fmt.Errorf("failed to create processor: %w", err)
			}
			defer proc.Close()

			result, err := proc.ProcessEvent(ctx, eventPath)
			if err != nil {
				return fmt.Errorf("processing failed: %w", err)
			}

			pipeline.PrintUnifiedResult(result)
			return nil
		},
	}

	_ = cmd.MarkPersistentFlagRequired("event-path")

	return cmd
}
