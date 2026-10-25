# q-secrets

**q-secrets** es un CLI para gestionar e inyectar secretos (API keys, tokens, credenciales) de forma local, segura y transparente.

```
q-secrets add pi ANTHROPIC_KEY
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
| API keys en texto plano en el disco | Se encriptan con **age** (X25519 + ChaCha20-Poly1305) |
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

### macOS — Homebrew

```bash
brew tap QuantumEdu/tap
brew install q-secrets
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

El binario incluye `filippo.io/age`: no necesita ejecutables age/rage ni acceso a red.
Para compilar: Go 1.26.1 o superior. Mantiene el formato de cifrado age compatible
con los secretos existentes. La master key sigue en el keychain del SO o `Q_SECRET_KEY`.

---

## Quick start

```bash
# 1. Inicializar la DB y generar la key en memoria
q-secrets init
# Respaldá la key que muestra init en tu gestor de contraseñas.

# 3. Agregar secrets a un proyecto
q-secrets add pi ANTHROPIC_KEY

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
q-secrets add <project> <KEY>            # prompt oculto en terminal
q-secrets add <project> <KEY> --stdin    # valor exacto hasta EOF

q-secrets add pi ANTHROPIC_KEY
secret-source | q-secrets add pi OPENAI_KEY --stdin
# Compatibilidad explícita; expone valores en argv e historial:
q-secrets add pi --allow-argv KEY=VALUE OTHER=value
```

El proyecto se crea automáticamente. `--stdin` conserva saltos de línea y permite
valores vacíos; evitá `echo` si no querés guardar un salto final. `--stdin` y
`--allow-argv` son excluyentes. Los nombres deben cumplir `[A-Za-z_][A-Za-z0-9_]*`;
`Q_SECRET_KEY` está reservado, incluso si se intenta importarlo o inyectarlo desde
una DB antigua. No se guardan nombres inválidos.

### `q-secrets list`

```bash
q-secrets list                       # tabla de proyectos y nombres
q-secrets list pi --output json      # metadatos, sin valores
```

`list` nunca desencripta ni muestra fragmentos de secretos, tampoco en JSON.
Para obtener valores, usá `get` o `export` explícitamente. `list --output env`
se rechaza porque una lista de nombres no es una exportación de valores.

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

El proceso hijo hereda stdin/stdout/stderr. Los secretos reemplazan las variables
heredadas con el mismo nombre, sin duplicados. `Q_SECRET_KEY` se elimina siempre.

```bash
q-secrets run pi --clean-env -- opencode
q-secrets run pi --no-inherit-env -- python app.py
```

En modo limpio sólo se hereda `PATH`; en Windows también `SystemRoot`, `WINDIR`,
`COMSPEC`, `PATHEXT`, `TEMP` y `TMP`. Los secretos del proyecto se agregan después.
No se heredan HOME, credenciales del padre ni configuraciones proxy; si tu programa
las necesita, usá el modo normal o configuralas en el proyecto deliberadamente.
Los flags deben aparecer antes de `--`.

Se conserva el código de salida del hijo. En Unix se reenvían INT/TERM/HUP;
una terminación por señal devuelve `128 + señal`. La cancelación espera hasta
3 segundos antes de forzar el cierre y recoge el proceso. Los hijos sin terminal
usan un grupo de procesos para detener también descendientes. Los interactivos
permanecen en el grupo de la terminal para poder leer stdin; la señal de terminal
alcanza ese grupo, y las señales enviadas al wrapper se reenvían al hijo directo.
Windows termina el hijo directo: no ofrece la misma propagación POSIX ni garantiza
el cierre de descendientes.

#### Watch mode (`--watch`)

```bash
q-secrets run pi --watch -- myapp
```

Con `--watch`, q-secrets monitorea la base de datos y su WAL de SQLite por cambios del proyecto. No reinicia por escrituras de otros proyectos ni por valores idénticos. Si agregás, actualizás o borrás un secret mientras el proceso hijo está corriendo, lo reinicia automáticamente:

```
$ q-secrets run pi --watch -- myapp
  ... (myapp running) ...
  [agregás un secret nuevo en otra terminal]
secrets changed, restarting...
  ... (myapp reiniciado con los nuevos secrets) ...
```

Ideal para desarrollo: arrancás tu app con `--watch`, y cada vez que configurás una key nueva se reinicia sola sin que tengas que hacer nada.

### `q-secrets get`

```bash
q-secrets get <project> <key>

q-secrets get pi ANTHROPIC_KEY | clip            # Windows
q-secrets get pi ANTHROPIC_KEY | pbcopy          # macOS
q-secrets get pi ANTHROPIC_KEY                   # stdout
```

### `q-secrets update`

