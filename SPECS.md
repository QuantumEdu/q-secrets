# Specs: `q-secret` CLI

## 0. Diferencia entre Cobra y Bubble Tea

Antes de entrar en detalle técnico, una aclaración importante:

| Aspecto | **Cobra** | **Bubble Tea** |
|---------|-----------|----------------|
| Qué es | Framework para **CLIs clásicos** con subcomandos | Framework para **TUIs** (interfaces de terminal interactivas) |
| Output | Texto plano → stdout | Renderiza en la terminal, ocupa toda la pantalla |
| Input | Flags y argumentos (posiciónales) | Eventos de teclado, mouse, resize |
| UX | `q-secret add pi KEY=val` | Formularios, tablas, spinners, menús interactivos |
| Analogía | `git`, `docker`, `kubectl` | `htop`, `vim`, `lazygit` |
| Para este proyecto | ✅ **Correcto** — q-secret necesita subcomandos | ❌ **Incorrecto** — un TUI no serviría para `q-secret run -- opencode` |

**q-secret usa Cobra.** Bubble Tea es para otro tipo de app (si en vez de `run --` quisieras un navegador interactivo de secrets, ahí entraría Bubble Tea).

---

## 1. Stack tecnológico

| Componente | Tecnología | Justificación |
|-----------|-----------|---------------|
| Lenguaje | **Go 1.23+** | Binario único, cross-compile nativo a 3 OS, excelente stdlib, sin runtime |
| SQLite | **modernc.org/sqlite** | SQLite puro en Go, sin CGO, sin DLL dependencias externas |
| Crypto | **[FiloSottile/age](https://github.com/FiloSottile/age)** v1.2+ | Llamado como subproceso (`os/exec`). Ya instalado en el sistema. |
| CLI framework | **[spf13/cobra](https://github.com/spf13/cobra)** | Estándar de facto para CLIs en Go. Subcomandos, flags, help autogenerado |

### Master key

La master key es una **age private key** que el usuario:

1. Genera con `age-keygen` (o el propio `q-secret init` se la genera)
2. **La guarda donde él quiera**: Bitwarden, 1Password, un USB, un papel
3. **La pega cuando ejecuta `q-secret init`**
4. Después, `q-secret` la guarda temporalmente durante la sesión o la pide cada vez

**Decisión:** La master key se almacena en el keychain del SO (Windows Credential Manager / macOS Keychain / libsecret). El usuario la pega una vez en `init` y `q-secret` la gestiona automáticamente después. Si el usuario prefiere no usar keychain, puede setear la env var `Q_SECRET_KEY` y `q-secret` la usará directamente.

---

## 2. Estructura del repositorio

```
q-secret/
├── main.go                    # Entrypoint
├── cmd/
│   ├── root.go                # Comando raíz
│   ├── init.go                # q-secret init
│   ├── add.go                 # q-secret add <project> <key>=<value>
│   ├── get.go                 # q-secret get <project> <key>
│   ├── list.go                # q-secret list [project]
│   ├── update.go              # q-secret update <project> <key> <value>
│   ├── delete.go              # q-secret delete <project> [key]
│   └── run.go                 # q-secret run <project> -- <command> [args...]
├── internal/
│   ├── db.go                  # Operaciones SQLite
│   ├── crypto.go              # Wrapper para age (encrypt/decrypt como subproceso)
│   ├── keychain.go            # Guardar/recuperar master key del keychain del SO
│   ├── inject.go              # Inyección de env vars + exec del proceso hijo
│   └── config.go              # Paths, defaults, config del usuario
├── go.mod
├── go.sum
├── PRD.md
├── SPECS.md
├── tasks.md
├── Makefile
└── README.md
```

---

## 3. Formato de la base de datos SQLite

```sql
CREATE TABLE projects (
    id        TEXT PRIMARY KEY,  -- slug del proyecto: "pi", "opencode"
    created   TEXT NOT NULL      -- ISO8601
);

CREATE TABLE secrets (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    key_name   TEXT NOT NULL,    -- ej: "ANTHROPIC_API_KEY"
    value_enc  TEXT NOT NULL,    -- valor encriptado con age (cifrado AES-256-GCM)
    created    TEXT NOT NULL,
    updated    TEXT NOT NULL,
    
    UNIQUE(project_id, key_name)
);

CREATE INDEX idx_secrets_project ON secrets(project_id);
```

### ¿Qué se encripta?

- **El valor del secret** se encripta con age usando la master key.
- **Los nombres de key y proyecto** quedan en texto plano en la DB.
- **La DB en sí no está encriptada.** Cualquiera puede ver qué proyectos y qué keys existen, pero no los valores.

Esto es deliberado: permite que el archivo `.db` sea sincronizado (OneDrive, Dropbox) sin exponer los valores. Además permite hacer `list` rápido sin desencriptar.

### Directorio de datos

```
~/.config/q-secret/
├── q-secret.db     ← SQLite
├── public.key      ← Public key (derivada de la master, para encryptar)
└── config.json     ← Preferencias (proyecto default, etc)
```

La master key **nunca está en este directorio**. Está en el keychain del SO, o en la env var `Q_SECRET_KEY`.

---

## 4. Flujo de encriptación con age

### Init: setup de claves

```go
func Init() {
    // 1. Preguntar al usuario: ¿pegar tu age private key o generar una nueva?
    // 2. Si genera nueva:
    //    age-keygen → capturar private + public key
    //    Mostrar ambas para que el usuario las respalde
    // 3. Si pega existente:
    //    Validar que es una key age válida
    //    Extraer public key de la private key
    // 4. Guardar private key en keychain del SO (o validar Q_SECRET_KEY)
    // 5. Guardar public key en ~/.config/q-secret/public.key
    // 6. Crear SQLite con schema
}
```

### Encriptar un valor

```go
func encryptWithAge(plaintext []byte, publicKey string) ([]byte, error) {
    tmpFile, _ := os.CreateTemp("", "age-plaintext-*")
    defer os.Remove(tmpFile.Name())
    os.WriteFile(tmpFile.Name(), plaintext, 0600)
    
    cmd := exec.Command("age", "--encrypt",
        "-r", publicKey,
        "-o", "-",           // stdout
        tmpFile.Name())
    
    return cmd.Output()
}
```

### Desencriptar un valor

```go
func decryptWithAge(ciphertext []byte, privateKey string) ([]byte, error) {
    // Opción A: temp file con la private key
    keyFile, _ := os.CreateTemp("", "age-key-*")
    defer os.Remove(keyFile.Name())
    os.WriteFile(keyFile.Name(), []byte(privateKey), 0600)
    
    cmd := exec.Command("age", "--decrypt",
        "-i", keyFile.Name(),
        "-o", "-",
        "-")  // stdin
    
    cmd.Stdin = bytes.NewReader(ciphertext)
    return cmd.Output()
}
```

**Riesgo mitigado:** el temp file de la key se crea con permiso `0600` (solo el usuario puede leer) y se borra inmediatamente después con `defer os.Remove()`.

---

## 5. Comando: `q-secret init`

```
q-secret init [--db-path ~/.config/q-secret/q-secret.db]
```

Pasos:

1. Si ya existe DB, preguntar si quiere sobrescribir (`--force` para saltar)
2. Si existe keychain entry, preguntar si reemplazar
3. Preguntar al usuario:
   - **¿Pegar tu age private key existente?** → la valida
   - **¿Generar una nueva?** → ejecuta `age-keygen`, muestra ambas keys, pide que las respalde
4. Guardar private key en keychain del SO (o validar que `Q_SECRET_KEY` está seteada)
5. Derivar public key y guardarla en `~/.config/q-secret/public.key`
6. Crear SQLite con schema
7. Output: `✓ q-secret initialized`

### Validación de master key

```go
func validateAgeKey(privateKey string) bool {
    // Intentar desencriptar un texto de prueba encriptado con su public key
    // Si falla, la key no es válida
    testText := []byte("q-secret-validation-test")
    pubKey := derivePublicKey(privateKey)
    
    encrypted, _ := encryptWithAge(testText, pubKey)
    decrypted, err := decryptWithAge(encrypted, privateKey)
    
    return err == nil && bytes.Equal(decrypted, testText)
}
```

---

## 6. Comando: `q-secret add`

```
q-secret add <project> <key>=<value> [<key2>=<value2>...]

$ q-secret add pi ANTHROPIC_API_KEY=sk-ant-xxx OPENAI_KEY=sk-open-xxx
$ q-secret add opencode OPENAI_KEY=sk-open-xxx
```

Pasos:

1. Obtener master key (keychain o `Q_SECRET_KEY`)
2. Leer public key de `~/.config/q-secret/public.key`
3. Si el proyecto no existe, crearlo
4. Por cada `key=value`:
   - Validar que key no esté vacía
   - Encriptar value con age
   - `INSERT OR REPLACE INTO secrets`
5. Output: `✓ Added 2 secrets to project "pi"`

Si el proyecto no existe: se crea automáticamente (no requiere comando separado).

---

## 7. Comando: `q-secret list`

```
q-secret list [<project>]

$ q-secret list
pi:
  ANTHROPIC_API_KEY
  OPENAI_KEY
opencode:
  OPENAI_KEY

$ q-secret list pi
ANTHROPIC_API_KEY    sk-ant-***xxx
OPENAI_KEY           sk-open-***xxx
```

- Sin proyecto: lista proyectos y keys (no desencripta nada)
- Con proyecto: desencripta y muestra valor truncado, mostrando últimos 4 caracteres: `***xxxx`

Para ver el valor completo: `q-secret get <project> <key>`

---

## 8. Comando: `q-secret run`

```
q-secret run <project> -- <command> [args...]

$ q-secret run pi -- opencode
$ q-secret run pi -- python app.py
$ q-secret run pi -- docker-compose up
```

### Algoritmo

```go
func Run(project string, command string, args []string) error {
    // 1. Obtener master key
    masterKey := getMasterKey()  // keychain o Q_SECRET_KEY
    
    // 2. Buscar todos los secrets del proyecto
    secrets := db.GetSecrets(project)
    if len(secrets) == 0 {
        return fmt.Errorf("no secrets found for project %q", project)
    }
    
    // 3. Desencriptar cada valor
    envMap := make(map[string]string)
    for _, s := range secrets {
        value, err := decryptWithAge(s.ValueEnc, masterKey)
        if err != nil {
            return fmt.Errorf("failed to decrypt %s: %w", s.KeyName, err)
        }
        envMap[s.KeyName] = string(value)
    }
    
    // 4. Construir el comando hijito con env vars inyectadas
    cmd := exec.Command(command, args...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Stdin = os.Stdin
    
    cmd.Env = os.Environ()
    for k, v := range envMap {
        cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
    }
    
    // 5. Ejecutar y esperar
    err := cmd.Run()
    
    // 6. Retornar exit code del proceso hijito
    if exitErr, ok := err.(*exec.ExitError); ok {
        os.Exit(exitErr.ExitCode())
    }
    return err
}
```

### Error handling

- Si no existe el proyecto: `error: project "x" not found. Available: pi, opencode`
- Si no hay secrets en el proyecto: `error: project "x" has no secrets`
- Si falla age: error con la salida de age
- Si el comando hijito falla: propagar el exit code
- Si no se encuentra el comando: error del sistema operativo

### Nota sobre Windows

En Windows, `exec.Command("opencode")` busca en PATH. Si opencode no está en PATH, usar ruta completa:
```bash
q-secret run pi -- C:\Users\me\bin\opencode.exe
```

---

## 9. Comandos restantes

### `q-secret get`

```
q-secret get <project> <key>
```

Desencripta y muestra el valor completo por stdout. Útil para scripting:

```bash
# Piping a otro comando
q-secret get pi ANTHROPIC_API_KEY | pbcopy  # macOS
q-secret get pi ANTHROPIC_API_KEY | clip     # Windows
```

### `q-secret update`

```
q-secret update <project> <key> <new-value>
```

Falla si no existe la key. Mensaje: `error: key "X" not found in project "Y". Use "q-secret add" to create it.`

### `q-secret delete`

```
q-secret delete <project> <key>       # Borra un secret
q-secret delete <project>              # Borra todo el proyecto (pide confirmación)
q-secret delete <project> --force      # Borra sin confirmar
```

---

## 10. Seguridad

### Threat model

| Actor | Puede hacer | No puede hacer |
|-------|------------|----------------|
| Usuario legítimo | Todo | N/A |
| Otro proceso mismo usuario | Leer env vars del proceso hijito en `/proc/[pid]/environ` | Leer la DB encriptada sin master key |
| Atacante con acceso al disco | Leer la DB (nombres de proyectos y keys visibles) | Desencriptar valores sin master key |
| Atacante con acceso al keychain | Leer master key y desencriptar todo | Depende de la protección del keychain del SO |

### Buenas prácticas

1. **Master key bajo control del usuario**: él decide dónde la guarda (Bitwarden, keychain, papel)
2. **Temp files para age con permiso 600**: se borran con `defer`
3. **No hay `.env` en disco**: inyección directa por env vars al proceso hijo
4. **Exit code propagation**: keyring falla con el mismo código que el proceso hijo
5. **No almacenar la master key en la misma DB que los secrets**: separación física de responsabilidades

---

## 11. Build y distribución

### Compilación

```bash
# Local
go build -o q-secret ./main.go

# Cross-compile
GOOS=linux GOARCH=amd64 go build -o bin/q-secret-linux ./main.go
GOOS=darwin GOARCH=amd64 go build -o bin/q-secret-darwin ./main.go
GOOS=windows GOARCH=amd64 go build -o bin/q-secret.exe ./main.go
```

### Makefile

```makefile
.PHONY: build clean test

build:
	go build -o q-secret .

build-all:
	GOOS=linux GOARCH=amd64 go build -o bin/q-secret-linux .
	GOOS=darwin GOARCH=amd64 go build -o bin/q-secret-darwin .
	GOOS=windows GOARCH=amd64 go build -o bin/q-secret.exe .

test:
	go test ./...

clean:
	rm -rf bin/ q-secret q-secret.exe
```

### Dependencias en runtime

| Dependencia | Por qué | Cómo se obtiene |
|-------------|---------|-----------------|
| **age** >= 1.2 | Encriptación | `scoop install age`, `brew install age`, `apt install age` |

---

## 12. Tests

### Unit tests

| Archivo | Qué testea |
|---------|------------|
| `internal/crypto_test.go` | Round-trip encrypt/decrypt con age |
| `internal/db_test.go` | CRUD en SQLite en memoria |
| `internal/keychain_test.go` | Mock del keychain |

### Integration tests

```go
func TestRunInjectsEnvVars(t *testing.T) {
    // Usar temp dir como home
    tmpHome := t.TempDir()
    t.Setenv("HOME", tmpHome)     // Linux/macOS
    t.Setenv("USERPROFILE", tmpHome) // Windows
    
    // Init
    runCLI("init", "--db-path", tmpHome+"/test.db", "--master-key", TEST_MASTER_KEY)
    
    // Add
    runCLI("add", "testproj", "MY_SECRET=hello")
    
    // Run + verify
    output, _ := runCLI("run", "testproj", "--", "printenv", "MY_SECRET")
    assert.Equal(t, "hello", strings.TrimSpace(output))
}
```

### Test helpers

```go
// testutil.go
var TEST_MASTER_KEY = "AGE-SECRET-KEY-1TEST...test-key-for-testing-purposes-only"

func runCLI(args ...string) (string, error) {
    // Ejecutar el binario compilado, o llamar a la función main internamente
}
```

---

## 13. Preguntas de diseño resueltas

| Pregunta | Decisión |
|----------|----------|
| ¿Dónde guardar la master key? | En el keychain del SO, o en `Q_SECRET_KEY` env var como fallback |
| ¿Cómo encriptar cada valor? | age --encrypt con la public key (derivada de la master) |
| ¿Cómo pasar la private key a age? | Temp file con permiso 600, borrado con defer |
| ¿Cómo organizar los secrets? | Por proyecto (un proyecto puede tener N secrets) |
| ¿Qué framework CLI? | Cobra (no Bubble Tea — q-secret no es una TUI) |
| ¿El usuario necesita age? | Sí. Es el motor de encriptación. |
| ¿Qué pasa si no hay keychain? | Env var `Q_SECRET_KEY` como fallback documentado |
