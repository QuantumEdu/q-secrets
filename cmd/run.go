package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/QuantumEdu/q-secrets/internal"
)

var runCmd = &cobra.Command{
	Use:   "run <project> -- <command> [args...]",
	Short: "Run a command with secrets injected as environment variables",
	Long: `Run a command with secrets from a project injected as environment variables.

The secrets are decrypted, injected into the child process, and
automatically cleaned up when the process exits.

Examples:
  q-secret run pi -- opencode
  q-secret run pi -- python app.py
  q-secret run pi -- docker-compose up
  q-secret run pi -- printenv ANTHROPIC_KEY
`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]

		// El resto de args es el comando + sus argumentos
		// Si hay "--", cobra ya separó los args correctamente
		command := args[1]
		var cmdArgs []string
		if len(args) > 2 {
			cmdArgs = args[2:]
		}

		// Verificar que el comando existe
		if _, err := os.Stat(command); os.IsNotExist(err) {
			// No es una ruta, buscar en PATH
			if _, err := os.Executable(); err != nil {
				// No podemos verificar en PATH fácilmente, dejamos que exec.Command falle
			}
		}

		// Abrir DB
		db, err := internal.OpenDB(dbPath)
		if err != nil {
			return err
		}
		defer db.Close()

		// Verificar que el proyecto existe
		exists, err := db.ProjectExists(project)
		if err != nil {
			return err
		}
		if !exists {
			projs, _ := db.ListProjects()
			if len(projs) == 0 {
				return errors.New("no projects found. Add one with: q-secret add <project> KEY=VALUE")
			}
			return fmt.Errorf("project %q not found. Available projects: %v", project, projs)
		}

		// Inyectar y ejecutar
		if err := db.InjectEnv(project, command, cmdArgs); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
