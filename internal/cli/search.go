package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/kaviruhapuarachchi/gh-simili/internal/config"
	"github.com/kaviruhapuarachchi/gh-simili/internal/processor"
	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	var (
		repo  string
		limit int
	)

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search for similar issues (debugging/testing)",
		Long:  `Interactively search for similar issues using semantic similarity.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			query := args[0]

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

			searcher, err := processor.NewSearcher(cfg)
			if err != nil {
				return fmt.Errorf("failed to create searcher: %w", err)
			}
			defer searcher.Close()

			// Parse org from repo if provided
			org := ""
			if repo != "" {
				parts := strings.Split(repo, "/")
				if len(parts) == 2 {
					org = parts[0]
				}
			}

			results, err := searcher.Search(ctx, query, org, limit)
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}

			if len(results) == 0 {
				fmt.Println("No similar issues found")
				return nil
			}

			fmt.Printf("Found %d similar issues:\n\n", len(results))
			for i, r := range results {
				status := "Open"
				if r.Issue.State == "closed" {
					status = "Closed"
				}
				fmt.Printf("%d. #%d - %s\n", i+1, r.Issue.Number, r.Issue.Title)
				fmt.Printf("   Repo: %s/%s | Similarity: %.1f%% | Status: %s\n",
					r.Issue.Org, r.Issue.Repo, r.Score*100, status)
				fmt.Printf("   %s\n\n", r.Issue.URL)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "limit search to repository (owner/repo)")
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum results to return")

	return cmd
}
