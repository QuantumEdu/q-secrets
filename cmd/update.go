package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/QuantumEdu/q-secrets/internal"
)

var updateCmd = &cobra.Command{
	Use:   "update <project> <key> <value>",
	Short: "Update an existing secret",
	Long: `Update the value of an existing secret.
Fails if the secret doesn't exist (use add instead).

Examples:
  q-secret update pi ANTHROPIC_KEY sk-ant-new-key
`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, key, value := args[0], args[1], args[2]

		publicKey, err := internal.ReadPublicKey()
		if err != nil {
			return err
		}

		db, err := internal.OpenDB(dbPath)
		if err != nil {
			return err
		}
		defer db.Close()

		// Verificar que existe
		_, err = db.GetSecret(project, key)
		if err != nil {
			return fmt.Errorf("cannot update: %w\nUse 'add' to create it", err)
		}

		// Encriptar y actualizar
		encrypted, err := internal.Encrypt([]byte(value), publicKey)
		if err != nil {
			return fmt.Errorf("encrypting: %w", err)
		}

		if err := db.UpsertSecret(project, key, encrypted); err != nil {
			return err
		}

		fmt.Printf("✓ Updated %s in project %q\n", key, project)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

// deleteCmd
var deleteCmd = &cobra.Command{
	Use:   "delete <project> [key]",
	Short: "Delete secrets or projects",
	Long: `Delete a specific secret or an entire project.
When deleting a project, all its secrets are removed too.

Examples:
  q-secret delete pi ANTHROPIC_KEY    # Delete one secret
  q-secret delete pi                   # Delete entire project
`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]

		db, err := internal.OpenDB(dbPath)
		if err != nil {
			return err
		}
		defer db.Close()

		if len(args) == 2 {
			// Borrar un secret específico
			key := args[1]
			if err := db.DeleteSecret(project, key); err != nil {
				return err
			}
			fmt.Printf("✓ Deleted %s from project %q\n", key, project)
		} else {
			// Borrar todo el proyecto
			if !deleteForce {
				fmt.Printf("Are you sure you want to delete project %q and all its secrets? (y/N): ", project)
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(strings.TrimSpace(response)) != "y" {
					fmt.Println("Cancelled.")
					return nil
				}
			}
			if err := db.DeleteProject(project); err != nil {
				return err
			}
			fmt.Printf("✓ Deleted project %q\n", project)
		}

		return nil
	},
}

var deleteForce bool

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "Delete without confirmation")
}
