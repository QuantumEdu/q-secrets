# Tasks: `q-secret` CLI

## Dependencia entre fases

```
Fase 1 (MVP)
├── T1.1  Init Go project + dependencias
├── T1.2  internal/db.go — schema SQLite + CRUD
├── T1.3  internal/keychain.go — guardar/leer master key
├── T1.4  internal/crypto.go — encrypt/decrypt con age
├── T1.5  cmd/root.go + cmd/init.go — q-secret init
├── T1.6  cmd/add.go — q-secret add
├── T1.7  cmd/list.go — q-secret list
├── T1.8  internal/inject.go + cmd/run.go — q-secret run
├── T1.9  Integration test full: init → add → run → verify
├── T1.10 README + Makefile
└── T1.11 Build cross-platform + verify

Fase 2 (Completitud) — ✅ Completada
├── T2.1  cmd/get.go — q-secret get ✅
├── T2.2  cmd/update.go — q-secret update ✅
├── T2.3  cmd/delete.go — q-secret delete ✅
├── T2.4  Tests: 23 unit + integration para crypto, db, inject ✅
├── T2.5  Autocompletado bash/zsh/powershell (cobra built-in) ✅
├── T2.6  cmd/export.go — q-secret export (JSON decrypt) ✅
└── T2.7  cmd/export.go — q-secret import (JSON re-encrypt) ✅

Fase 3 (Polish) — ✅ Completada
├── T3.1  Keychain del SO (Windows Credential Manager / macOS Keychain / Linux libsecret) ✅
├── T3.2  --watch mode ✅
├── T3.3  GitHub Actions para build + release ✅
└── T3.4  Homebrew tap / scoop bucket ✅
```

---

## Fase 1 — MVP

### T1.1 Init Go project + dependencias

- `go mod init github.com/tu-user/q-secret`
- `go get modernc.org/sqlite`
- `go get github.com/spf13/cobra`
- Dependencia externa: `age` instalado en el sistema (no es dependencia Go)
- Criterio: `go build` compila sin errores

### T1.2 internal/db.go — schema SQLite + CRUD

**Archivo:** `internal/db.go`

**Funciones:**

| Función | Descripción |
|----------|-------------|
| `OpenDB(path string) (*DB, error)` | Abre o crea DB, ejecuta migrations |
| `(db *DB) Close()` | Cierra conexión |
| `(db *DB) CreateProject(name string) error` | Crea proyecto si no existe |
| `(db *DB) UpsertSecret(project, key string, valueEnc []byte) error` | Inserta o actualiza |
| `(db *DB) GetSecret(project, key string) ([]byte, error)` | Obtiene valor encriptado |
| `(db *DB) ListProjects() ([]string, error)` | Lista todos los proyectos |
| `(db *DB) ListSecrets(project string) ([]Secret, error)` | Lista keys de un proyecto (sin desencriptar) |
| `(db *DB) DeleteSecret(project, key string) error` | Borra un secret |
| `(db *DB) DeleteProject(project string) error` | Borra proyecto y todos sus secrets (cascade) |

**Schema SQL:**

```sql
CREATE TABLE IF NOT EXISTS projects (
    id      TEXT PRIMARY KEY,
    created TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS secrets (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    key_name   TEXT NOT NULL,
    value_enc  TEXT NOT NULL,
    created    TEXT NOT NULL DEFAULT (datetime('now')),
    updated    TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(project_id, key_name)
);

CREATE INDEX IF NOT EXISTS idx_secrets_project ON secrets(project_id);
```

**Criterio de éxito:** Tests en memoria pasan sin errores. CRUD completo probado.

### T1.3 internal/keychain.go — master key

**Archivo:** `internal/keychain.go`

**Funciones:**

| Función | Descripción |
|----------|-------------|
| `GetMasterKey() (string, error)` | Busca en orden: `Q_SECRET_KEY` env var → keychain del SO |
| `SaveMasterKey(key string) error` | Guarda en keychain del SO |
| `MasterKeyExists() bool` | Verifica si ya hay master key configurada |