```bash
q-secrets update <project> <KEY>           # prompt oculto
secret-source | q-secrets update <project> <KEY> --stdin
# Sólo para compatibilidad:
q-secrets update <project> <KEY> <new-value> --allow-argv
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
q-secrets export --file backup.json
q-secrets export --project pi --output env
q-secrets export --project pi --output table

# Importar desde JSON (re-encripta)
q-secrets import backup.json
cat backup.json | q-secrets import
```

`export --output` elige `json|env|table` (JSON por defecto); para archivos usá
`--file` o `-o`. El archivo se crea con permisos 0600 y no sobrescribe archivos
existentes ni sigue symlinks. El formato `env` usa comillas POSIX para conservar
comillas, espacios y saltos de línea, y rechaza claves repetidas entre proyectos;
usá `--project` para resolver ambigüedades. Import sólo acepta el JSON de export.

`get --output json|env|table` usa esos mismos formatos. Sin `--output`, `get`
conserva la salida raw, sin agregar un salto de línea.

**El export contiene los valores en texto plano.** Borrá el archivo después de usarlo.

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
q-secrets add openrouter OPENROUTER_API_KEY
q-secrets add openrouter OPENAI_API_KEY

# Usar (todos los días)
q-secrets run openrouter -- opencode
```

### 2. Servicio con .env

```bash
# Agregar credenciales de DB
q-secrets add app DB_HOST
q-secrets add app DB_USER
q-secrets add app DB_PASSWORD

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
q-secrets run pi -- myapp
./deploy.sh
```

---

## Seguridad

No se escriben plaintext ni private keys en temporales durante cifrado o
descifrado. Eso no garantiza borrado de memoria: Go, swap, core dumps y el propio
proceso hijo pueden conservar copias. `init` muestra la nueva key para respaldarla;
`init --master-key` sigue siendo una opción heredada que expone la key en argv.
Usá el modo interactivo para generar una nueva key.

### Modelo de amenazas

| Actor | Puede hacer | No puede hacer |
|-------|------------|----------------|
| Vos (usuario legítimo) | Todo | N/A |
| Otro proceso mismo usuario | Leer env vars de procesos en `/proc/[pid]/environ` | Leer la DB sin la master key |
| Alguien con acceso a tu disco | Ver proyectos y keys en la DB | Desencriptar valores sin la master key |
| Alguien con tu sesión de Windows | Si la master key está en el keychain, puede desencriptar | — |

### La master key

- **La tenés que guardar vos** en Bitwarden, 1Password, un USB, o un papel
- q-secrets la lee de la variable `Q_SECRET_KEY` o del keychain del sistema operativo
- En `q-secrets init`, la master key se guarda automáticamente en el keychain:
  - **Windows:** Windows Credential Manager (service `q-secrets`)
  - **macOS:** macOS Keychain (service `q-secrets`)
  - **Linux:** libsecret / gnome-keyring (`secret-tool` debe estar corriendo)
- `Q_SECRET_KEY` tiene prioridad: si está seteada, ignora el keychain (útil para CI y scripting)
- **Si la perdés, no hay recovery.** Hacé backup.

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
| Keychain del SO | ✅ Listo | Guardar master key en Windows Credential Manager, macOS Keychain, Linux libsecret |
| `--watch` mode | ✅ Listo | Reiniciar proceso hijo si cambian los secrets |
| GitHub Actions | ✅ Listo | Build cross-platform automático con GoReleaser |
| Scoop bucket | ✅ Listo | `scoop install q-secrets` |
| Homebrew tap | ✅ Listo | `brew install q-secrets` para usuarios macOS |
| Caché offline | 🟡 Pendiente | Cachear secrets para ejecución sin acceso a la DB |

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
│   ├── crypto.go  # age en memoria, sin archivos temporales
│   ├── keychain.go# Master key management
│   └── inject.go  # Env var injection + exec
├── main.go
└── [docs: PRD.md, SPECS.md, tasks.md]
```

**Stack:**
- **Go 1.26** — binario único, sin runtime
- **modernc.org/sqlite** — SQLite puro Go, sin CGO
- **filippo.io/age** — cifrado en memoria, compatible con age/rage
- **cobra** — CLI framework

---

## Contribuir

```bash
git clone https://github.com/QuantumEdu/q-secrets.git
cd q-secrets
go build ./...
go test -race ./... -count=1 -timeout 60s
go vet ./...
# Integración Linux: vault temporal aislado y secretos sintéticos
python3 tests/cli_hardening.py ./q-secrets
# Opcional: compatibilidad con el CLI oficial de age
python3 tests/cli_hardening.py ./q-secrets /path/to/age
```

PRs bienvenidas. Mantené la cobertura de tests y seguí la estructura de `internal/` y `cmd/`.

---

## Licencia

MIT © QuantumEdu
