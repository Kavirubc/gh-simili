package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	dryRun  bool
	version = "dev"
)

var rootCmd = &cobra.Command{
	Use:   "gh-simili",
	Short: "GitHub Issue Intelligence Bot",
	Long: `gh-simili auto-transfers issues to the correct repository based on
classification rules and detects duplicate/similar issues using semantic search.

Uses Gemini embeddings + Qdrant vector DB for similarity detection.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "skip all writes (GitHub + Qdrant)")

	rootCmd.AddCommand(newIndexCmd())
	rootCmd.AddCommand(newProcessCmd())
	rootCmd.AddCommand(newSyncCmd())
	rootCmd.AddCommand(newSearchCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newTriageCmd())
	rootCmd.AddCommand(newTriageExecuteCmd())
	rootCmd.AddCommand(newVersionCmd())
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("gh-simili version %s\n", version)
		},
	}
}
