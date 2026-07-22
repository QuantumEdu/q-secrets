package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/QuantumEdu/q-secrets/internal"
)

var (
	dbPath   string
	cliVersion = "dev"
	cliCommit  = "none"
	cliDate    = "unknown"
)

// SetVersion inyecta version, commit y date desde ldflags en el build
func SetVersion(v, c, d string) {
	cliVersion = v
	cliCommit = c
	cliDate = d
}

var rootCmd = &cobra.Command{
	Use:   "q-secrets",
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
		// Commands that don't require a database
		switch cmd.Name() {
		case "init", "version", "completion", "help":
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

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("q-secrets %s (commit: %s, built: %s)\n", cliVersion, cliCommit, cliDate)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db-path", internal.DefaultDBPath, "Path to database file")
	rootCmd.AddCommand(versionCmd)
}
