package internal

import (
	"fmt"
	"os"
	"os/exec"
)

// InjectEnv ejecuta un comando con secrets inyectados como variables de entorno.
// project: nombre del proyecto del cual tomar los secrets
// command: comando a ejecutar
// args: argumentos del comando
func (db *DB) InjectEnv(project, command string, args []string) error {
	// 1. Obtener master key
	masterKey, err := GetMasterKey()
	if err != nil {
		return err
	}

	// 2. Obtener public key (para derivar, pero no la necesitamos para decrypt)
	// La private key es suficiente.

	// 3. Buscar secrets del proyecto
	secrets, err := db.ListSecrets(project)
	if err != nil {
		return fmt.Errorf("listing secrets for project %q: %w", project, err)
	}
	if len(secrets) == 0 {
		return fmt.Errorf("no secrets found for project %q", project)
	}

	// 4. Desencriptar cada valor
	envMap := make(map[string]string)
	for _, s := range secrets {
		decrypted, err := Decrypt([]byte(s.ValueEnc), masterKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt %s: %w", s.KeyName, err)
		}
		envMap[s.KeyName] = string(decrypted)
	}

	// 5. Construir comando con env vars inyectadas
	cmd := exec.Command(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Heredar env vars del padre + agregar secrets
	cmd.Env = os.Environ()
	for k, v := range envMap {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// 6. Ejecutar
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("executing command: %w", err)
	}

	return nil
}
