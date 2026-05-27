package internal

import (
	"fmt"
	"os"
)

// serviceName es el nombre usado en el keychain del SO.
const serviceName = "q-secrets"

// GetMasterKey busca la master key en orden:
// 1. Variable de entorno Q_SECRET_KEY
// 2. Keychain del SO (por implementar con go-keyring)
func GetMasterKey() (string, error) {
	// 1. Env var (rápido, portable)
	if key := os.Getenv("Q_SECRET_KEY"); key != "" {
		return key, nil
	}

	return "", fmt.Errorf(`master key not found.

Set the Q_SECRET_KEY environment variable with your age private key:

  # Windows PowerShell
  $env:Q_SECRET_KEY = "AGE-SECRET-KEY-1..."

  # Linux/macOS
  export Q_SECRET_KEY="AGE-SECRET-KEY-1..."

Or run 'q-secret init' to generate a new key pair.`)
}

// SaveMasterKey guarda la master key.
// Por ahora solo valida que no esté vacía.
// En el futuro: guardar en keychain del SO (Windows Credential Manager,
// macOS Keychain, Linux secret-tool) usando go-keyring.
func SaveMasterKey(key string) error {
	if key == "" {
		return fmt.Errorf("cannot save empty master key")
	}
	return nil
}

// MasterKeyExists verifica si hay una master key configurada.
func MasterKeyExists() bool {
	return os.Getenv("Q_SECRET_KEY") != ""
}
