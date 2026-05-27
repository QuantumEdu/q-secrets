package internal

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// findKeygen busca age-keygen o rage-keygen en PATH
func findKeygen() string {
	for _, name := range []string{"age-keygen", "rage-keygen"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return ""
}

// findAge busca age o rage en PATH
func findAge() string {
	for _, name := range []string{"age", "rage"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return ""
}

// GenerateAgeKey genera un nuevo par de claves age.
// Busca age-keygen o rage-keygen en PATH y ejecuta el que encuentre.
func GenerateAgeKey() (privateKey, publicKey string, err error) {
	keygen := findKeygen()
	if keygen == "" {
		return "", "", fmt.Errorf("neither age-keygen nor rage-keygen found in PATH\nInstall age: scoop install age (or rage)")
	}

	cmd := exec.Command(keygen)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("age-keygen failed: %w\nIs age installed?", err)
	}

	lines := strings.Split(out.String(), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "# public key: ") {
			publicKey = strings.TrimPrefix(line, "# public key: ")
		}
		if strings.HasPrefix(line, "AGE-SECRET-KEY-") {
			privateKey = strings.TrimSpace(line)
		}
	}

	if privateKey == "" || publicKey == "" {
		return "", "", fmt.Errorf("failed to parse age-keygen output:\n%s", out.String())
	}

	return privateKey, publicKey, nil
}

// DerivePublicKey extrae la public key de la private key.
// Busca en el comentario primero, luego intenta con age-keygen -y o rage-keygen -y.
func DerivePublicKey(privateKey string) (string, error) {
	// Primero buscar en el contenido si incluye el comentario
	for _, line := range strings.Split(privateKey, "\n") {
		if strings.HasPrefix(line, "# public key: ") {
			return strings.TrimPrefix(line, "# public key: "), nil
		}
	}

	// Si no hay comentario, intentar extraer con keygen -y
	keygen := findKeygen()
	if keygen == "" {
		return "", fmt.Errorf("neither age-keygen nor rage-keygen found in PATH")
	}

	cmd := exec.Command(keygen, "-y")
	cmd.Stdin = strings.NewReader(strings.TrimSpace(privateKey))
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to derive public key: %w", err)
	}

	return strings.TrimSpace(out.String()), nil
}

// Encrypt encripta plaintext usando age (o rage) con la public key.
func Encrypt(plaintext []byte, publicKey string) ([]byte, error) {
	age := findAge()
	if age == "" {
		return nil, fmt.Errorf("neither age nor rage found in PATH\nInstall age: scoop install age (or rage)")
	}

	// Escribir plaintext a temp file para pasarlo a age
	tmpFile, err := os.CreateTemp("", "q-secret-plain-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if err := os.WriteFile(tmpFile.Name(), plaintext, 0600); err != nil {
		return nil, fmt.Errorf("writing temp file: %w", err)
	}
	tmpFile.Close()

	cmd := exec.Command(age, "--encrypt",
		"-r", publicKey,
		"-o", "-",
		tmpFile.Name())

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("age encrypt failed: %w", err)
	}

	return output, nil
}

// Decrypt desencripta ciphertext usando age (o rage) con la private key.
// Crea un temp file con permiso 0600 para pasar la key a age.
func Decrypt(ciphertext []byte, privateKey string) ([]byte, error) {
	age := findAge()
	if age == "" {
		return nil, fmt.Errorf("neither age nor rage found in PATH\nInstall age: scoop install age (or rage)")
	}

	// Temp file para la private key
	keyFile, err := os.CreateTemp("", "q-secret-key-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp key file: %w", err)
	}
	defer os.Remove(keyFile.Name())

	if err := os.WriteFile(keyFile.Name(), []byte(privateKey), 0600); err != nil {
		return nil, fmt.Errorf("writing temp key file: %w", err)
	}
	keyFile.Close()

	cmd := exec.Command(age, "--decrypt",
		"-i", keyFile.Name(),
		"-o", "-",
		"-") // leer ciphertext de stdin

	cmd.Stdin = bytes.NewReader(ciphertext)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("age decrypt failed: %w", err)
	}

	return output, nil
}

// ValidateKeyPair hace un round-trip encrypt → decrypt para validar
// que la private key y public key corresponden.
func ValidateKeyPair(privateKey, publicKey string) bool {
	testText := []byte("q-secret-validation-12345")

	encrypted, err := Encrypt(testText, publicKey)
	if err != nil {
		return false
	}

	decrypted, err := Decrypt(encrypted, privateKey)
	if err != nil {
		return false
	}

	return bytes.Equal(decrypted, testText)
}
