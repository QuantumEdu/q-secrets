package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) (*DB, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := OpenDB(path)
	if err != nil {
		t.Fatalf("OpenDB() error: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db, path
}

func TestOpenDBCreatesFile(t *testing.T) {
	db, path := setupTestDB(t)
	db.Close()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestCreateProject(t *testing.T) {
	db, _ := setupTestDB(t)

	if err := db.CreateProject("testproj"); err != nil {
		t.Fatalf("CreateProject() error: %v", err)
	}

	// Crear el mismo proyecto de nuevo no debe fallar
	if err := db.CreateProject("testproj"); err != nil {
		t.Fatalf("CreateProject() duplicate error: %v", err)
	}
}

func TestUpsertAndGetSecret(t *testing.T) {
	db, _ := setupTestDB(t)

	value := []byte("encrypted-blob-here")
	if err := db.UpsertSecret("testproj", "MY_KEY", value); err != nil {
		t.Fatalf("UpsertSecret() error: %v", err)
	}

	got, err := db.GetSecret("testproj", "MY_KEY")
	if err != nil {
		t.Fatalf("GetSecret() error: %v", err)
	}
	if string(got) != string(value) {
		t.Errorf("got %q, want %q", string(got), string(value))
	}
}

func TestUpsertOverwrite(t *testing.T) {
	db, _ := setupTestDB(t)

	db.UpsertSecret("proj", "K", []byte("v1"))
	db.UpsertSecret("proj", "K", []byte("v2"))

	got, _ := db.GetSecret("proj", "K")
	if string(got) != "v2" {
		t.Errorf("expected updated value %q, got %q", "v2", string(got))
	}
}

func TestGetSecretNotFound(t *testing.T) {
	db, _ := setupTestDB(t)

	_, err := db.GetSecret("nonexistent", "KEY")
	if err == nil {
		t.Fatal("expected error for nonexistent secret, got nil")
	}
}

func TestListProjects(t *testing.T) {
	db, _ := setupTestDB(t)

	db.CreateProject("alpha")
	db.CreateProject("beta")
	db.CreateProject("gamma")

	projects, err := db.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects() error: %v", err)
	}

	if len(projects) != 3 {
		t.Errorf("expected 3 projects, got %d: %v", len(projects), projects)
	}
}

func TestListSecrets(t *testing.T) {
	db, _ := setupTestDB(t)

	db.UpsertSecret("proj", "A", []byte("encA"))
	db.UpsertSecret("proj", "B", []byte("encB"))
	db.UpsertSecret("proj", "C", []byte("encC"))

	secrets, err := db.ListSecrets("proj")
	if err != nil {
		t.Fatalf("ListSecrets() error: %v", err)
	}

	if len(secrets) != 3 {
		t.Errorf("expected 3 secrets, got %d", len(secrets))
	}

	for _, s := range secrets {
		if s.ProjectID != "proj" {
			t.Errorf("secret %s has wrong project: %s", s.KeyName, s.ProjectID)
		}
	}
}

func TestDeleteSecret(t *testing.T) {
	db, _ := setupTestDB(t)

	db.UpsertSecret("proj", "K", []byte("val"))

	if err := db.DeleteSecret("proj", "K"); err != nil {
		t.Fatalf("DeleteSecret() error: %v", err)
	}

	_, err := db.GetSecret("proj", "K")
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestDeleteProject(t *testing.T) {
	db, _ := setupTestDB(t)

	db.UpsertSecret("proj", "A", []byte("val1"))
	db.UpsertSecret("proj", "B", []byte("val2"))

	if err := db.DeleteProject("proj"); err != nil {
		t.Fatalf("DeleteProject() error: %v", err)
	}

	secrets, _ := db.ListSecrets("proj")
	if len(secrets) != 0 {
		t.Errorf("expected 0 secrets after project deletion, got %d", len(secrets))
	}

	exists, _ := db.ProjectExists("proj")
	if exists {
		t.Error("project should not exist after deletion")
	}
}

func TestProjectExists(t *testing.T) {
	db, _ := setupTestDB(t)

	exists, _ := db.ProjectExists("missing")
	if exists {
		t.Error("expected false for nonexistent project")
	}

	db.CreateProject("exists")
	exists, _ = db.ProjectExists("exists")
	if !exists {
		t.Error("expected true for existing project")
	}
}

func TestCreateProjectAutoOnUpsert(t *testing.T) {
	db, _ := setupTestDB(t)

	// Upsert en proyecto que no existe debe crearlo
	db.UpsertSecret("auto-created", "K", []byte("v"))

	exists, _ := db.ProjectExists("auto-created")
	if !exists {
		t.Error("project should be auto-created on UpsertSecret")
	}
}

func TestMultipleProjects(t *testing.T) {
	db, _ := setupTestDB(t)

	db.UpsertSecret("p1", "K1", []byte("v1"))
	db.UpsertSecret("p2", "K2", []byte("v2"))
	db.UpsertSecret("p1", "K3", []byte("v3"))

	p1Secrets, _ := db.ListSecrets("p1")
	if len(p1Secrets) != 2 {
		t.Errorf("expected 2 secrets in p1, got %d", len(p1Secrets))
	}

	p2Secrets, _ := db.ListSecrets("p2")
	if len(p2Secrets) != 1 {
		t.Errorf("expected 1 secret in p2, got %d", len(p2Secrets))
	}
}
