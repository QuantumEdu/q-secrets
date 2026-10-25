package internal

import (
	"bytes"
	"filippo.io/age/armor"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testMasterKey es una key generada para testing.
// Es segura compartirla en un test — se usó solo para validar round-trip.
const testMasterKey = `AGE-SECRET-KEY-1LTVQHE83J75LA6G4TDDVQ4U87ENDSG5NN24M6AQQCXQ0R5YSE0AQ3XHPEH`
const testPublicKey = `age1lmy7x4ecl32gceseduy9j0c977uxnay2yu97vsjav0k5vpa9t4dq6k9zm7`

func TestGenerateAgeKey(t *testing.T) {

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

	got, err := DerivePublicKey(testMasterKey)
	if err != nil {
		t.Fatalf("DerivePublicKey() error: %v", err)
	}
	if got != testPublicKey {
		t.Errorf("expected %q, got %q", testPublicKey, got)
	}
}

func TestEncryptDecrypt(t *testing.T) {

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

	encrypted, _ := Encrypt([]byte("test"), testPublicKey)

	_, err := Decrypt(encrypted, "AGE-SECRET-KEY-INVALIDKEYTHATDOESNOTWORK1234567890")
	if err == nil {
		t.Error("expected error when decrypting with wrong key, got nil")
	}
}

func TestValidateKeyPair(t *testing.T) {

	if !ValidateKeyPair(testMasterKey, testPublicKey) {
		t.Error("ValidateKeyPair should return true for valid pair")
	}

	if ValidateKeyPair(testMasterKey, "age1invalidkey0000000000000000000000000000000000000000") {
		t.Error("ValidateKeyPair should return false for invalid public key")
	}
}

func TestEncryptEmpty(t *testing.T) {

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

func TestCryptoWithoutTempFilesOrExecutables(t *testing.T) {
	// A non-existent temp directory makes any accidental temporary-file use fail.
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("TMP", os.Getenv("TMPDIR"))
	t.Setenv("TEMP", os.Getenv("TMPDIR"))
	t.Setenv("PATH", "")
	private, public, err := GenerateAgeKey()
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := Encrypt([]byte("memory-only"), public)
	if err != nil {
		t.Fatal(err)
	}
	var armored bytes.Buffer
	writer := armor.NewWriter(&armored)
	if _, err := writer.Write(encrypted); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	for _, ciphertext := range [][]byte{encrypted, armored.Bytes()} {
		plaintext, err := Decrypt(ciphertext, private)
		if err != nil || string(plaintext) != "memory-only" {
			t.Fatalf("round trip: %v", err)
		}
	}
}

func TestDeriveRejectsMisleadingComment(t *testing.T) {
	public, err := DerivePublicKey("# public key: untrusted\n" + testMasterKey)
	if err != nil || public != testPublicKey {
		t.Fatalf("derive: %v", err)
	}
	_, err = DerivePublicKey("AGE-SECRET-KEY-SENSITIVE-INVALID")
	if err == nil || strings.Contains(err.Error(), "SENSITIVE") {
		t.Fatal("invalid identity must not be echoed")
	}
}

func TestDecryptWrongIdentityAndTampering(t *testing.T) {
	private, _, err := GenerateAgeKey()
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := Encrypt([]byte("secret"), testPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(encrypted, private); err == nil {
		t.Fatal("wrong identity accepted")
	}
	encrypted[len(encrypted)-1] ^= 1
	if _, err := Decrypt(encrypted, testMasterKey); err == nil {
		t.Fatal("tampering accepted")
	}
}
