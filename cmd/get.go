package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/QuantumEdu/q-secrets/internal"
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

		fmt.Print(string(decrypted))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
}
