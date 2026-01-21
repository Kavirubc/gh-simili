package config

import (
	"os"
	"regexp"
)

var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// expandEnvVars replaces ${VAR_NAME} patterns with environment variable values
func expandEnvVars(s string) string {
	return envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		// Extract variable name from ${VAR_NAME}
		varName := envVarPattern.FindStringSubmatch(match)[1]
		if value := os.Getenv(varName); value != "" {
			return value
		}
		return match // Keep original if env var not set
	})
}

// expandConfigEnvVars expands environment variables in config string fields
func expandConfigEnvVars(cfg *Config) {
	cfg.Qdrant.URL = expandEnvVars(cfg.Qdrant.URL)
	cfg.Qdrant.APIKey = expandEnvVars(cfg.Qdrant.APIKey)
	cfg.Embedding.Primary.APIKey = expandEnvVars(cfg.Embedding.Primary.APIKey)
	cfg.Embedding.Fallback.APIKey = expandEnvVars(cfg.Embedding.Fallback.APIKey)
}
