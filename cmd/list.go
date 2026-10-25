package cmd

import (
	"fmt"

	"github.com/QuantumEdu/q-secrets/internal"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list [<project>]",
	Short: "List project and secret names without decrypting values",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("output")
		if err := validateOutput(format, false); err != nil {
			return err
		}
		db, err := internal.OpenDB(dbPath)
		if err != nil {
			return err
		}
		defer db.Close()
		projects := args
		if len(projects) == 0 {
			projects, err = db.ListProjects()
			if err != nil {
				return err
			}
		} else {
			exists, err := db.ProjectExists(projects[0])
			if err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("project %q not found", projects[0])
			}
		}
		records := make([]exportSecret, 0)
		for _, project := range projects {
			secrets, err := db.ListSecrets(project)
			if err != nil {
				return err
			}
			if len(secrets) == 0 {
				records = append(records, exportSecret{Project: project})
			}
			for _, secret := range secrets {
				records = append(records, exportSecret{Project: project, Key: secret.KeyName})
			}
		}
		return renderSecrets(cmd.OutOrStdout(), records, format, false)
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().String("output", "table", "Output format: json or table (names only)")
}
