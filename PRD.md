# PRD: `q-secret` — CLI de gestión e inyección de secretos

## 1. Resumen ejecutivo

CLI multiplataforma que almacena secretos (API keys, tokens, credenciales) en una base de datos SQLite local encriptada con age, y los inyecta como variables de entorno al ejecutar un proceso hijo. El archivo `.db` puede sincronizarse manualmente (OneDrive, Dropbox, etc.) para backup y uso en múltiples máquinas.

Inspirado en `op run` (1Password), `bws run` (Bitwarden) e `infisical run`, pero:
- **100% local**: sin servidor, sin nube, sin cuenta
- **Zero dependencias externas**: age para crypto, SQLite para storage
- **Open source desde el día 1**

## 2. Problema que resuelve

- Tener API keys y tokens en texto plano en el filesystem
- Tener que desencriptar/borrar archivos manualmente cada sesión (SOPS + age manual)
- Depender de servicios cloud para secrets personales
- No existir una herramienta local, liviana y multiplataforma que haga `run --` con inyección

## 3. Usuario target

- Desarrollador individual
- Quiere sus API keys seguras pero accesibles
- Trabaja en Windows, a veces Linux/macOS
- No quiere pagar ni mantener un server

## 4. Funcionalidades

| ID | Feature | Prioridad | Descripción |
|----|---------|-----------|-------------|
| F1 | Init DB | P0 | Crear la base de datos encriptada y generar/clave master |
| F2 | CRUD secrets | P0 | Agregar, obtener, listar, actualizar, borrar secrets |
| F3 | Proyectos | P0 | Agrupar secrets por proyecto |
| F4 | Inject run | P0 | `q-secret run -- <cmd>` inyecta secrets del proyecto como env vars |
| F5 | Multiplataforma | P0 | Windows, Linux, macOS |
| F6 | Backup | P1 | Flag para exportar/importar DB |

## 5. No goals

- No es un gestor de contraseñas como Bitwarden (no tiene UI web, ni sharing, ni sync automático)
- No maneja rotación automática de secrets
- No tiene server remoto ni API REST
- No soporta equipos multi-usuario
- No reemplaza Vault para CI/CD corporativo

## 6. Experiencia de uso

### Setup inicial

```bash
$ q-secret init
# Crea ~/.config/q-secret/db
# Pide la master key (age private key) que el usuario pega desde su gestor
# O genera una nueva con age-keygen
```

### Agregar secrets

```bash
$ q-secret add pi ANTHROPIC_API_KEY=sk-ant-xxx
$ q-secret add pi OPENAI_API_KEY=sk-open-xxx
$ q-secret add opencode OPENAI_API_KEY=sk-open-xxx
```

### Listar

```bash
$ q-secret list
pi:
  ANTHROPIC_API_KEY
  OPENAI_API_KEY
opencode:
  OPENAI_API_KEY

$ q-secret list pi   # muestra valores truncados (últimos 4 chars)
```

### Inyectar y ejecutar

```bash
$ q-secret run pi -- python app.py
# Inyecta ANTHROPIC_API_KEY y OPENAI_API_KEY como env vars
# Ejecuta python app.py
# Al cerrar, las env vars mueren
```

## 7. Arquitectura high-level

```
┌──────────────────────────────────────┐
│          q-secret CLI                │
│                                      │
│  ┌──────────┐  ┌──────────────────┐  │
│  │ Comandos  │  │  Master key      │  │
│  │ (add,get, │──│ (la pega el user │  │
│  │  list,    │  │  desde su gestor)│  │
│  │  run,...) │  └──────────────────┘  │
│  └─────┬────┘                        │
│        │                              │
│  ┌─────▼────┐  ┌──────────────────┐  │
│  │  age      │  │  SQLite DB (.db) │  │
│  │ (encrypt /│──│ (almacena secrets│  │
│  │  decrypt) │  │  encriptados)    │  │
│  └──────────┘  └──────────────────┘  │
└──────────────────────────────────────┘
         │
         ▼
   Proceso hijito (env vars inyectadas)
```

## 8. Riesgos y mitigaciones

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| Perder master key | Alto — secrets irrecuperables | Backup en Bitwarden/gestor de contraseñas |
| SQLite corrupto | Medio — perder secrets de ese snapshot | Backup automático antes de escribir |
| Env vars legibles por otro proceso en la misma sesión | Bajo — riesgo teórico | No mitigable completamente, igual que BWS/1Password |
| Age no instalado en el sistema | Alto — no funciona | Documentar instalación de age como prerequisito |
