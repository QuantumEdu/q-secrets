package internal

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"
)

// GenerateAgeKey generates an X25519 identity entirely in memory.
func GenerateAgeKey() (privateKey, publicKey string, err error) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return "", "", fmt.Errorf("generating age identity: %w", err)
	}
	return identity.String(), identity.Recipient().String(), nil
}

func parseIdentity(privateKey string) (*age.X25519Identity, error) {
	identities, err := age.ParseIdentities(strings.NewReader(privateKey))
	if err != nil {
		return nil, fmt.Errorf("invalid age identity")
	}
	if len(identities) != 1 {
		return nil, fmt.Errorf("expected one X25519 age identity")
	}
	identity, ok := identities[0].(*age.X25519Identity)
	if !ok {
		return nil, fmt.Errorf("expected an X25519 age identity")
	}
	return identity, nil
}

// DerivePublicKey derives from the identity, never trusting an optional comment.
func DerivePublicKey(privateKey string) (string, error) {
	identity, err := parseIdentity(privateKey)
	if err != nil {
		return "", err
	}
	return identity.Recipient().String(), nil
}

// Encrypt uses the age wire format without subprocesses or temporary files.
func Encrypt(plaintext []byte, publicKey string) ([]byte, error) {
	recipient, err := age.ParseX25519Recipient(strings.TrimSpace(publicKey))
	if err != nil {
		return nil, fmt.Errorf("invalid age recipient")
	}
	var out bytes.Buffer
	writer, err := age.Encrypt(&out, recipient)
	if err != nil {
		return nil, fmt.Errorf("age encrypt: %w", err)
	}
	if _, err := writer.Write(plaintext); err != nil {
		return nil, fmt.Errorf("age write: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("age close: %w", err)
	}
	return out.Bytes(), nil
}

// Decrypt accepts existing age/rage binary or armored ciphertext. Keys and
// plaintext stay in process memory; Go does not guarantee secure memory erasure.
func Decrypt(ciphertext []byte, privateKey string) ([]byte, error) {
	identity, err := parseIdentity(privateKey)
	if err != nil {
		return nil, err
	}
	var input io.Reader = bytes.NewReader(ciphertext)
	if bytes.HasPrefix(ciphertext, []byte(armor.Header)) {
		input = armor.NewReader(input)
	}
	reader, err := age.Decrypt(input, identity)
	if err != nil {
		return nil, fmt.Errorf("age decrypt: %w", err)
	}
	plaintext, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("age read: %w", err)
	}
	return plaintext, nil
}

func ValidateKeyPair(privateKey, publicKey string) bool {
	derived, err := DerivePublicKey(privateKey)
	return err == nil && derived == strings.TrimSpace(publicKey)
}
