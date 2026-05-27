package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/QuantumEdu/q-secrets/internal"
)

var (
	dbPath string
)

var rootCmd = &cobra.Command{
	Use:   "q-secret",
	Short: "Secret manager for local development",
	Long: `q-secret manages API keys, tokens, and credentials
in an encrypted SQLite database and injects them as
environment variables when running commands.

Quick start:
  q-secret init                         # Initialize database
  q-secret add pi ANTHROPIC_KEY=xxx     # Add a secret
  q-secret list                         # List projects
  q-secret run pi -- myapp             # Run with injected secrets

Documentation: https://github.com/QuantumEdu/q-secrets`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip DB check for init command
		if cmd.Name() == "init" {
			return nil
		}

		// Verificar que la DB existe
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found at %s\nRun 'q-secret init' first", dbPath)
		}
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db-path", internal.DefaultDBPath, "Path to database file")
}
