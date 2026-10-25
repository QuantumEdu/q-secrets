package cmd

import (
	"fmt"

	"github.com/QuantumEdu/q-secrets/internal"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get <project> <key>",
	Short: "Get the decrypted value of a secret",
	Long: `Get the decrypted value of a specific secret.
Useful for scripting or piping to other commands.

Examples:
  q-secret get pi ANTHROPIC_KEY
  q-secret get pi ANTHROPIC_KEY | clip
`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("output")
		if format != "" {
			if err := validateOutput(format, true); err != nil {
				return err
			}
		}
		project, key := args[0], args[1]

		masterKey, err := internal.GetMasterKey()
		if err != nil {
			return err
		}

		db, err := internal.OpenDB(dbPath)
		if err != nil {
			return err
		}
		defer db.Close()

		encrypted, err := db.GetSecret(project, key)
		if err != nil {
			return err
		}

		decrypted, err := internal.Decrypt([]byte(encrypted), masterKey)
		if err != nil {
			return fmt.Errorf("decrypting secret: %w", err)
		}

		if format == "" {
			_, err := cmd.OutOrStdout().Write(decrypted)
			return err
		}
		return renderSecrets(cmd.OutOrStdout(), []exportSecret{{Project: project, Key: key, Value: string(decrypted)}}, format, true)
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
	getCmd.Flags().String("output", "", "Output format: json, env or table (default: raw value)")
}
