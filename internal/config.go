package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// DefaultConfigDir devuelve el directorio de configuración según el SO.
func DefaultConfigDir() string {
	switch runtime.GOOS {
	case "windows":
		if d := os.Getenv("APPDATA"); d != "" {
			return filepath.Join(d, "q-secret")
		}
		return filepath.Join(os.Getenv("USERPROFILE"), ".config", "q-secret")
	default:
		if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
			return filepath.Join(d, "q-secret")
		}
		return filepath.Join(os.Getenv("HOME"), ".config", "q-secret")
	}
}

// DefaultDBPath devuelve la ruta por defecto de la base de datos.
var DefaultDBPath = filepath.Join(DefaultConfigDir(), "q-secret.db")

// PublicKeyPath devuelve la ruta del archivo de public key.
var PublicKeyPath = filepath.Join(DefaultConfigDir(), "public.key")

// ReadPublicKey lee la public key del archivo.
func ReadPublicKey() (string, error) {
	data, err := os.ReadFile(PublicKeyPath)
	if err != nil {
		return "", fmt.Errorf("reading public key: %w\nRun 'q-secret init' first", err)
	}
	return string(data), nil
}

// WritePublicKey guarda la public key en el archivo.
func WritePublicKey(pubKey string) error {
	dir := filepath.Dir(PublicKeyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	return os.WriteFile(PublicKeyPath, []byte(pubKey), 0644)
}