**Comportamiento por OS:**

- **Windows:** Windows Credential Manager, service `q-secret`, account `master-key`
- **macOS:** macOS Keychain, service `q-secret`, account `master-key`
- **Linux:** `secret-tool store --label='q-secret master key' service q-secret account master-key`
- **Fallback:** Si no hay keychain disponible, pedir al usuario que setee `Q_SECRET_KEY` en su perfil de shell

**Criterio de éxito:** En Windows con Credential Manager, guarda y recupera correctamente. En Linux sin keychain, da error claro.

### T1.4 internal/crypto.go — encrypt/decrypt con age

**Archivo:** `internal/crypto.go`

**Funciones:**

| Función | Descripción |
|----------|-------------|
| `GenerateAgeKey() (privateKey, publicKey string, err error)` | Ejecuta `age-keygen` |
| `DerivePublicKey(privateKey string) (string, error)` | Extrae public key de una private key |
| `Encrypt(plaintext []byte, publicKey string) ([]byte, error)` | age --encrypt |
| `Decrypt(ciphertext []byte, privateKey string) ([]byte, error)` | age --decrypt con temp file |
| `ValidateKeyPair(privateKey, publicKey string) bool` | Round-trip encrypt → decrypt para validar |

**Detalles de implementación:**

```go
func Decrypt(ciphertext []byte, privateKey string) ([]byte, error) {
    // Temp file con permiso 0600
    keyFile, err := os.CreateTemp("", "q-secret-key-*")
    if err != nil {
        return nil, fmt.Errorf("creating temp key file: %w", err)
    }
    defer os.Remove(keyFile.Name())
    
    if err := os.WriteFile(keyFile.Name(), []byte(privateKey), 0600); err != nil {
        return nil, fmt.Errorf("writing temp key file: %w", err)
    }
    keyFile.Close()
    
    // age --decrypt -i <keyfile>
    cmd := exec.Command("age", "--decrypt", "-i", keyFile.Name())
    cmd.Stdin = bytes.NewReader(ciphertext)
    
    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("age decrypt failed: %w", err)
    }
    
    return output, nil
}
```

**Criterio de éxito:** Round-trip encrypt/decrypt funciona correctamente. Validación de key pair detecta keys inválidas.

### T1.5 cmd/root.go + cmd/init.go — q-secret init

**Archivo:** `cmd/root.go`

```go
var rootCmd = &cobra.Command{
    Use:   "q-secret",
    Short: "Secret manager for local development",
    Long:  `q-secret manages API keys, tokens, and credentials
in an encrypted SQLite database and injects them as
environment variables when running commands.`,
}

func Execute() {
    cobra.CheckErr(rootCmd.Execute())
}

func init() {
    rootCmd.AddCommand(initCmd)
    rootCmd.AddCommand(addCmd)
    rootCmd.AddCommand(listCmd)
    rootCmd.AddCommand(runCmd)
    // ... (Fase 2: get, update, delete)
}
```

**Archivo:** `cmd/init.go`

```
q-secret init [flags]

Flags:
  --db-path string   Path to database (default: ~/.config/q-secret/q-secret.db)
  --force            Skip confirmation prompts
  --master-key string  Provide master key directly (useful for scripting)
```

**Flujo:**

1. Verificar si ya hay DB y master key
2. Si hay, preguntar confirmación (salvo `--force`)
3. Si `--master-key` fue provisto, usarla directamente
4. Si no, preguntar al usuario: ¿pegar key o generar nueva?
   - **Opción 1:** Pegar private key existente → validar
   - **Opción 2:** Generar nueva → ejecutar age-keygen, mostrar ambas keys, pedir respaldo
5. Guardar master key en keychain
6. Guardar public key en `~/.config/q-secret/public.key`
7. Crear DB con schema

**Output esperado:**
```
✓ q-secret initialized
  Database: ~/.config/q-secret/q-secret.db
  Public key: age1abc...def
  Master key stored in system keychain
```

