package security

import (
	"strings"
	"testing"
)

func TestCodeProtectorEncryptDecryptAndHash(t *testing.T) {
	protector := NewCodeProtector("test-secret-that-is-long-and-random")
	plaintext := "GC2609251234560000ABCDEF1234"

	encrypted, err := protector.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if encrypted == plaintext || !strings.HasPrefix(encrypted, ciphertextPrefix) {
		t.Fatalf("unexpected ciphertext: %q", encrypted)
	}
	decrypted, err := protector.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}

	hash1 := protector.Hash(plaintext)
	hash2 := protector.Hash(strings.ToLower(plaintext))
	if hash1 == "" || hash1 != hash2 {
		t.Fatalf("lookup hash must be deterministic after normalization: %q / %q", hash1, hash2)
	}
	if strings.Contains(hash1, plaintext) {
		t.Fatal("lookup hash must not contain plaintext")
	}
}

func TestCodeProtectorRejectsWrongSecret(t *testing.T) {
	first := NewCodeProtector("first-secret")
	second := NewCodeProtector("second-secret")

	encrypted, err := first.Encrypt("GC-SECRET-001")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if _, err := second.Decrypt(encrypted); err == nil {
		t.Fatal("expected decrypt failure with different secret")
	}
}

func TestCodeProtectorSupportsLegacyPlaintextForMigration(t *testing.T) {
	protector := NewCodeProtector("migration-secret")
	got, err := protector.Decrypt("  gc-legacy-001  ")
	if err != nil {
		t.Fatalf("decrypt legacy plaintext: %v", err)
	}
	if got != "GC-LEGACY-001" {
		t.Fatalf("expected normalized legacy plaintext, got %q", got)
	}
}

func TestMaskCode(t *testing.T) {
	code := "GC2609251234560000ABCDEF1234"
	masked := MaskCode(code)
	if masked == code {
		t.Fatal("masked code must differ from plaintext")
	}
	if !strings.HasPrefix(masked, "GC2609") || !strings.HasSuffix(masked, "1234") {
		t.Fatalf("unexpected mask: %q", masked)
	}
	if strings.Contains(masked, "ABCDEF") {
		t.Fatalf("mask exposed middle of code: %q", masked)
	}
}
