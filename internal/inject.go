package internal

import (
	"fmt"
	"os"
	"os/exec"
)

// BuildEnv desencripta todos los secrets de un proyecto y los retorna como
// un mapa de variables de entorno.
func (db *DB) BuildEnv(project string) (map[string]string, error) {
	masterKey, err := GetMasterKey()
	if err != nil {
		return nil, err
	}

	secrets, err := db.ListSecrets(project)
	if err != nil {
		return nil, fmt.Errorf("listing secrets for project %q: %w", project, err)
	}
	if len(secrets) == 0 {
		return nil, fmt.Errorf("no secrets found for project %q", project)
	}

	envMap := make(map[string]string)
	for _, s := range secrets {
		decrypted, err := Decrypt([]byte(s.ValueEnc), masterKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt %s: %w", s.KeyName, err)
		}
		envMap[s.KeyName] = string(decrypted)
	}

	return envMap, nil
}

// InjectEnv ejecuta un comando con secrets inyectados como variables de entorno.
// project: nombre del proyecto del cual tomar los secrets
// command: comando a ejecutar
// args: argumentos del comando
func (db *DB) InjectEnv(project, command string, args []string) error {
	envMap, err := db.BuildEnv(project)
	if err != nil {
		return err
	}

	// Construir comando con env vars inyectadas
	cmd := exec.Command(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Heredar env vars del padre + agregar secrets
	cmd.Env = os.Environ()
	for k, v := range envMap {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("executing command: %w", err)
	}

	return nil
}
