# v0.2 hardening — implementación y migración

Se conserva el diseño local, individual y offline: SQLite, cifrado por secreto,
master key en keychain del SO o `Q_SECRET_KEY`. No se agregan servidor, RBAC,
organizaciones, cuentas de servicio, UUID ni proveedor remoto.

| Hallazgo | Implementación |
| --- | --- |
| Valores en argv | `add` y `update` usan prompt oculto o `--stdin`. Legacy requiere `--allow-argv`. Los errores de entrada no repiten valores. |
| Temporales de plaintext y private key | `filippo.io/age` cifra y descifra en memoria. No ejecuta age/rage ni escribe temporales. Generación y derivación de keys también son in-process. |
| Formato existente | Se conservan X25519 y el formato age; se acepta ciphertext binario y armored. La derivación no confía en comentarios de public key. |
| Herencia de master key | Se elimina `Q_SECRET_KEY` del entorno heredado, sin distinguir mayúsculas. Se reserva ese nombre al guardar y al inyectar, también para DBs antiguas. |
| Entorno limpio | `run --clean-env` / `--no-inherit-env`, aplicados también a watch. Unix conserva PATH; Windows además SystemRoot, WINDIR, COMSPEC, PATHEXT, TEMP y TMP. |
| Nombres y colisiones | Validación POSIX en add/update/import/UpsertSecret y BuildEnv. Se mantiene UNIQUE(project_id,key_name); se deduplica el entorno heredado y el proyecto tiene precedencia. Windows detecta colisiones sin distinguir mayúsculas. Valores con NUL se rechazan al inyectar. |
| Ciclo de vida del hijo | Las librerías retornan errores en vez de llamar os.Exit. El CLI conserva exit codes. En Unix se reenvían INT/TERM/HUP, se representa una muerte por señal como 128+señal y se escala a kill tras 3 s en shutdown. |
| Watch | Se observa el directorio para detectar DB y WAL, se comparan los valores del proyecto, se espera la salida antes de reiniciar y se conservan las mismas reglas de entorno. |
| Salidas | get/export: json, env y table; list: json/table de metadatos, sin descifrar. get sin formato conserva bytes raw. Env usa quoting POSIX y rechaza colisiones entre proyectos antes de escribir. |
| Exposición adicional | Se corrige list, que mostraba casi todo el valor al intentar truncarlo. Los errores del keychain ya no incluyen la master key. |

## Migración de comandos

```sh
# Antes: add proyecto TOKEN=valor
q-secrets add proyecto TOKEN                 # prompt oculto
secret-source | q-secrets add proyecto TOKEN --stdin
q-secrets add proyecto --allow-argv TOKEN=valor  # compatibilidad explícita

# Antes: update proyecto TOKEN valor
secret-source | q-secrets update proyecto TOKEN --stdin
q-secrets update proyecto TOKEN valor --allow-argv

q-secrets run proyecto --clean-env -- myapp
q-secrets list proyecto --output json
q-secrets get proyecto TOKEN --output env
q-secrets export --project proyecto --output json

# --output ahora elige formato. --file/-o elige archivo.
q-secrets export --file backup.json
```

`--stdin` lee hasta EOF y conserva saltos finales y valores vacíos. El formato
JSON de export sigue siendo importable. Los archivos de export se crean con 0600
y se rechazan destinos existentes (incluidos symlinks). En Windows el acceso a
archivos depende además de las ACL del directorio; 0600 no equivale a una ACL Unix.
La salida env está pensada para shells POSIX, no para PowerShell o cmd.exe.

## Verificación reproducible

```sh
go test -race ./...
go vet ./...
go build -o q-secrets .
python3 tests/cli_hardening.py ./q-secrets /path/to/age
GOOS=windows GOARCH=amd64 go build -o q-secrets.exe .
```

La suite Go cubre cifrado sin PATH ni directorio temporal disponible, claves
incorrectas, integridad del ciphertext, formato armored, validación de nombres,
DBs antiguas, filtrado de entorno, exit codes y cancelación con escalado.
La suite Linux usa sólo un vault temporal y un socket D-Bus inexistente para
no escribir en el keychain del usuario. Comprueba prompt por PTY sin eco,
lectura exacta de stdin, JSON/env, exportación segura, códigos de salida y
señales en modo normal/watch, aislamiento del entorno y reinicios reales por WAL.
Con un binario age comprueba descifrado de ambos formatos en ambos sentidos.

Se ejecutaron con éxito Go test con race, vet, build Linux, build cruzado Windows
y la suite Linux con el CLI oficial age v1.3.1, tanto con el binario directo
como mediante `skillvault secrets`. También pasaron las pruebas de resolución
del binario y sustitución del wrapper en q-skillvault. En Unix el wrapper ahora
usa exec para que q-secrets reciba señales directamente y conserve el PID;
Windows conserva el wrapper por subprocess. La compilación cruzada no sustituye
pruebas de ejecución ni de keychain nativo en Windows/macOS.

## Límites y pendientes explícitos

- Go no garantiza borrado seguro de memoria; swap y core dumps siguen fuera de
  las garantías. Los valores inyectados son accesibles al proceso hijo y a su
  contexto de usuario, como en el diseño original.
- `init` muestra una nueva master key para respaldo. La opción heredada
  `init --master-key` y la entrada interactiva de una key existente mantienen
  su comportamiento anterior; no constituyen una nueva vía de entrada segura.
- En Unix los hijos sin terminal usan un grupo separado. Los interactivos
  permanecen en el grupo de la terminal para poder leer stdin. Las señales
  explícitas al wrapper se reenvían al hijo directo en ese caso.
- En Windows se termina el hijo directo; no se implementa semántica POSIX ni
  gestión de árboles con Job Objects. No se promete eliminar descendientes.
- No se publica una versión ni se cambia el puntero del submódulo a un commit
  inexistente: primero debe publicarse el cambio de q-secrets y después fijar
  ese commit en q-skillvault.
