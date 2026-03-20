// Package internal — crypto.go
//
// Provides all cryptographic operations for vault-t2:
//   - SealSeed / UnsealSeed: protect the 32-byte tier-2 seed with the hardware
//     fingerprint via AES-256-GCM
//   - EncryptSecret / DecryptSecret: per-secret AES-256-GCM encryption using
//     the unsealed seed as the key, with a 4-byte plaintext-length prefix
//   - AuditLog: append-only JSON audit trail
//
// Wire formats
// ─────────────
// Sealed seed on disk (.sealed_seed):
//
//	[ 12-byte nonce ][ ciphertext + 16-byte GCM tag ]   (total: 12 + 32 + 16 = 60 bytes)
//
// Secret file on disk (<name>.enc):
//
//	[ 4-byte big-endian plaintext length ][ 12-byte nonce ][ ciphertext + 16-byte GCM tag ]
//
// The 4-byte length prefix is intentionally written so the future FUSE layer
// (Phase 2) can answer Getattr(size) without decrypting the full payload.
//
// All nonces are generated with crypto/rand and must never be reused.
package internal

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// ─── shared AES-GCM helper ────────────────────────────────────────────────────

// newGCMCipher constructs an AES-256-GCM AEAD from a 32-byte key.
// All four crypto operations (SealSeed, UnsealSeed, EncryptSecret,
// DecryptSecret) share this initialization path.
func newGCMCipher(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be exactly 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM wrapper: %w", err)
	}
	return gcm, nil
}

// ─── Seal / Unseal ────────────────────────────────────────────────────────────

// SealSeed encrypts a 32-byte tier-2 seed with the hardware fingerprint key.
// Returns the on-disk blob: [ 12-byte nonce || ciphertext+tag ].
func SealSeed(seed []byte, fingerprint []byte) ([]byte, error) {
	if len(seed) != 32 {
		return nil, fmt.Errorf("seed must be exactly 32 bytes, got %d", len(seed))
	}
	gcm, err := newGCMCipher(fingerprint)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, seed, nil) // nonce is prepended
	return ciphertext, nil
}

// UnsealSeed decrypts the sealed seed blob produced by SealSeed.
// The blob must be [ 12-byte nonce || ciphertext+tag ].
func UnsealSeed(sealedSeed []byte, fingerprint []byte) ([]byte, error) {
	gcm, err := newGCMCipher(fingerprint)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize() // 12
	if len(sealedSeed) < nonceSize {
		return nil, fmt.Errorf("sealed seed too short: %d bytes", len(sealedSeed))
	}

	nonce := sealedSeed[:nonceSize]
	ciphertextAndTag := sealedSeed[nonceSize:]

	seed, err := gcm.Open(nil, nonce, ciphertextAndTag, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypting sealed seed (wrong hardware?): %w", err)
	}
	return seed, nil
}

// ─── Secret Encrypt / Decrypt ─────────────────────────────────────────────────

// EncryptSecret encrypts plaintext with key (the unsealed tier-2 seed) and
// returns the .enc file payload:
//
//	[ 4-byte big-endian plaintext length ][ 12-byte nonce ][ ciphertext + 16-byte GCM tag ]
func EncryptSecret(plaintext []byte, key []byte) ([]byte, error) {
	gcm, err := newGCMCipher(key)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	// Encode plaintext length as 4-byte big-endian prefix.
	// This lets the FUSE layer answer Getattr without full decryption.
	lengthPrefix := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthPrefix, uint32(len(plaintext)))

	// Seal appends ciphertext+tag after nonce.
	encrypted := gcm.Seal(nonce, nonce, plaintext, nil)

	// Final layout: [ length(4) ][ nonce(12) ][ ciphertext+tag ]
	result := make([]byte, 0, 4+len(encrypted))
	result = append(result, lengthPrefix...)
	result = append(result, encrypted...)
	return result, nil
}

// DecryptSecret decrypts a .enc file payload produced by EncryptSecret.
// It reads and strips the 4-byte length prefix, then decrypts the remainder.
func DecryptSecret(payload []byte, key []byte) ([]byte, error) {
	// Minimum: 4 (length) + 12 (nonce) + 16 (GCM tag) = 32 bytes
	if len(payload) < 32 {
		return nil, fmt.Errorf("payload too short: %d bytes", len(payload))
	}

	// Strip and validate the plaintext length prefix.
	plaintextLen := binary.BigEndian.Uint32(payload[:4])
	body := payload[4:] // [ 12-byte nonce ][ ciphertext + tag ]

	gcm, err := newGCMCipher(key)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize() // 12
	if len(body) < nonceSize {
		return nil, fmt.Errorf("payload body too short for nonce: %d bytes", len(body))
	}

	nonce := body[:nonceSize]
	ciphertextAndTag := body[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertextAndTag, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypting secret: %w", err)
	}

	// Sanity-check: decoded plaintext length must match the prefix.
	if uint32(len(plaintext)) != plaintextLen {
		return nil, fmt.Errorf(
			"plaintext length mismatch: prefix says %d, decrypted %d bytes",
			plaintextLen, len(plaintext),
		)
	}

	return plaintext, nil
}

// ─── Audit Log ────────────────────────────────────────────────────────────────

// auditEntry is a single structured audit log record.
// It intentionally does NOT include secret values — only metadata.
type auditEntry struct {
	Timestamp string `json:"timestamp"`
	Action    string `json:"action"`
	Secret    string `json:"secret"`
	CallerPID int    `json:"caller_pid"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// AuditLog appends one JSON line to <dataDir>/.audit.log.
// Failures to write the audit log are printed to stderr but do not abort the
// caller — the operation has already completed and we must not lose the secret.
// Security-relevant fields (action, success/fail, caller PID) are always logged.
func AuditLog(dataDir, action, secret string, callerPID int, success bool, errMsg string) {
	entry := auditEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Action:    action,
		Secret:    secret,
		CallerPID: callerPID,
		Success:   success,
		Error:     errMsg,
	}

	line, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vault-t2: audit marshal error: %v\n", err)
		return
	}
	line = append(line, '\n')

	logPath := filepath.Join(dataDir, ".audit.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vault-t2: audit log open error: %v\n", err)
		return
	}
	defer f.Close()

	if _, err := f.Write(line); err != nil {
		fmt.Fprintf(os.Stderr, "vault-t2: audit log write error: %v\n", err)
	}
}
