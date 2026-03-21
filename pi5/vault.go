package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	SecretsDir     = "/mnt/data/secrets"
	SessionKeyFile = "/mnt/data/secrets/.session_key"
	ExpiryFile     = "/mnt/data/secrets/.session_expiry"
	AuditLog       = "/mnt/data/secrets/.audit.log"
	AuthProxyURL   = "http://localhost:3000/derive-key"
)

type SessionKeyResponse struct {
	SessionKey string `json:"session_key"`
	ExpiresAt  string `json:"expires_at"`
	Window     float64 `json:"window"`
}

type AuditEntry struct {
	Timestamp string `json:"timestamp"`
	Action    string `json:"action"`
	Secret    string `json:"secret"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// Get session key (cached or fresh from YubiKey via auth proxy)
func getSessionKey() ([]byte, error) {
	// Check cached session key
	if expiryBytes, err := ioutil.ReadFile(ExpiryFile); err == nil {
		expiryStr := strings.TrimSpace(string(expiryBytes))
		expiry, _ := strconv.ParseInt(expiryStr, 10, 64)

		if time.Now().Unix() < expiry {
			// Cached key still valid
			keyB64, err := ioutil.ReadFile(SessionKeyFile)
			if err == nil {
				return base64.StdEncoding.DecodeString(strings.TrimSpace(string(keyB64)))
			}
		}
	}

	// Need fresh session key from YubiKey via auth proxy
	fmt.Fprintln(os.Stderr, "Requesting session key from laptop YubiKey...")

	resp, err := http.Post(AuthProxyURL, "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("cannot reach auth proxy (is SSH tunnel active?): %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("auth proxy returned status %d: %s", resp.StatusCode, body)
	}

	var skResp SessionKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&skResp); err != nil {
		return nil, fmt.Errorf("invalid response from auth proxy: %v", err)
	}

	// Decode session key
	sessionKey, err := base64.StdEncoding.DecodeString(skResp.SessionKey)
	if err != nil {
		return nil, fmt.Errorf("invalid session key format: %v", err)
	}

	// Cache session key
	if err := ioutil.WriteFile(SessionKeyFile, []byte(skResp.SessionKey), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not cache session key: %v\n", err)
	}

	// Cache expiry
	expiresAt, _ := time.Parse(time.RFC3339, skResp.ExpiresAt)
	expiryBytes := []byte(fmt.Sprint(expiresAt.Unix()))
	if err := ioutil.WriteFile(ExpiryFile, expiryBytes, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not cache expiry: %v\n", err)
	}

	fmt.Fprintln(os.Stderr, "✓ Session key cached (valid for 30 minutes)")

	return sessionKey, nil
}

// Encrypt secret with AES-256-GCM
func encryptSecret(plaintext string, sessionKey []byte) ([]byte, error) {
	block, err := aes.NewCipher(sessionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return ciphertext, nil
}

// Decrypt secret with AES-256-GCM
func decryptSecret(name string, sessionKey []byte) (string, error) {
	secretFile := filepath.Join(SecretsDir, name+".enc")

	ciphertext, err := ioutil.ReadFile(secretFile)
	if err != nil {
		return "", fmt.Errorf("secret not found: %s", name)
	}

	block, err := aes.NewCipher(sessionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("invalid ciphertext")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed (wrong key or corrupted data)")
	}

	return string(plaintext), nil
}

// Audit log
func auditLog(action, secret string, success bool, errMsg string) {
	entry := AuditEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Action:    action,
		Secret:    secret,
		Success:   success,
		Error:     errMsg,
	}

	data, _ := json.Marshal(entry)
	f, err := os.OpenFile(AuditLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(data, '\n'))
}

// Commands

func cmdGet(name string) error {
	sessionKey, err := getSessionKey()
	if err != nil {
		auditLog("get", name, false, err.Error())
		return err
	}

	secret, err := decryptSecret(name, sessionKey)
	if err != nil {
		auditLog("get", name, false, err.Error())
		return err
	}

	auditLog("get", name, true, "")
	fmt.Print(secret)
	return nil
}

func cmdSet(name string, value string) error {
	sessionKey, err := getSessionKey()
	if err != nil {
		auditLog("set", name, false, err.Error())
		return err
	}

	ciphertext, err := encryptSecret(value, sessionKey)
	if err != nil {
		auditLog("set", name, false, err.Error())
		return err
	}

	secretFile := filepath.Join(SecretsDir, name+".enc")
	if err := ioutil.WriteFile(secretFile, ciphertext, 0600); err != nil {
		auditLog("set", name, false, err.Error())
		return err
	}

	auditLog("set", name, true, "")
	fmt.Fprintln(os.Stderr, "✓ Secret encrypted and stored")
	return nil
}

func cmdList() error {
	files, err := ioutil.ReadDir(SecretsDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".enc" {
			name := strings.TrimSuffix(file.Name(), ".enc")
			fmt.Println(name)
		}
	}
	return nil
}

func cmdDelete(name string) error {
	secretFile := filepath.Join(SecretsDir, name+".enc")
	if err := os.Remove(secretFile); err != nil {
		auditLog("delete", name, false, err.Error())
		return err
	}

	auditLog("delete", name, true, "")
	fmt.Fprintln(os.Stderr, "✓ Secret deleted")
	return nil
}

func exposeDir() string {
	return filepath.Join("/run/user", strconv.Itoa(os.Getuid()), "vault-t1")
}

func cleanupSecret(name string) error {
	secretPath := filepath.Join(exposeDir(), name)
	err := os.Remove(secretPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove secret file: %v", err)
	}
	return nil
}

func cmdExpose(name string, duration int) error {
	sessionKey, err := getSessionKey()
	if err != nil {
		auditLog("expose", name, false, err.Error())
		return err
	}

	secret, err := decryptSecret(name, sessionKey)
	if err != nil {
		auditLog("expose", name, false, err.Error())
		return err
	}

	dir := exposeDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("cannot create tmpfs directory: %v", err)
	}

	secretPath := filepath.Join(dir, name)
	if _, err := os.Stat(secretPath); err == nil {
		return fmt.Errorf("secret '%s' is already exposed — run: vault-cleanup %s", name, name)
	}

	if err := os.WriteFile(secretPath, []byte(secret), 0400); err != nil {
		return fmt.Errorf("cannot write secret file: %v", err)
	}

	// Auto-cleanup after duration
	go func() {
		time.Sleep(time.Duration(duration) * time.Minute)
		cleanupSecret(name)
		auditLog("expose-cleanup", name, true, fmt.Sprintf("auto-cleanup after %dm", duration))
	}()

	// Cleanup on signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigChan
		cleanupSecret(name)
		auditLog("expose-cleanup", name, true, "signal-triggered cleanup")
		os.Exit(0)
	}()

	auditLog("expose", name, true, fmt.Sprintf("duration=%dm path=%s", duration, secretPath))
	fmt.Fprintln(os.Stderr, "⚠ Secret exposed — will auto-cleanup in", duration, "minutes")
	fmt.Println(secretPath)
	return nil
}

func cmdCleanup(name string) error {
	if err := cleanupSecret(name); err != nil {
		auditLog("cleanup", name, false, err.Error())
		return err
	}
	auditLog("cleanup", name, true, "manual cleanup")
	fmt.Fprintln(os.Stderr, "✓ Cleaned up:", name)
	return nil
}

func cmdCleanupAll() error {
	dir := exposeDir()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "No exposed secrets to clean up")
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot read expose directory: %v", err)
	}

	count := 0
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if err := cleanupSecret(e.Name()); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to cleanup %s: %v\n", e.Name(), err)
		} else {
			count++
		}
	}
	auditLog("cleanup-all", "", true, fmt.Sprintf("cleaned %d secrets", count))
	fmt.Fprintf(os.Stderr, "✓ Cleaned up %d exposed secrets\n", count)
	return nil
}

func main() {
	command := filepath.Base(os.Args[0])

	// Support multiple symlinks: vault-get, vault-set, vault-list, etc.
	switch command {
	case "vault-get":
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: vault-get <name>")
			os.Exit(1)
		}
		if err := cmdGet(os.Args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "vault-set":
		if len(os.Args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: vault-set <name> [value]")
			os.Exit(1)
		}
		name := os.Args[1]
		var value string
		if len(os.Args) >= 3 {
			value = os.Args[2]
		} else {
			// Read from stdin
			data, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
				os.Exit(1)
			}
			value = strings.TrimSpace(string(data))
		}
		if err := cmdSet(name, value); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "vault-list":
		if err := cmdList(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "vault-delete":
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: vault-delete <name>")
			os.Exit(1)
		}
		if err := cmdDelete(os.Args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "vault-expose":
		if len(os.Args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: vault-expose <name> [--duration MINUTES]")
			fmt.Fprintln(os.Stderr, "  Exposes a secret as a temporary file in tmpfs (default: 5 minutes)")
			fmt.Fprintln(os.Stderr, "  Prints the file path to stdout.")
			os.Exit(1)
		}
		name := os.Args[1]
		duration := 5
		for i := 2; i < len(os.Args)-1; i++ {
			if os.Args[i] == "--duration" {
				d, err := strconv.Atoi(os.Args[i+1])
				if err != nil || d < 1 || d > 60 {
					fmt.Fprintln(os.Stderr, "Error: --duration must be 1–60 minutes")
					os.Exit(1)
				}
				duration = d
			}
		}
		if err := cmdExpose(name, duration); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		// Block until auto-cleanup goroutine fires (keeps signal handler alive)
		select {}

	case "vault-cleanup":
		if len(os.Args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage:")
			fmt.Fprintln(os.Stderr, "  vault-cleanup <name>    Remove specific exposed secret")
			fmt.Fprintln(os.Stderr, "  vault-cleanup --all     Remove all exposed secrets")
			os.Exit(1)
		}
		if os.Args[1] == "--all" {
			if err := cmdCleanupAll(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			if err := cmdCleanup(os.Args[1]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}

	default:
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  vault-get <name>              Get secret")
		fmt.Fprintln(os.Stderr, "  vault-set <name> [value]      Set secret (from arg or stdin)")
		fmt.Fprintln(os.Stderr, "  vault-list                    List all secrets")
		fmt.Fprintln(os.Stderr, "  vault-delete <name>           Delete secret")
		fmt.Fprintln(os.Stderr, "  vault-expose <name>           Expose secret as tmpfs file")
		fmt.Fprintln(os.Stderr, "  vault-cleanup <name>|--all    Remove exposed secret(s)")
		os.Exit(1)
	}
}
