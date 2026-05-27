package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/QuantumEdu/q-secrets/internal"
)

var listCmd = &cobra.Command{
	Use:   "list [<project>]",
	Short: "List projects or secrets",
	Long: `List all projects, or list secrets within a project.

Examples:
  q-secret list              # List all projects
  q-secret list pi           # List secrets in project "pi" with truncated values
`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := internal.OpenDB(dbPath)
		if err != nil {
			return err
		}
		defer db.Close()

		if len(args) == 0 {
			// Listar proyectos
			projects, err := db.ListProjects()
			if err != nil {
				return err
			}

			if len(projects) == 0 {
				fmt.Println("No projects found. Add one with: q-secret add <project> KEY=VALUE")
				return nil
			}

			for _, p := range projects {
				secrets, _ := db.ListSecrets(p)
				fmt.Printf("%s (%d secrets)\n", p, len(secrets))
				for _, s := range secrets {
					fmt.Printf("  └─ %s\n", s.KeyName)
				}
			}
		} else {
			// Listar secrets de un proyecto
			project := args[0]

			exists, err := db.ProjectExists(project)
			if err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("project %q not found", project)
			}

			secrets, err := db.ListSecrets(project)
			if err != nil {
				return err
			}

			if len(secrets) == 0 {
				fmt.Printf("Project %q has no secrets.\n", project)
				return nil
			}

			// Desencriptar para mostrar valores truncados
			masterKey, err := internal.GetMasterKey()
			if err != nil {
				// Si no hay master key, mostrar solo keys
				for _, s := range secrets {
					fmt.Printf("%s\n", s.KeyName)
				}
				return nil
			}

			for _, s := range secrets {
				decrypted, err := internal.Decrypt([]byte(s.ValueEnc), masterKey)
				if err != nil {
					fmt.Printf("%s    (error decrypting)\n", s.KeyName)
					continue
				}
				val := string(decrypted)
				if len(val) > 4 {
					val = val[:len(val)-4] + "***" + val[len(val)-4:]
				}
				fmt.Printf("%s    %s\n", s.KeyName, val)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
