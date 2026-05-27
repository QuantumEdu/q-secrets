package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/QuantumEdu/q-secrets/internal"
)

var addCmd = &cobra.Command{
	Use:   "add <project> <key>=<value> [...]",
	Short: "Add secrets to a project",
	Long: `Add one or more secrets to a project.
The project is created automatically if it doesn't exist.

Examples:
  q-secret add pi ANTHROPIC_KEY=sk-ant-xxx
  q-secret add pi OPENAI_KEY=sk-open-xxx DB_URL=postgres://...
  q-secret add opencode OPENAI_KEY=sk-open-xxx
`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]
		pairs := args[1:]

		// Obtener public key
		publicKey, err := internal.ReadPublicKey()
		if err != nil {
			return err
		}

		// Abrir DB
		db, err := internal.OpenDB(dbPath)
		if err != nil {
			return err
		}
		defer db.Close()

		// Procesar cada key=value
		var added int
		for _, pair := range pairs {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return fmt.Errorf("invalid format: %q. Use: KEY=VALUE", pair)
			}
			key, value := parts[0], parts[1]

			// Encriptar valor
			encrypted, err := internal.Encrypt([]byte(value), publicKey)
			if err != nil {
				return fmt.Errorf("encrypting %s: %w", key, err)
			}

			// Guardar en DB
			if err := db.UpsertSecret(project, key, encrypted); err != nil {
				return fmt.Errorf("saving %s: %w", key, err)
			}

			added++
		}

		if added == 1 {
			fmt.Printf("✓ Added 1 secret to project %q\n", project)
		} else {
			fmt.Printf("✓ Added %d secrets to project %q\n", added, project)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}
