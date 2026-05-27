package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/QuantumEdu/q-secrets/internal"
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
  q-secret export --output backup.json
`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		var secrets []exportSecret

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

		if len(secrets) == 0 {
			fmt.Println("No secrets to export.")
			return nil
		}

		data, err := json.MarshalIndent(secrets, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling JSON: %w", err)
		}

		outputPath, _ := cmd.Flags().GetString("output")
		if outputPath != "" {
			if err := os.WriteFile(outputPath, data, 0600); err != nil {
				return fmt.Errorf("writing file: %w", err)
			}
			fmt.Printf("✓ Exported %d secrets to %s\n", len(secrets), outputPath)
		} else {
			fmt.Println(string(data))
		}

		return nil
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
func readAllStdin() ([]byte, error) {
	var data []byte
	buf := make([]byte, 4096)
	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	return data, nil
}

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().StringP("output", "o", "", "Output file path")

	rootCmd.AddCommand(importCmd)

	// Registrar timestamp de exportación
	_ = time.Now
}
