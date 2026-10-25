package internal

import (
	"fmt"
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
		if err := ValidateEnvName(s.KeyName); err != nil {
			return nil, err
		}
		if _, exists := envMap[s.KeyName]; exists {
			return nil, fmt.Errorf("duplicate environment variable %q", s.KeyName)
		}
		decrypted, err := Decrypt([]byte(s.ValueEnc), masterKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt %s: %w", s.KeyName, err)
		}
		envMap[s.KeyName] = string(decrypted)
	}

	return envMap, nil
}

// InjectEnv is kept for callers that inherit the normal parent environment.
func (db *DB) InjectEnv(project, command string, args []string) error {
	envMap, err := db.BuildEnv(project)
	if err != nil {
		return err
	}
	cmd, err := NewChildCommand(envMap, command, args, false)
	if err != nil {
		return err
	}
	return RunChild(cmd)
}
