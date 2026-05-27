package internal

import (
	"fmt"
	"os"
	"runtime"
)

// GetMasterKey busca la master key en orden:
// 1. Variable de entorno Q_SECRET_KEY
// 2. Keychain del SO (cuando esté implementado)
//
// Por ahora solo soporta Q_SECRET_KEY.
// El keychain del SO se agregará cuando tengamos el package correspondiente.
func GetMasterKey() (string, error) {
	// 1. Env var
	if key := os.Getenv("Q_SECRET_KEY"); key != "" {
		return key, nil
	}

	// 2. Keychain del SO (próximamente)
	// Por ahora damos un error instructivo.
	return "", fmt.Errorf(`master key not found.

Options:
  1. Set the Q_SECRET_KEY environment variable with your age private key:
     $env:Q_SECRET_KEY = "AGE-SECRET-KEY-1..."
     
  2. Initialize with q-secret init to store it in the system keychain.

  Run: q-secret init --help`)
}

// SaveMasterKey guarda la master key en el keychain del SO.
// Por implementar: Windows Credential Manager, macOS Keychain, Linux libsecret.
func SaveMasterKey(key string) error {
	// Placeholder: por ahora solo validamos que la key no esté vacía
	if key == "" {
		return fmt.Errorf("cannot save empty master key")
	}

	_ = runtime.GOOS // para referencia futura
	return nil
}

// MasterKeyExists verifica si hay una master key configurada.
func MasterKeyExists() bool {
	if os.Getenv("Q_SECRET_KEY") != "" {
		return true
	}
	return false
}
