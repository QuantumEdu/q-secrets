package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/QuantumEdu/q-secrets/internal"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// readSecretInput validates every name before reading or storing any value.
func readSecretInput(cmd *cobra.Command, args []string) (map[string]string, error) {
	stdin, _ := cmd.Flags().GetBool("stdin")
	argv, _ := cmd.Flags().GetBool("allow-argv")
	result := make(map[string]string)
	if stdin || !argv {
		if len(args) != 1 || strings.Contains(args[0], "=") {
			return nil, fmt.Errorf("use add <project> <KEY> [--stdin]; legacy KEY=VALUE input requires --allow-argv")
		}
		if err := internal.ValidateEnvName(args[0]); err != nil {
			return nil, err
		}
		var value []byte
		var err error
		if stdin {
			value, err = io.ReadAll(cmd.InOrStdin())
		} else {
			input, ok := cmd.InOrStdin().(*os.File)
			if !ok || !term.IsTerminal(int(input.Fd())) {
				return nil, fmt.Errorf("hidden input requires a terminal; use --stdin for piped input")
			}
			fmt.Fprint(cmd.ErrOrStderr(), "Secret value: ")
			value, err = term.ReadPassword(int(input.Fd()))
			fmt.Fprintln(cmd.ErrOrStderr())
		}
		if err != nil {
			return nil, fmt.Errorf("reading secret input: %w", err)
		}
		// stdin is byte-preserving: trailing newlines and empty values are intentional.
		result[args[0]] = string(value)
		return result, nil
	}
	for _, pair := range args {
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("expected KEY=VALUE; prefer KEY --stdin")
		}
		if err := internal.ValidateEnvName(key); err != nil {
			return nil, err
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate environment variable %q", key)
		}
		result[key] = value
	}
	return result, nil
}

var addCmd = &cobra.Command{
	Use:   "add <project> <KEY> [--stdin]",
	Short: "Add a secret with hidden terminal input or stdin",
	Long: `Add a secret using a hidden terminal prompt, or read its exact bytes from stdin.
The project is created automatically. Legacy KEY=VALUE arguments require --allow-argv
and may expose values through shell history and process listings.

Examples:
  q-secrets add pi API_KEY
  secret-source | q-secrets add pi API_KEY --stdin
  q-secrets add pi --allow-argv KEY=VALUE`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		values, err := readSecretInput(cmd, args[1:])
		if err != nil {
			return err
		}
		publicKey, err := internal.ReadPublicKey()
		if err != nil {
			return err
		}
		db, err := internal.OpenDB(dbPath)
		if err != nil {
			return err
		}
		defer db.Close()
		for key, value := range values {
			encrypted, err := internal.Encrypt([]byte(value), publicKey)
			if err != nil {
				return fmt.Errorf("encrypting %s: %w", key, err)
			}
			if err := db.UpsertSecret(args[0], key, encrypted); err != nil {
				return err
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Added %d secret(s) to project %q\n", len(values), args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().Bool("stdin", false, "Read the exact secret value from stdin until EOF")
	addCmd.Flags().Bool("allow-argv", false, "Explicitly allow legacy KEY=VALUE arguments (exposes values in argv)")
	addCmd.MarkFlagsMutuallyExclusive("stdin", "allow-argv")
}
