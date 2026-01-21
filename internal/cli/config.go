package cli

import (
	"fmt"

	"github.com/kaviruhapuarachchi/gh-simili/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management commands",
	}

	cmd.AddCommand(newConfigValidateCmd())
	return cmd
}

func newConfigValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := config.FindConfigPath(cfgFile)
			if cfgPath == "" {
				return fmt.Errorf("config file not found")
			}

			fmt.Printf("Validating config: %s\n", cfgPath)

			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			errs := config.Validate(cfg)
			if len(errs) > 0 {
				fmt.Println("\nValidation errors:")
				for _, e := range errs {
					fmt.Printf("  - %v\n", e)
				}
				return fmt.Errorf("configuration is invalid")
			}

			fmt.Println("\nConfiguration is valid!")
			fmt.Printf("  - Qdrant URL: %s\n", cfg.Qdrant.URL)
			fmt.Printf("  - Primary embedding: %s (%s)\n", cfg.Embedding.Primary.Provider, cfg.Embedding.Primary.Model)
			fmt.Printf("  - Repositories: %d configured\n", len(cfg.Repositories))

			totalRules := 0
			for _, repo := range cfg.Repositories {
				totalRules += len(repo.TransferRules)
			}
			fmt.Printf("  - Transfer rules: %d total\n", totalRules)

			return nil
		},
	}
}