**Criterio de éxito:** Ejecuto `q-secret init` y el directorio `~/.config/q-secret/` existe con los archivos esperados.

### T1.6 cmd/add.go — q-secret add

```
q-secret add <project> <key>=<value> [<key2>=<value2>...]

Examples:
  q-secret add pi ANTHROPIC_API_KEY=sk-ant-xxx
  q-secret add pi ANTHROPIC_API_KEY=sk-ant-xxx OPENAI_KEY=sk-open-xxx
  q-secret add opencode OPENAI_KEY=sk-open-xxx
```

**Flujo:**
1. Obtener master key
2. Leer public key de `~/.config/q-secret/public.key`
3. Validar key=value (no vacío, no espacios en key)
4. Si el proyecto no existe, crearlo
5. Encriptar value con age
6. `INSERT OR REPLACE INTO secrets`
7. Repetir para cada par key=value

**Criterio de éxito:** `q-secret add test FOO=bar` → en la DB aparece `FOO` con valor encriptado.

### T1.7 cmd/list.go — q-secret list

```
q-secret list [<project>]
```

**Flujo sin proyecto:**
1. Leer todos los proyectos de la DB
2. Por cada proyecto, listar sus keys (sin desencriptar)
3. Output formateado en columnas

**Flujo con proyecto:**
1. Desencriptar cada valor
2. Mostrar key + valor truncado (últimos 4 chars visibles)
3. No mostrar el valor completo (para eso está `get`)

**Criterio de éxito:** `q-secret list pi` muestra keys con valores truncados.

### T1.8 internal/inject.go + cmd/run.go — q-secret run

```
q-secret run <project> -- <command> [args...]
```

**Ver SPECS.md → sección 8** para el algoritmo completo.

**Casos borde:**
- Si `--` no se provee, el primer argumento posicional es el comando
- Si el comando tiene args con espacios, usar `--` obligatorio
- Si el proyecto existe pero no tiene secrets, error claro
- El proceso hereda stdin/stdout/stderr

**Criterio de éxito:**
```bash
$ q-secret add test MY_VAR=hello
$ q-secret run test -- printenv MY_VAR
hello
```

### T1.9 Integration test full cycle

```go
func TestFullCycle(t *testing.T) {
    tmpDir := t.TempDir()
    dbPath := filepath.Join(tmpDir, "test.db")
    keyPath := filepath.Join(tmpDir, "public.key")
    
    // Usar una master key de testing
    masterKey := "AGE-SECRET-KEY-1..."  // key para tests
    
    // Init
    runCLI("init", "--db-path", dbPath, "--master-key", masterKey, "--force")
    
    // Add
    runCLI("add", "testproj", "MY_SECRET=hello")
    
    // List
    output, _ := runCLI("list", "testproj")
    assert.Contains(t, output, "MY_SECRET")
    
    // Run
    output, _ = runCLI("run", "testproj", "--", "printenv", "MY_SECRET")
    assert.Equal(t, "hello", strings.TrimSpace(output))
}
```

### T1.10 README + Makefile

README.md debe contener:
- ¿Qué es q-secret?
- Instalación (go install, scoop, brew, binario)
- Prerequisito: age instalado
- Quick start: 5 líneas
- Uso completo: init, add, list, run, get, update, delete
- Cómo respaldar la master key
- Cómo migrar la DB a otra máquina

Makefile: ver SPECS.md → sección 11

### T1.11 Build cross-platform + verify

```bash
# Build
make build-all

# Verificar binarios
file bin/q-secret-linux
file bin/q-secret-darwin
file bin/q-secret.exe
```

---

## Criterios de "done" para la Fase 1

1. `q-secret init` crea DB + keychain entry
2. `q-secret add test FOO=bar` guarda en DB
3. `q-secret list` muestra proyectos
4. `q-secret list test` muestra keys con valores truncados
5. `q-secret run test -- printenv FOO` imprime `bar`
6. `make test` pasa todos los tests
7. Binarios compilan en Windows, Linux y macOS
8. README documenta el flujo completo
