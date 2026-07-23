package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/QuantumEdu/q-secrets/internal"
)

var (
	initForce      bool
	initMasterKey  string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the secret database",
	Long: `Initialize the q-secret database and configure the master key.

The master key is an age private key used to encrypt and decrypt your secrets.
You can:
  1. Provide an existing age private key with --master-key
  2. Let q-secret generate a new key pair (recommended for new users)

Example:
  q-secret init
  q-secret init --master-key "AGE-SECRET-KEY-1..."
  q-secret init --force
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Verificar si ya existe DB
		if _, err := os.Stat(dbPath); err == nil && !initForce {
			return fmt.Errorf("database already exists at %s\nUse --force to overwrite", dbPath)
		}

		// 2. Obtener master key
		var masterKey, publicKey string

		if initMasterKey != "" {
			// Usar la key provista
			masterKey = initMasterKey
			pk, err := internal.DerivePublicKey(masterKey)
			if err != nil {
				return fmt.Errorf("invalid master key: %w", err)
			}
			publicKey = pk

			// Validar round-trip
			if !internal.ValidateKeyPair(masterKey, publicKey) {
				return fmt.Errorf("master key validation failed: key pair is invalid")
			}
			fmt.Println("✓ Master key validated")
		} else {
			// Preguntar al usuario
			fmt.Println("No master key provided. You can:")
			fmt.Println("  1. Generate a new key pair (recommended)")
			fmt.Println("  2. Paste an existing age private key")
			fmt.Print("\nChoose [1/2] (default: 1): ")

			reader := bufio.NewReader(os.Stdin)
			choice, _ := reader.ReadString('\n')
			choice = strings.TrimSpace(choice)

			if choice == "2" {
				// Pedir que pegue la key
				fmt.Print("Paste your age private key (starts with AGE-SECRET-KEY-): ")
				keyInput, _ := reader.ReadString('\n')
				masterKey = strings.TrimSpace(keyInput)

				if !strings.HasPrefix(masterKey, "AGE-SECRET-KEY-") {
					return fmt.Errorf("invalid age private key: must start with AGE-SECRET-KEY-")
				}

				pk, err := internal.DerivePublicKey(masterKey)
				if err != nil {
					return fmt.Errorf("invalid master key: %w", err)
				}
				publicKey = pk

				if !internal.ValidateKeyPair(masterKey, publicKey) {
					return fmt.Errorf("master key validation failed")
				}
				fmt.Println("✓ Master key validated")
			} else {
				// Generar nueva
				fmt.Print("\nGenerating new age key pair...")
				var err error
				masterKey, publicKey, err = internal.GenerateAgeKey()
				if err != nil {
					return err
				}
				fmt.Println(" done!")

				fmt.Println("\n⚠️  IMPORTANT: Back up your master key!")
				fmt.Println("   If you lose it, your secrets cannot be recovered.")
				fmt.Println("\n   Private key (save this somewhere safe - Bitwarden, 1Password, etc.):")
				fmt.Println(masterKey)
				fmt.Printf("\n   Public key: %s\n\n", publicKey)
				fmt.Print("Press Enter after you have backed up the key...")
				reader.ReadString('\n')
			}
		}

		// 3. Guardar public key
		if err := internal.WritePublicKey(publicKey); err != nil {
			return fmt.Errorf("saving public key: %w", err)
		}

		// 4. Guardar master key (non-fatal — keychain may not be available)
		if err := internal.SaveMasterKey(masterKey); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			fmt.Fprintf(os.Stderr, "Set Q_SECRET_KEY=%s in your environment to use q-secrets.\n", masterKey)
		}

		// 5. Crear DB
		db, err := internal.OpenDB(dbPath)
		if err != nil {
			return fmt.Errorf("creating database: %w", err)
		}
		db.Close()

		fmt.Println("\n  Database:", dbPath)
		fmt.Println("  Public key:", publicKey)
		fmt.Println("  Master key: stored in system keychain (or Q_SECRET_KEY env var)")
		fmt.Println("\n ✓ q-secret initialized successfully!")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "Overwrite existing database")
	initCmd.Flags().StringVarP(&initMasterKey, "master-key", "m", "", "Provide master key directly")
}
