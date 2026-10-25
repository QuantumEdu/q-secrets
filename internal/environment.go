package internal

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
)

// ValidateEnvName accepts portable POSIX names and reserves the vault master key.
func ValidateEnvName(name string) error {
	if strings.EqualFold(name, "Q_SECRET_KEY") {
		return fmt.Errorf("Q_SECRET_KEY is reserved and cannot be injected")
	}
	if name == "" {
		return fmt.Errorf("environment variable name is empty")
	}
	for i, c := range []byte(name) {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_' || (i > 0 && c >= '0' && c <= '9') {
			continue
		}
		return fmt.Errorf("invalid environment variable name %q: use [A-Za-z_][A-Za-z0-9_]*", name)
	}
	return nil
}

// ChildEnv removes the master key, deduplicates the parent environment and lets
// project secrets override inherited values. Clean mode retains only OS essentials.
func ChildEnv(parent []string, secrets map[string]string, clean bool) ([]string, error) {
	canonical := func(s string) string {
		if runtime.GOOS == "windows" {
			return strings.ToUpper(s)
		}
		return s
	}
	values := make(map[string]string)
	names := make(map[string]string)
	for _, item := range parent {
		key, value, ok := strings.Cut(item, "=")
		if !ok || key == "" || strings.EqualFold(key, "Q_SECRET_KEY") {
			continue
		}
		if clean {
			allowed := key == "PATH"
			if runtime.GOOS == "windows" {
				switch strings.ToUpper(key) {
				case "PATH", "SYSTEMROOT", "WINDIR", "COMSPEC", "PATHEXT", "TEMP", "TMP":
					allowed = true
				}
			}
			if !allowed {
				continue
			}
		}
		c := canonical(key)
		names[c] = key
		values[c] = value
	}
	seen := make(map[string]bool)
	for key, value := range secrets {
		if err := ValidateEnvName(key); err != nil {
			return nil, err
		}
		if strings.ContainsRune(value, 0) {
			return nil, fmt.Errorf("environment value for %s contains NUL", key)
		}
		c := canonical(key)
		if seen[c] {
			return nil, fmt.Errorf("duplicate environment variable %q", key)
		}
		seen[c] = true
		names[c] = key
		values[c] = value
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, names[key]+"="+values[key])
	}
	return env, nil
}
