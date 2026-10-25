package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/QuantumEdu/q-secrets/internal"
	"github.com/spf13/cobra"
)

// exportSecret es la estructura para exportar/importar secrets
type exportSecret struct {
	Project string `json:"project"`
	Key     string `json:"key"`
	Value   string `json:"value"`
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export all secrets to JSON (decrypted)",
	Long: `Export all secrets decrypted to a JSON file or stdout.
WARNING: The exported file contains secrets in plain text.
Handle it carefully and delete after use.

Examples:
  q-secret export > backup.json
  q-secret export --file backup.json
  q-secret export --project pi --output env
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("output")
		if err := validateOutput(format, true); err != nil {
			return err
		}
		masterKey, err := internal.GetMasterKey()
		if err != nil {
			return err
		}

		db, err := internal.OpenDB(dbPath)
		if err != nil {
			return err
		}
		defer db.Close()

		projects, err := db.ListProjects()
		if err != nil {
			return err
		}

		project, _ := cmd.Flags().GetString("project")
		if project != "" {
			exists, err := db.ProjectExists(project)
			if err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("project %q not found", project)
			}
			projects = []string{project}
		}
		secrets := make([]exportSecret, 0)

		for _, proj := range projects {
			projSecrets, err := db.ListSecrets(proj)
			if err != nil {
				return fmt.Errorf("listing secrets for %q: %w", proj, err)
			}

			for _, s := range projSecrets {
				decrypted, err := internal.Decrypt([]byte(s.ValueEnc), masterKey)
				if err != nil {
					return fmt.Errorf("decrypting %s/%s: %w", proj, s.KeyName, err)
				}
				secrets = append(secrets, exportSecret{
					Project: proj,
					Key:     s.KeyName,
					Value:   string(decrypted),
				})
			}
		}

		var data bytes.Buffer
		if err := renderSecrets(&data, secrets, format, true); err != nil {
			return err
		}
		outputPath, _ := cmd.Flags().GetString("file")
		if outputPath != "" {
			// Refuse symlinks/existing files rather than retaining permissive modes.
			file, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				return fmt.Errorf("creating export file: %w", err)
			}
			_, writeErr := file.Write(data.Bytes())
			closeErr := file.Close()
			if writeErr != nil {
				return writeErr
			}
			return closeErr
		}
		_, err = cmd.OutOrStdout().Write(data.Bytes())
		return err
	},
}

var importCmd = &cobra.Command{
	Use:   "import [<file>]",
	Short: "Import secrets from a JSON file",
	Long: `Import secrets from a JSON file previously exported with 'q-secret export'.
Reads from a file or stdin.

Format:
  [
    {"project": "pi", "key": "ANTHROPIC_KEY", "value": "sk-ant-xxx"},
    {"project": "opencode", "key": "OPENAI_KEY", "value": "sk-open-xxx"}
  ]

Examples:
  q-secret import backup.json
  cat backup.json | q-secret import
`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		publicKey, err := internal.ReadPublicKey()
		if err != nil {
			return err
		}

		var data []byte
		if len(args) > 0 {
			data, err = os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}
		} else {
			// Leer de stdin
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) != 0 {
				return fmt.Errorf("no input file specified and stdin is a terminal\nUsage: q-secret import <file>")
			}
			data, err = readAllStdin()
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
		}

		var secrets []exportSecret
		if err := json.Unmarshal(data, &secrets); err != nil {
			return fmt.Errorf("parsing JSON: %w", err)
		}

		db, err := internal.OpenDB(dbPath)
		if err != nil {
			return err
		}
		defer db.Close()

		for _, secret := range secrets {
			if err := internal.ValidateEnvName(secret.Key); err != nil {
				return err
			}
		}
		var imported int
		for _, s := range secrets {
			encrypted, err := internal.Encrypt([]byte(s.Value), publicKey)
			if err != nil {
				return fmt.Errorf("encrypting %s/%s: %w", s.Project, s.Key, err)
			}
			if err := db.UpsertSecret(s.Project, s.Key, encrypted); err != nil {
				return fmt.Errorf("saving %s/%s: %w", s.Project, s.Key, err)
			}
			imported++
		}

		fmt.Printf("✓ Imported %d secrets\n", imported)
		return nil
	},
}

// readAllStdin lee todo de stdin hasta EOF
func readAllStdin() ([]byte, error) { return io.ReadAll(os.Stdin) }

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().String("output", "json", "Output format: json, env or table")
	exportCmd.Flags().StringP("file", "o", "", "Write a new plaintext file with mode 0600 (refuses existing files)")
	exportCmd.Flags().String("project", "", "Export only this project")

	rootCmd.AddCommand(importCmd)

}
