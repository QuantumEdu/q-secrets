# q-secrets

**q-secrets** es un CLI para gestionar e inyectar secretos (API keys, tokens, credenciales) de forma local, segura y transparente.

```
q-secrets add pi ANTHROPIC_KEY=sk-ant-xxx
q-secrets run pi -- opencode   # inyecta la key y ejecuta opencode
```

---

## Tabla de contenidos

- [¿Qué resuelve?](#qué-resuelve)
- [Comparativa con alternativas](#comparativa-con-alternativas)
- [Instalación](#instalación)
- [Prerequisitos](#prerequisitos)
- [Quick start](#quick-start)
- [Uso completo](#uso-completo)
- [Casos de uso](#casos-de-uso)
- [Seguridad](#seguridad)
- [Roadmap / Mejoras pendientes](#roadmap--mejoras-pendientes)
- [Contribuir](#contribuir)
- [Licencia](#licencia)

---

## ¿Qué resuelve?

Si trabajás con APIs, tenés un archivo `.env` con tus keys, o pegás tokens cada vez que ejecutás una herramienta, q-secrets resuelve:

| Problema | Cómo lo resuelve |
|----------|------------------|
| API keys en texto plano en el disco | Se encriptan con **age** (AES-256-GCM) |
| Tener que desencriptar/borrar archivos cada sesión | `q-secrets run --` inyecta los valores como env vars |
| Sincronizar secrets entre PCs | La DB encriptada se puede subir a OneDrive/Dropbox |
| Recordar qué keys usás y para qué | Organización por **proyectos** con `list` |

### ¿Qué NO cubre?

- **No** es un gestor de contraseñas tipo Bitwarden (no tiene sharing multi-usuario)
- **No** tiene servidor ni API REST
- **No** hace rotación automática de secrets
- **No** está pensado para CI/CD corporativo ni equipos grandes

---

## Comparativa con alternativas

| Herramienta | Local | Cloud | Inyección | Gratis | Open Source |
|-------------|-------|-------|-----------|--------|-------------|
| **q-secrets** | ✅ 100% | ❌ | ✅ `run --` | ✅ | ✅ MIT |
| **SOPS + age** | ✅ | ❌ | ❌ manual | ✅ | ✅ |
| **1Password CLI** | ❌ | ✅ | ✅ `op run --` | ❌ ~$4/mes | ❌ |
| **Bitwarden BWS** | ❌ | ✅ | ✅ `bws run --` | ✅ (limitado) | ✅ |
| **Infisical** | ❌ | ✅/self-host | ✅ `infisical run --` | ✅ (limitado) | ✅ MIT |
| **Windows Credential Manager** | ✅ | ❌ | ❌ scripting | ✅ | N/A |
| **Env vars del SO** | ✅ | ❌ | ✅ automático | ✅ | N/A |

**q-secrets está en el medio:** más automático que SOPS (no requiere decrypt manual), más liviano que Infisical (sin server), y completamente offline y open source.

---

## Instalación

### Windows — Scoop (recomendado)

```powershell
scoop bucket add q-secrets https://github.com/QuantumEdu/scoop-q-secrets
scoop install q-secrets/q-secrets
```

### go install

```bash
go install github.com/QuantumEdu/q-secrets@latest
```

### Binario (cualquier OS)

Descargar desde [releases](https://github.com/QuantumEdu/q-secrets/releases):

```bash
# Linux / macOS
curl -LO https://github.com/QuantumEdu/q-secrets/releases/latest/download/q-secrets_linux_amd64.tar.gz
tar -xzf q-secrets_linux_amd64.tar.gz
sudo mv q-secrets /usr/local/bin/

# Windows (PowerShell)
curl -LO https://github.com/QuantumEdu/q-secrets/releases/latest/download/q-secrets_windows_amd64.zip
Expand-Archive q-secrets_windows_amd64.zip -DestinationPath .
Move-Item q-secrets.exe C:\Users\iQuantum\bin\
```

### Build desde source

```bash
git clone https://github.com/QuantumEdu/q-secrets.git
cd q-secrets
go build -o q-secrets .
```

---

## Prerequisitos

- **[age](https://github.com/FiloSottile/age)** ≥ 1.2 (o **[rage](https://github.com/str4d/rage)**, la implementación en Rust)

```powershell
# Windows (Scoop)
scoop install age

# o también funciona con rage
scoop install rage
```

```bash
# macOS
brew install age

# Linux (Debian/Ubuntu)
apt install age
```

Si instalaste `rage` en vez de `age`, q-secrets lo detecta automáticamente. No necesitás ambos.

---

## Quick start

```bash
# 1. Generar una age key
age-keygen -o ~/.config/q-secrets/keys.txt
# Guardá esa key en Bitwarden/1Password!

# 2. Inicializar la DB
q-secrets init --master-key "$(cat ~/.config/q-secrets/keys.txt)"

# 3. Agregar secrets a un proyecto
q-secrets add pi ANTHROPIC_KEY=sk-ant-xxx

# 4. Ejecutar un programa con los secrets inyectados
q-secrets run pi -- opencode

# 5. Listar
q-secrets list
q-secrets list pi
```

---

## Uso completo

### `q-secrets init`

```bash
q-secrets init [--master-key "AGE-SECRET-KEY-1..."] [--db-path ~/.config/q-secrets/db] [--force]
```

- Sin `--master-key`: modo interactivo (generar key nueva o pegar existente)
- Con `--master-key`: modo scripteable
- `--force`: sobrescribe DB existente
- `--db-path`: ruta custom de la DB

### `q-secrets add`

```bash
q-secrets add <project> <key>=<value> [...]

q-secrets add pi ANTHROPIC_KEY=sk-ant-xxx
q-secrets add pi OPENAI_KEY=sk-open-xxx DB_URL=postgres://...
q-secrets add opencode OPENAI_KEY=sk-open-xxx
```

El proyecto se crea automáticamente si no existe.

### `q-secrets list`

```bash
q-secrets list              # lista proyectos con cantidad de secrets
q-secrets list <project>    # lista keys + valores truncados (últimos 4 chars)
```

### `q-secrets run`

```bash
q-secrets run <project> -- <command> [args...]
```

**Flags antes de `--`, siempre:**

```bash
q-secrets run pi -- opencode
q-secrets run pi --db-path ~/mi.db -- python app.py
q-secrets run pi -- docker-compose up
q-secrets run pi -- npm run dev
```

El proceso hijo hereda stdin/stdout/stderr y las env vars del padre, más los secrets del proyecto.

### `q-secrets get`

```bash
q-secrets get <project> <key>

q-secrets get pi ANTHROPIC_KEY | clip            # Windows
q-secrets get pi ANTHROPIC_KEY | pbcopy          # macOS
q-secrets get pi ANTHROPIC_KEY                   # stdout
```

### `q-secrets update`

```bash
q-secrets update <project> <key> <new-value>
```

### `q-secrets delete`

```bash
q-secrets delete <project> <key>     # borra un secret
q-secrets delete <project>           # borra todo el proyecto (pide confirmación)
q-secrets delete <project> --force   # borra sin confirmar
```

### `q-secrets export / import`

```bash
# Exportar todos los secrets desencriptados a JSON
q-secrets export > backup.json
q-secrets export -o backup.json

# Importar desde JSON (re-encripta)
q-secrets import backup.json
cat backup.json | q-secrets import
```

**⚠️ El export contiene los valores en texto plano.** Borrá el archivo después de usarlo.

### `q-secrets version`

```bash
q-secrets version
# q-secrets 0.1.0 (commit: abc123, built: 2026-05-26)
```

### `q-secrets completion`

```bash
# Bash
source <(q-secrets completion bash)

# Zsh
source <(q-secrets completion zsh)

# PowerShell
q-secrets completion powershell | Out-String | Invoke-Expression
```

### Flags globales

| Flag | Default | Descripción |
|------|---------|-------------|
| `--db-path` | `~/.config/q-secrets/q-secrets.db` | Ruta de la base de datos |
| `-h, --help` | | Ayuda del comando |

---

## Casos de uso

### 1. Desarrollador local con API keys

```bash
# Setup (una vez)
q-secrets add openrouter OPENROUTER_API_KEY=sk-or-xxx
q-secrets add openrouter OPENAI_API_KEY=sk-open-xxx

# Usar (todos los días)
q-secrets run openrouter -- opencode
```

### 2. Servicio con .env

```bash
# Agregar credenciales de DB
q-secrets add app DB_HOST=localhost
q-secrets add app DB_USER=admin
q-secrets add app DB_PASSWORD=SuperSecret123

# Levantar la app
q-secrets run app -- docker-compose up

# Sincronizar DB entre PCs
# 1. Copiar ~/.config/q-secrets/q-secrets.db a OneDrive
# 2. En la otra PC, copiar de OneDrive a ~/.config/q-secrets/
# 3. Setear Q_SECRET_KEY con la misma master key
```

### 3. Scripting

```bash
# En un script bash
export $(q-secrets get pi ANTHROPIC_KEY)
./deploy.sh
```

---

## Seguridad

### Modelo de amenazas

| Actor | Puede hacer | No puede hacer |
|-------|------------|----------------|
| Vos (usuario legítimo) | Todo | N/A |
| Otro proceso mismo usuario | Leer env vars de procesos en `/proc/[pid]/environ` | Leer la DB sin la master key |
| Alguien con acceso a tu disco | Ver proyectos y keys en la DB | Desencriptar valores sin la master key |
| Alguien con tu sesión de Windows | Si la master key está en el keychain, puede desencriptar | — |

### La master key

- **La tenés que guardar vos** en Bitwarden, 1Password, un USB, o un papel
- q-secrets la lee de la variable `Q_SECRET_KEY`
- **Si la perdés, no hay recovery.** Hacé backup.
- Si la DB se corrompe, perdés los secrets de ese snapshot. La master key te permite arrancar de nuevo.

### Buenas prácticas

1. **Respalda la master key** en un gestor de contraseñas antes de usarla
2. **Nunca compartas la DB** sin haber encriptado los valores (están encriptados por defecto)
3. **Borrá los archivos de export** inmediatamente después de usarlos
4. **Usá `--db-path`** para tener DBs separadas por proyecto si querés
5. Si trabajás en equipo, recordá que **q-secrets no tiene control de acceso** — es para uso individual

---

## Roadmap / Mejoras pendientes

| Feature | Estado | Descripción |
|---------|--------|-------------|
| Keychain del SO | 🟡 Pendiente | Guardar master key en Windows Credential Manager, macOS Keychain, Linux libsecret |
| `--watch` mode | 🟡 Pendiente | Reiniciar proceso hijito si cambian los secrets |
| GitHub Actions | ✅ Listo | Build cross-platform automático con GoReleaser |
| Scoop bucket | ✅ Listo | `scoop install q-secrets` |
| Homebrew tap | 🟡 Pendiente | Para usuarios macOS |
| Caché offline | 🟡 Pendiente | Cachear secrets para ejecución sin accesso a la DB |

---

## Arquitectura

```
q-secrets/
├── cmd/           # Comandos (cobra)
│   ├── root.go    # Raíz + flags globales
│   ├── init.go    # q-secrets init
│   ├── add.go     # q-secrets add
│   ├── get.go     # q-secrets get
│   ├── list.go    # q-secrets list
│   ├── update.go  # q-secrets update + delete
│   ├── export.go  # q-secrets export / import
│   └── run.go     # q-secrets run
├── internal/
│   ├── config.go  # Paths por OS
│   ├── db.go      # SQLite CRUD (modernc.org/sqlite)
│   ├── crypto.go  # age encrypt/decrypt (soporta age y rage)
│   ├── keychain.go# Master key management
│   └── inject.go  # Env var injection + exec
├── main.go
└── [docs: PRD.md, SPECS.md, tasks.md]
```

**Stack:**
- **Go 1.26** — binario único, sin runtime
- **modernc.org/sqlite** — SQLite puro Go, sin CGO
- **age/rage** — encriptación AES-256-GCM
- **cobra** — CLI framework

---

## Contribuir

```bash
git clone https://github.com/QuantumEdu/q-secrets.git
cd q-secrets
go build ./...
go test ./... -count=1 -timeout 60s
```

PRs bienvenidas. Mantené la cobertura de tests y seguí la estructura de `internal/` y `cmd/`.

---

## Licencia

MIT © QuantumEdu
