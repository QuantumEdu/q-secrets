package internal

import (
	"fmt"
	"os"

	"github.com/zalando/go-keyring"
)

// serviceName es el nombre usado en el keychain del SO.
const serviceName = "q-secrets"
const keychainKey = "master-key"

// GetMasterKey busca la master key en orden:
// 1. Variable de entorno Q_SECRET_KEY (mayor prioridad — override)
// 2. Keychain del SO via go-keyring
func GetMasterKey() (string, error) {
	// 1. Env var (rápido, portable, mayor prioridad)
	if key := os.Getenv("Q_SECRET_KEY"); key != "" {
		return key, nil
	}

	// 2. Keychain del SO
	key, err := keyring.Get(serviceName, keychainKey)
	if err == nil {
		return key, nil
	}

	// Keychain not available — specific diagnostic
	return "", fmt.Errorf(`master key not found.

Neither the Q_SECRET_KEY environment variable nor the system keychain has a master key configured.

To fix:

  • Set the Q_SECRET_KEY environment variable:
    # Linux/macOS
    export Q_SECRET_KEY="AGE-SECRET-KEY-1..."

    # Windows PowerShell
    $env:Q_SECRET_KEY = "AGE-SECRET-KEY-1..."

  • Or run 'q-secrets init' to generate and store a new key pair.

  • Or install a keychain backend:
    Linux:  apt install libsecret-1-0 gnome-keyring
    macOS:  keychain is built-in
    Windows: credential manager is built-in

Diagnostic: %v`, err)
}

// SaveMasterKey guarda la master key en el keychain del SO.
func SaveMasterKey(key string) error {
	if key == "" {
		return fmt.Errorf("cannot save empty master key")
	}

	err := keyring.Set(serviceName, keychainKey, key)
	if err != nil {
		return fmt.Errorf("failed to save master key to system keychain: %w\n\nYour master key was NOT saved.\nSet the Q_SECRET_KEY environment variable instead:\n  export Q_SECRET_KEY=\"%s\"\n\nFor persistent keychain storage, install a backend:\n  Linux: apt install libsecret-1-0 gnome-keyring\n  macOS: built-in keychain\n  Windows: built-in credential manager", err, key)
	}

	return nil
}

// MasterKeyExists verifica si hay una master key configurada.
// Retorna true si Q_SECRET_KEY está seteada O el keychain tiene una entrada.
func MasterKeyExists() bool {
	if os.Getenv("Q_SECRET_KEY") != "" {
		return true
	}

	_, err := keyring.Get(serviceName, keychainKey)
	return err == nil
}
