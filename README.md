# q-secret

**q-secret** es un CLI para gestionar e inyectar secretos (API keys, tokens, credenciales) de forma local, segura y transparente.

```
q-secret add pi ANTHROPIC_KEY=sk-ant-xxx
q-secret run pi -- opencode   # inyecta la key y ejecuta opencode
```

## Concepto

Los secrets se almacenan en una base de datos SQLite local. Los **valores** están encriptados con age (AES-256-GCM). Los **nombres** quedan visibles para poder listar proyectos sin desencriptar.

El archivo `.db` podés sincronizarlo con OneDrive/Dropbox para backup y uso en múltiples máquinas, sin exponer los valores reales.

## Prerequisitos

- [age](https://github.com/FiloSottile/age) ≥ 1.2 (o [rage](https://github.com/str4d/rage), la implementación en Rust)
  ```bash
  # Windows (scoop)
  scoop install rage
  # o
  scoop install age

  # macOS
  brew install age

  # Linux
  apt install age
  ```

## Instalación

```bash
go install github.com/iquantum/q-secret@latest
```

O descargá el binario de [releases](https://github.com/iquantum/q-secret/releases).

## Quick start

```bash
# 1. Generar una age key (si no tenés una)
age-keygen -o ~/.config/q-secret/keys.txt

# 2. Inicializar la base de datos
q-secret init --master-key "$(cat ~/.config/q-secret/keys.txt)"

# 3. Agregar secrets a un proyecto
q-secret add pi ANTHROPIC_KEY=sk-ant-xxx

# 4. Ejecutar un programa con los secrets inyectados
q-secret run pi -- opencode

# 5. Listar proyectos y secrets
q-secret list
q-secret list pi
```

## Uso

### `q-secret init`

Inicializa la base de datos y configura la master key.

```bash
q-secret init [--master-key "AGE-SECRET-KEY-1..."] [--db-path ~/.config/q-secret/db]
```

Sin `--master-key`, el CLI te guía: generar una nueva key o pegar una existente.

### `q-secret add`

Agrega uno o más secrets a un proyecto (se crea automáticamente si no existe).

```bash
q-secret add <project> <key>=<value> [...]

q-secret add pi ANTHROPIC_KEY=sk-ant-xxx
q-secret add pi OPENAI_KEY=sk-open-xxx DB_URL=postgres://...
```

### `q-secret list`

```bash
q-secret list              # lista proyectos
q-secret list <project>    # lista keys del proyecto con valores truncados
```

### `q-secret run`

Ejecuta un comando con los secrets del proyecto inyectados como variables de entorno.

```bash
q-secret run <project> -- <command> [args...]

q-secret run pi -- opencode
q-secret run pi -- python app.py
q-secret run pi -- docker-compose up
```

**Importante:** los flags de `q-secret` van antes del `--`.

```bash
q-secret run pi --db-path ~/mi.db -- opencode
```

### `q-secret get`

Obtiene el valor desencriptado de un secret. Útil para scripting.

```bash
q-secret get <project> <key>

q-secret get pi ANTHROPIC_KEY | clip          # Windows
q-secret get pi ANTHROPIC_KEY | pbcopy        # macOS
```

### `q-secret update`

```bash
q-secret update <project> <key> <new-value>
```

### `q-secret delete`

```bash
q-secret delete <project> <key>     # borra un secret
q-secret delete <project>           # borra todo el proyecto
```

## Cómo guardar la master key

La master key (age private key) se puede guardar:

1. **En el keychain del SO** (próximamente: Windows Credential Manager, macOS Keychain, Linux libsecret)
2. **En la variable de entorno `Q_SECRET_KEY`**
   ```bash
   # Lo ideal: agregar al perfil de la shell
   export Q_SECRET_KEY="AGE-SECRET-KEY-1..."


   # Windows PowerShell
   $env:Q_SECRET_KEY = "AGE-SECRET-KEY-1..."
   ```

3. **En un gestor de contraseñas** (Bitwarden, 1Password) y pegarla cuando se necesita

## Arquitectura

```
q-secret/
├── cmd/           # Comandos cobra
│   ├── root.go    # Raíz + flags globales
│   ├── init.go    # q-secret init
│   ├── add.go     # q-secret add
│   ├── get.go     # q-secret get
│   ├── list.go    # q-secret list
│   ├── update.go  # q-secret update
│   ├── delete.go  # q-secret delete
│   └── run.go     # q-secret run
├── internal/
│   ├── config.go  # Paths, defaults
│   ├── db.go      # SQLite CRUD
│   ├── crypto.go  # age encrypt/decrypt
│   ├── keychain.go# Master key management
│   └── inject.go  # Env var injection + exec
├── main.go
├── PRD.md
├── SPECS.md
├── tasks.md
└── README.md
```

## Stack

| Componente | Tecnología |
|-----------|------------|
| Lenguaje | Go 1.23+ |
| SQLite | modernc.org/sqlite (sin CGO) |
| Crypto | age / rage (subproceso) |
| CLI | spf13/cobra |
| Keychain | Zalando/go-keyring (próximamente) |

## Licencia

MIT
