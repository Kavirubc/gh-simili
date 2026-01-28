package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/Kavirubc/gh-simili/internal/config"
	"github.com/Kavirubc/gh-simili/internal/pipeline"
	"github.com/spf13/cobra"
)

func newFullProcessCmd() *cobra.Command {
	var (
		execute bool
	)

	cmd := &cobra.Command{
		Use:   "full-process",
		Short: "Process a new issue through the unified pipeline",
		Long: `Process a new issue through the complete unified pipeline:
1. Find similar issues using semantic search
2. Run triage analysis (labels, quality, duplicates)
3. Check and apply transfer rules (with delayed actions if enabled)
4. Index the issue in the vector database

This command combines 'process' and 'triage' into a single coherent flow,
avoiding duplicate comments and ensuring proper execution order.

Use --execute to apply actions (labels, comments, transfers, closes).
Without --execute, only analysis is performed (read-only mode).`,
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

			// Use separate transfer token if provided
			transferToken := os.Getenv("TRANSFER_TOKEN")
			proc, err := pipeline.NewUnifiedProcessorWithTransferToken(cfg, dryRun, execute, transferToken)
			if err != nil {
				return fmt.Errorf("failed to create processor: %w", err)
			}
			defer proc.Close()

			result, err := proc.ProcessEvent(ctx, eventPath)
			if err != nil {
				return fmt.Errorf("processing failed: %w", err)
			}

			// Print result summary
			pipeline.PrintUnifiedResult(result)

			if result.Skipped {
				fmt.Printf("\nSkipped: %s\n", result.SkipReason)
				return nil
			}

			// Summary
			fmt.Println("\n--- Summary ---")
			if len(result.SimilarFound) > 0 {
				fmt.Printf("✓ Found %d similar issues\n", len(result.SimilarFound))
			}
			if result.TriageResult != nil {
				if len(result.TriageResult.Labels) > 0 {
					fmt.Printf("✓ Suggested %d labels\n", len(result.TriageResult.Labels))
				}
				if result.TriageResult.Duplicate != nil && result.TriageResult.Duplicate.IsDuplicate {
					fmt.Printf("⚠ Detected as duplicate (%.0f%% similar)\n", result.TriageResult.Duplicate.Similarity*100)
				}
			}
			if result.TransferTarget != "" {
				if result.Transferred {
					fmt.Printf("✓ Transferred to %s\n", result.TransferTarget)
				} else {
					fmt.Printf("→ Would transfer to %s\n", result.TransferTarget)
				}
			}
			if result.CommentPosted {
				fmt.Println("✓ Comment posted")
			}
			if result.Indexed {
				fmt.Println("✓ Issue indexed")
			}
			if result.ActionsExecuted > 0 {
				fmt.Printf("✓ Executed %d actions\n", result.ActionsExecuted)
			}

			if !execute && !dryRun {
				fmt.Println("\n💡 Tip: Use --execute to apply actions")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&execute, "execute", false, "execute actions (labels, comments, transfers, closes)")
	_ = cmd.MarkPersistentFlagRequired("event-path")

	return cmd
}
