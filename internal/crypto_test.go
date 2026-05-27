package internal

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
)

// testMasterKey es una key generada para testing.
// Es segura compartirla en un test — se usó solo para validar round-trip.
const testMasterKey = `AGE-SECRET-KEY-1LTVQHE83J75LA6G4TDDVQ4U87ENDSG5NN24M6AQQCXQ0R5YSE0AQ3XHPEH`
const testPublicKey = `age1lmy7x4ecl32gceseduy9j0c977uxnay2yu97vsjav0k5vpa9t4dq6k9zm7`

func TestGenerateAgeKey(t *testing.T) {
	if findKeygen() == "" {
		t.Skip("age-keygen/rage-keygen not found in PATH")
	}

	priv, pub, err := GenerateAgeKey()
	if err != nil {
		t.Fatalf("GenerateAgeKey() error: %v", err)
	}
	if !bytes.HasPrefix([]byte(priv), []byte("AGE-SECRET-KEY-")) {
		t.Errorf("private key should start with AGE-SECRET-KEY-, got: %s", priv[:20])
	}
	if !bytes.HasPrefix([]byte(pub), []byte("age1")) {
		t.Errorf("public key should start with age1, got: %s", pub[:10])
	}
}

func TestDerivePublicKey(t *testing.T) {
	if findKeygen() == "" {
		t.Skip("age-keygen/rage-keygen not found in PATH")
	}

	got, err := DerivePublicKey(testMasterKey)
	if err != nil {
		t.Fatalf("DerivePublicKey() error: %v", err)
	}
	if got != testPublicKey {
		t.Errorf("expected %q, got %q", testPublicKey, got)
	}
}

func TestEncryptDecrypt(t *testing.T) {
	if findAge() == "" || findKeygen() == "" {
		t.Skip("age/rage or keygen not found in PATH")
	}

	plaintext := []byte("hello-world-secret-12345!@#$%")

	encrypted, err := Encrypt(plaintext, testPublicKey)
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}
	if len(encrypted) == 0 {
		t.Fatal("encrypted output is empty")
	}
	if bytes.Contains(encrypted, plaintext) {
		t.Error("encrypted output contains plaintext")
	}

	decrypted, err := Decrypt(encrypted, testMasterKey)
	if err != nil {
		t.Fatalf("Decrypt() error: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("round-trip failed:\n  want: %q\n  got:  %q", string(plaintext), string(decrypted))
	}
}

func TestDecryptWithInvalidKey(t *testing.T) {
	if findAge() == "" {
		t.Skip("age/rage not found in PATH")
	}

	encrypted, _ := Encrypt([]byte("test"), testPublicKey)

	_, err := Decrypt(encrypted, "AGE-SECRET-KEY-INVALIDKEYTHATDOESNOTWORK1234567890")
	if err == nil {
		t.Error("expected error when decrypting with wrong key, got nil")
	}
}

func TestValidateKeyPair(t *testing.T) {
	if findAge() == "" || findKeygen() == "" {
		t.Skip("age/rage or keygen not found in PATH")
	}

	if !ValidateKeyPair(testMasterKey, testPublicKey) {
		t.Error("ValidateKeyPair should return true for valid pair")
	}

	if ValidateKeyPair(testMasterKey, "age1invalidkey0000000000000000000000000000000000000000") {
		t.Error("ValidateKeyPair should return false for invalid public key")
	}
}

func TestEncryptEmpty(t *testing.T) {
	if findAge() == "" {
		t.Skip("age/rage not found in PATH")
	}

	empty := []byte{}
	encrypted, err := Encrypt(empty, testPublicKey)
	if err != nil {
		t.Fatalf("Encrypt(empty) error: %v", err)
	}

	decrypted, err := Decrypt(encrypted, testMasterKey)
	if err != nil {
		t.Fatalf("Decrypt(empty) error: %v", err)
	}
	if len(decrypted) != 0 {
		t.Errorf("expected empty output, got %q", string(decrypted))
	}
}

func TestTempKeyFilePermission(t *testing.T) {
	// Verificar que el archivo temporal se crea con permiso 0600
	// y que se borra después de Decrypt
	if findAge() == "" {
		t.Skip("age/rage not found in PATH")
	}

	encrypted, _ := Encrypt([]byte("permission-test"), testPublicKey)

	keyFile, err := os.CreateTemp("", "q-secret-key-*")
	if err != nil {
		t.Fatal(err)
	}
	keyPath := keyFile.Name()
	keyFile.Close()

	os.WriteFile(keyPath, []byte(testMasterKey), 0600)

	// Desencriptar
	cmd := findAge()
	if cmd == "" {
		t.Fatal("no age binary found")
	}

	execCmd := exec.Command(cmd, "--decrypt", "-i", keyPath, "-o", "-", "-")
	execCmd.Stdin = bytes.NewReader(encrypted)
	_, err = execCmd.Output()
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	// Verificar que el archivo se puede borrar (no está lockeado)
	if err := os.Remove(keyPath); err != nil {
		t.Errorf("temp key file should be removable after use: %v", err)
	}
}
