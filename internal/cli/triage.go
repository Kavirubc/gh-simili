package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Kavirubc/gh-simili/internal/config"
	"github.com/Kavirubc/gh-simili/internal/embedding"
	"github.com/Kavirubc/gh-simili/internal/github"
	"github.com/Kavirubc/gh-simili/internal/llm"
	"github.com/Kavirubc/gh-simili/internal/processor"
	"github.com/Kavirubc/gh-simili/internal/triage"
	"github.com/Kavirubc/gh-simili/internal/vectordb"
	"github.com/Kavirubc/gh-simili/pkg/models"
	"github.com/spf13/cobra"
)

func newTriageCmd() *cobra.Command {
	var (
		eventPath  string
		outputPath string
		execute    bool
	)

	cmd := &cobra.Command{
		Use:   "triage",
		Short: "Triage a new issue with AI-powered analysis",
		Long: `Analyze a GitHub issue and suggest labels, detect quality issues,
and identify duplicates. Can output actions to JSON for multi-job workflows
or execute actions directly.`,
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

			if !cfg.Triage.Enabled {
				return fmt.Errorf("triage is not enabled in config")
			}

			// Parse the event to get the issue
			event, err := github.ParseEventFile(eventPath)
			if err != nil {
				return fmt.Errorf("failed to parse event: %w", err)
			}

			if !event.IsIssueEvent() || !event.IsOpenedEvent() {
				fmt.Println("Skipped: not an issue opened event")
				return nil
			}

			issue := event.ToIssue()
			if issue == nil {
				return fmt.Errorf("failed to extract issue from event")
			}

			// Create LLM provider
			llmProvider, err := createLLMProvider(&cfg.Triage.LLM)
			if err != nil {
				return fmt.Errorf("failed to create LLM provider: %w", err)
			}
			defer llmProvider.Close()

			// Create similarity finder
			embedder, err := embedding.NewFallbackProvider(&cfg.Embedding)
			if err != nil {
				return fmt.Errorf("failed to create embedder: %w", err)
			}
			defer embedder.Close()

			vdb, err := vectordb.NewClient(&cfg.Qdrant)
			if err != nil {
				return fmt.Errorf("failed to create vector DB client: %w", err)
			}
			defer vdb.Close()

			similarity := processor.NewSimilarityFinder(cfg, embedder, vdb)

			// Create GitHub client for delayed actions
			ghClient, err := github.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create GitHub client: %w", err)
			}
			agent := triage.NewAgentWithGitHub(cfg, llmProvider, similarity, ghClient)

			// Run triage
			fmt.Printf("Triaging issue #%d: %s\n", issue.Number, issue.Title)
			result, err := agent.Triage(ctx, issue)
			if err != nil {
				return fmt.Errorf("triage failed: %w", err)
			}

			// Output results
			printTriageResult(result)

			// Write output file if specified
			if outputPath != "" {
				if err := triage.WriteOutput(result, outputPath); err != nil {
					return fmt.Errorf("failed to write output: %w", err)
				}
				fmt.Printf("Output written to: %s\n", outputPath)
			}

			// Execute actions if requested
			if execute && !dryRun {
				executor := triage.NewExecutor(ghClient, dryRun)
				if err := executor.Execute(ctx, issue, result); err != nil {
					return fmt.Errorf("failed to execute actions: %w", err)
				}
				fmt.Println("Actions executed successfully")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&eventPath, "event-path", "", "path to GitHub event JSON file")
	cmd.Flags().StringVar(&outputPath, "output", "", "path to write triage output JSON")
	cmd.Flags().BoolVar(&execute, "execute", false, "execute actions (default: analyze only)")
	_ = cmd.MarkFlagRequired("event-path")

	return cmd
}

func createLLMProvider(cfg *config.LLMConfig) (llm.Provider, error) {
	switch cfg.Provider {
	case "gemini":
		return llm.NewGeminiProvider(cfg.APIKey, cfg.Model)
	case "openai":
		return llm.NewOpenAIProvider(cfg.APIKey, cfg.Model)
	default:
		return nil, fmt.Errorf("unknown LLM provider: %s", cfg.Provider)
	}
}

func printTriageResult(result *triage.Result) {
	fmt.Println("\n=== Triage Result ===")

	if len(result.Labels) > 0 {
		fmt.Println("\nLabels:")
		for _, l := range result.Labels {
			fmt.Printf("  - %s (%.0f%% confidence, %s)\n", l.Label, l.Confidence*100, l.Reason)
		}
	}

	if result.Quality != nil {
		fmt.Printf("\nQuality Score: %.0f%%\n", result.Quality.Score*100)
		if len(result.Quality.Missing) > 0 {
			fmt.Printf("  Missing: %v\n", result.Quality.Missing)
		}
	}

	if result.Duplicate != nil && result.Duplicate.IsDuplicate {
		fmt.Printf("\nDuplicate Detected (%.0f%% similarity)\n", result.Duplicate.Similarity*100)
		if result.Duplicate.Original != nil {
			fmt.Printf("  Original: #%d - %s\n", result.Duplicate.Original.Number, result.Duplicate.Original.Title)
		}
		fmt.Printf("  Auto-close: %v\n", result.Duplicate.ShouldClose)
	}

	if len(result.Actions) > 0 {
		fmt.Println("\nActions:")
		for _, a := range result.Actions {
			switch a.Type {
			case triage.ActionAddLabel:
				fmt.Printf("  - Add label: %s\n", a.Label)
			case triage.ActionRemoveLabel:
				fmt.Printf("  - Remove label: %s\n", a.Label)
			case triage.ActionComment:
				fmt.Printf("  - Post comment (%d chars)\n", len(a.Comment))
			case triage.ActionClose:
				fmt.Printf("  - Close issue\n")
			}
		}
	}
}

// newTriageExecuteCmd creates a command to execute pre-computed triage actions
func newTriageExecuteCmd() *cobra.Command {
	var (
		inputPath string
		issueJSON string
	)

	cmd := &cobra.Command{
		Use:   "triage-execute",
		Short: "Execute pre-computed triage actions",
		Long:  `Execute actions from a previously generated triage output file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			result, err := triage.ReadOutput(inputPath)
			if err != nil {
				return fmt.Errorf("failed to read triage output: %w", err)
			}

			// Parse issue context
			var issue models.Issue
			data, err := os.ReadFile(issueJSON)
			if err != nil {
				return fmt.Errorf("failed to read issue JSON: %w", err)
			}
			if err := json.Unmarshal(data, &issue); err != nil {
				return fmt.Errorf("failed to parse issue JSON: %w", err)
			}

			ghClient, err := github.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create GitHub client: %w", err)
			}

			executor := triage.NewExecutor(ghClient, dryRun)
			if err := executor.Execute(ctx, &issue, result); err != nil {
				return fmt.Errorf("failed to execute actions: %w", err)
			}

			fmt.Printf("Executed %d actions\n", len(result.Actions))
			return nil
		},
	}

	cmd.Flags().StringVar(&inputPath, "input", "", "path to triage output JSON")
	cmd.Flags().StringVar(&issueJSON, "issue", "", "path to issue context JSON")
	_ = cmd.MarkFlagRequired("input")
	_ = cmd.MarkFlagRequired("issue")

	return cmd
}
