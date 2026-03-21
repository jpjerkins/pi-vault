package internal

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

func key32(b byte) []byte { return bytes.Repeat([]byte{b}, 32) }

// ─── newGCMCipher ─────────────────────────────────────────────────────────────

func TestNewGCMCipherRejectsShortKey(t *testing.T) {
	_, err := newGCMCipher(make([]byte, 16))
	if err == nil {
		t.Fatal("expected error for 16-byte key, got nil")
	}
}

func TestNewGCMCipherRejectsEmptyKey(t *testing.T) {
	_, err := newGCMCipher(nil)
	if err == nil {
		t.Fatal("expected error for nil key, got nil")
	}
}

// ─── SealSeed / UnsealSeed ────────────────────────────────────────────────────

func TestSealUnsealRoundTrip(t *testing.T) {
	seed := key32(0xAA)
	fp := key32(0xBB)

	sealed, err := SealSeed(seed, fp)
	if err != nil {
		t.Fatalf("SealSeed: %v", err)
	}

	got, err := UnsealSeed(sealed, fp)
	if err != nil {
		t.Fatalf("UnsealSeed: %v", err)
	}
	if !bytes.Equal(got, seed) {
		t.Fatalf("unsealed seed mismatch: got %x, want %x", got, seed)
	}
}

func TestSealProducesDistinctCiphertexts(t *testing.T) {
	// Each call uses a fresh random nonce — same inputs must not produce
	// identical ciphertext (nonce reuse would be a serious security flaw).
	seed := key32(0x01)
	fp := key32(0x02)

	a, _ := SealSeed(seed, fp)
	b, _ := SealSeed(seed, fp)
	if bytes.Equal(a, b) {
		t.Fatal("SealSeed produced identical ciphertexts for two calls — nonce reuse")
	}
}

func TestUnsealWrongKey(t *testing.T) {
	seed := key32(0xAA)
	sealed, _ := SealSeed(seed, key32(0xBB))

	_, err := UnsealSeed(sealed, key32(0xCC)) // wrong fingerprint
	if err == nil {
		t.Fatal("expected error when unsealing with wrong key, got nil")
	}
}

func TestUnsealShortBlob(t *testing.T) {
	_, err := UnsealSeed([]byte("short"), key32(0x01))
	if err == nil {
		t.Fatal("expected error for short blob, got nil")
	}
}

func TestSealRejectsWrongSeedSize(t *testing.T) {
	_, err := SealSeed(make([]byte, 16), key32(0x01))
	if err == nil {
		t.Fatal("expected error for 16-byte seed, got nil")
	}
}

func TestUnsealTamperedCiphertext(t *testing.T) {
	seed := key32(0xAA)
	fp := key32(0xBB)
	sealed, _ := SealSeed(seed, fp)

	// Flip a byte in the ciphertext (after the 12-byte nonce).
	sealed[20] ^= 0xFF

	_, err := UnsealSeed(sealed, fp)
	if err == nil {
		t.Fatal("expected error after tampering ciphertext, got nil")
	}
}

// ─── EncryptSecret / DecryptSecret ───────────────────────────────────────────

func TestEncryptDecryptRoundTrip(t *testing.T) {
	cases := []string{
		"",                  // empty plaintext
		"hello",             // short
		"s3cr3t-p@ssw0rd!",  // typical password
		string(make([]byte, 1024)), // 1 KB
	}

	key := key32(0x42)
	for _, tc := range cases {
		payload, err := EncryptSecret([]byte(tc), key)
		if err != nil {
			t.Fatalf("EncryptSecret(%q): %v", tc, err)
		}
		got, err := DecryptSecret(payload, key)
		if err != nil {
			t.Fatalf("DecryptSecret(%q): %v", tc, err)
		}
		if string(got) != tc {
			t.Fatalf("plaintext mismatch: got %q, want %q", got, tc)
		}
	}
}

func TestEncryptProducesDistinctCiphertexts(t *testing.T) {
	key := key32(0x42)
	a, _ := EncryptSecret([]byte("secret"), key)
	b, _ := EncryptSecret([]byte("secret"), key)
	if bytes.Equal(a, b) {
		t.Fatal("EncryptSecret produced identical ciphertexts — nonce reuse")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	payload, _ := EncryptSecret([]byte("secret"), key32(0x42))

	_, err := DecryptSecret(payload, key32(0x99))
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key, got nil")
	}
}

func TestDecryptShortPayload(t *testing.T) {
	_, err := DecryptSecret(make([]byte, 10), key32(0x01))
	if err == nil {
		t.Fatal("expected error for payload shorter than minimum, got nil")
	}
}

func TestDecryptTamperedPayload(t *testing.T) {
	key := key32(0x42)
	payload, _ := EncryptSecret([]byte("secret"), key)

	// Flip a byte in the ciphertext region (after 4-byte prefix + 12-byte nonce).
	payload[20] ^= 0xFF

	_, err := DecryptSecret(payload, key)
	if err == nil {
		t.Fatal("expected error after tampering payload, got nil")
	}
}

func TestEncryptRejectsWrongKeySize(t *testing.T) {
	_, err := EncryptSecret([]byte("hello"), make([]byte, 16))
	if err == nil {
		t.Fatal("expected error for 16-byte key, got nil")
	}
}

// ─── AuditLog ─────────────────────────────────────────────────────────────────

func TestAuditLogWritesJSONL(t *testing.T) {
	dir := t.TempDir()
	AuditLog(dir, "test-action", "my_secret", 1234, true, "")
	AuditLog(dir, "test-action", "my_secret", 1234, false, "some error")

	data, err := os.ReadFile(filepath.Join(dir, ".audit.log"))
	if err != nil {
		t.Fatalf("reading audit log: %v", err)
	}

	lines := bytes.Split(bytes.TrimRight(data, "\n"), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines, got %d", len(lines))
	}
	for i, line := range lines {
		if !bytes.Contains(line, []byte("test-action")) {
			t.Errorf("line %d missing action field: %s", i, line)
		}
		if !bytes.Contains(line, []byte("my_secret")) {
			t.Errorf("line %d missing secret field: %s", i, line)
		}
	}
}

func TestAuditLogMissingDirIsNonFatal(t *testing.T) {
	// Should not panic or return an error — just prints to stderr.
	AuditLog("/nonexistent/dir/that/cannot/exist", "action", "secret", 0, true, "")
}
