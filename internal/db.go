package internal

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Secret representa un secret almacenado (valor encriptado)
type Secret struct {
	ID        int64
	ProjectID string
	KeyName   string
	ValueEnc  string
	Created   string
	Updated   string
}

// DB wrapper sobre sql.DB
type DB struct {
	conn *sql.DB
}

// OpenDB abre o crea la base de datos en path, ejecuta migrations
func OpenDB(path string) (*DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening db: %w", err)
	}

	// WAL mode para mejor concurrencia
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}

	// Foreign keys on (needed for DELETE CASCADE)
	if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS projects (
			id      TEXT PRIMARY KEY,
			created TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS secrets (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			key_name   TEXT NOT NULL,
			value_enc  TEXT NOT NULL,
			created    TEXT NOT NULL,
			updated    TEXT NOT NULL,
			UNIQUE(project_id, key_name)
		);

		CREATE INDEX IF NOT EXISTS idx_secrets_project ON secrets(project_id);
	`)
	return err
}

// CreateProject crea un proyecto si no existe
func (db *DB) CreateProject(name string) error {
	_, err := db.conn.Exec(
		"INSERT OR IGNORE INTO projects (id, created) VALUES (?, ?)",
		name, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// UpsertSecret inserta o actualiza un secret
func (db *DB) UpsertSecret(project, key string, valueEnc []byte) error {
	now := time.Now().UTC().Format(time.RFC3339)

	// Asegurar que el proyecto existe
	if err := db.CreateProject(project); err != nil {
		return err
	}

	_, err := db.conn.Exec(`
		INSERT INTO secrets (project_id, key_name, value_enc, created, updated)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(project_id, key_name) DO UPDATE SET
			value_enc = excluded.value_enc,
			updated = excluded.updated
	`, project, key, string(valueEnc), now, now)

	return err
}

// GetSecret obtiene el valor encriptado de un secret
func (db *DB) GetSecret(project, key string) ([]byte, error) {
	var valueEnc string
	err := db.conn.QueryRow(
		"SELECT value_enc FROM secrets WHERE project_id = ? AND key_name = ?",
		project, key,
	).Scan(&valueEnc)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("secret %q not found in project %q", key, project)
	}
	if err != nil {
		return nil, err
	}

	return []byte(valueEnc), nil
}

// ListProjects lista todos los proyectos
func (db *DB) ListProjects() ([]string, error) {
	rows, err := db.conn.Query("SELECT id FROM projects ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// ListSecrets lista los secrets de un proyecto (sin desencriptar)
func (db *DB) ListSecrets(project string) ([]Secret, error) {
	rows, err := db.conn.Query(
		"SELECT id, project_id, key_name, value_enc, created, updated FROM secrets WHERE project_id = ? ORDER BY key_name",
		project,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var secrets []Secret
	for rows.Next() {
		var s Secret
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.KeyName, &s.ValueEnc, &s.Created, &s.Updated); err != nil {
			return nil, err
		}
		secrets = append(secrets, s)
	}
	return secrets, rows.Err()
}

// DeleteSecret borra un secret específico
func (db *DB) DeleteSecret(project, key string) error {
	res, err := db.conn.Exec(
		"DELETE FROM secrets WHERE project_id = ? AND key_name = ?",
		project, key,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("secret %q not found in project %q", key, project)
	}
	return nil
}

// DeleteProject borra un proyecto y todos sus secrets (CASCADE)
func (db *DB) DeleteProject(project string) error {
	_, err := db.conn.Exec("DELETE FROM projects WHERE id = ?", project)
	return err
}

// ProjectExists verifica si un proyecto existe
func (db *DB) ProjectExists(project string) (bool, error) {
	var count int
	err := db.conn.QueryRow(
		"SELECT COUNT(*) FROM projects WHERE id = ?", project,
	).Scan(&count)
	return count > 0, err
}
