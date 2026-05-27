package internal

import (
	"os"
	"testing"
)

func TestGetMasterKeyFromEnv(t *testing.T) {
	// Guardar valor original y restaurarlo
	orig := os.Getenv("Q_SECRET_KEY")
	defer os.Setenv("Q_SECRET_KEY", orig)

	os.Setenv("Q_SECRET_KEY", testMasterKey)
	got, err := GetMasterKey()
	if err != nil {
		t.Fatalf("GetMasterKey() error: %v", err)
	}
	if got != testMasterKey {
		t.Errorf("got %q, want %q", got, testMasterKey)
	}
}

func TestGetMasterKeyMissing(t *testing.T) {
	orig := os.Getenv("Q_SECRET_KEY")
	defer os.Setenv("Q_SECRET_KEY", orig)

	os.Unsetenv("Q_SECRET_KEY")
	_, err := GetMasterKey()
	if err == nil {
		t.Error("expected error when no master key is configured")
	}
}

func TestDefaultConfigDir(t *testing.T) {
	dir := DefaultConfigDir()
	if dir == "" {
		t.Error("DefaultConfigDir() returned empty string")
	}
}

func TestReadWritePublicKey(t *testing.T) {
	// Backup y restore
	orig := PublicKeyPath
	defer func() { PublicKeyPath = orig }()

	tmpDir := t.TempDir()
	PublicKeyPath = tmpDir + "/public.key"

	if err := WritePublicKey(testPublicKey); err != nil {
		t.Fatalf("WritePublicKey() error: %v", err)
	}

	got, err := ReadPublicKey()
	if err != nil {
		t.Fatalf("ReadPublicKey() error: %v", err)
	}
	if got != testPublicKey {
		t.Errorf("got %q, want %q", got, testPublicKey)
	}
}

func TestInjectEnv(t *testing.T) {
	if findAge() == "" || findKeygen() == "" {
		t.Skip("age/rage or keygen not found in PATH")
	}

	orig := os.Getenv("Q_SECRET_KEY")
	defer os.Setenv("Q_SECRET_KEY", orig)
	os.Setenv("Q_SECRET_KEY", testMasterKey)

	tmpDir := t.TempDir()
	oldPubKey := PublicKeyPath
	PublicKeyPath = tmpDir + "/public.key"
	defer func() { PublicKeyPath = oldPubKey }()
	WritePublicKey(testPublicKey)

	db, _ := OpenDB(tmpDir + "/test.db")
	defer db.Close()

	// Agregar un secret
	encrypted, _ := Encrypt([]byte("injected-value"), testPublicKey)
	db.UpsertSecret("testproj", "TEST_VAR", encrypted)

	// Ejecutar inject
	err := db.InjectEnv("testproj", "printenv", []string{"TEST_VAR"})
	if err != nil {
		t.Fatalf("InjectEnv() error: %v", err)
	}
}
