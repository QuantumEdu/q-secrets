package internal

import (
	"os"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestGetMasterKey_EnvVar(t *testing.T) {
	orig := os.Getenv("Q_SECRET_KEY")
	defer os.Setenv("Q_SECRET_KEY", orig)

	os.Setenv("Q_SECRET_KEY", "AGE-SECRET-KEY-1-test")
	key, err := GetMasterKey()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if key != "AGE-SECRET-KEY-1-test" {
		t.Fatalf("expected 'AGE-SECRET-KEY-1-test', got: %q", key)
	}

	// Env var should take priority over anything in keychain
	t.Run("env var has priority", func(t *testing.T) {
		keyring.MockInit()
		keyring.Set(serviceName, keychainKey, "from-keychain")

		key, err := GetMasterKey()
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if key != "AGE-SECRET-KEY-1-test" {
			t.Fatalf("env var should override keychain, got: %q", key)
		}
	})
}

func TestGetMasterKey_NoEnvVar_NoKeychain(t *testing.T) {
	orig := os.Getenv("Q_SECRET_KEY")
	os.Unsetenv("Q_SECRET_KEY")
	defer os.Setenv("Q_SECRET_KEY", orig)

	keyring.MockInit()

	_, err := GetMasterKey()
	if err == nil {
		t.Fatal("expected error when no key is set, got nil")
	}
}

func TestSaveMasterKey_Empty(t *testing.T) {
	err := SaveMasterKey("")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestSaveAndGetMasterKey_RoundTrip(t *testing.T) {
	orig := os.Getenv("Q_SECRET_KEY")
	os.Unsetenv("Q_SECRET_KEY")
	defer os.Setenv("Q_SECRET_KEY", orig)

	keyring.MockInit()

	testKey := "AGE-SECRET-KEY-1-roundtrip-test-value"

	err := SaveMasterKey(testKey)
	if err != nil {
		t.Fatalf("SaveMasterKey failed: %v", err)
	}

	got, err := GetMasterKey()
	if err != nil {
		t.Fatalf("GetMasterKey failed: %v", err)
	}
	if got != testKey {
		t.Fatalf("round-trip failed: saved %q, got %q", testKey, got)
	}
}

func TestMasterKeyExists(t *testing.T) {
	orig := os.Getenv("Q_SECRET_KEY")
	os.Unsetenv("Q_SECRET_KEY")
	defer os.Setenv("Q_SECRET_KEY", orig)

	keyring.MockInit()

	if MasterKeyExists() {
		t.Fatal("expected MasterKeyExists to be false when nothing is set")
	}

	if err := SaveMasterKey("AGE-SECRET-KEY-1-exists-test"); err != nil {
		t.Fatalf("SaveMasterKey failed: %v", err)
	}

	if !MasterKeyExists() {
		t.Fatal("expected MasterKeyExists to be true after saving to mock keychain")
	}
}

func TestMasterKeyExists_EnvVar(t *testing.T) {
	orig := os.Getenv("Q_SECRET_KEY")
	defer os.Setenv("Q_SECRET_KEY", orig)

	os.Setenv("Q_SECRET_KEY", "AGE-SECRET-KEY-1-exists-env")
	if !MasterKeyExists() {
		t.Fatal("expected MasterKeyExists to be true when Q_SECRET_KEY is set")
	}
}
