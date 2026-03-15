package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

type KeyResponse struct {
	SessionKey string    `json:"session_key"`
	ExpiresAt  time.Time `json:"expires_at"`
	Window     int64     `json:"window"`
}

type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// Fixed challenge for HMAC-SHA1 key derivation ("pi5-vault" in hex).
// Both YubiKeys must be programmed with the same HMAC-SHA1 secret so that
// either key produces the same output for this challenge.
const vaultChallenge = "7069352d7661756c74"

func deriveSessionKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("🔐 Deriving session key from YubiKey (plug in YubiKey if not already)...")

	// Compute HMAC-SHA1 challenge-response from YubiKey slot 2
	cmd := exec.Command("ykman", "otp", "calculate", "2", vaultChallenge)
	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := fmt.Sprintf("YubiKey error: %v - %s", err, output)
		log.Printf("❌ %s\n", errMsg)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: errMsg})
		return
	}

	hmacHex := strings.TrimSpace(string(output))

	// SHA256 the HMAC output to produce a 32-byte AES-256 key
	hash := sha256.Sum256([]byte(hmacHex))
	sessionKey := base64.StdEncoding.EncodeToString(hash[:])

	log.Println("✓ Session key derived")

	response := KeyResponse{
		SessionKey: sessionKey,
		ExpiresAt:  time.Now().Add(30 * time.Minute),
		Window:     0,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func health(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "running",
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	log.Println("Health check")
}

func main() {
	http.HandleFunc("/derive-key", deriveSessionKey)
	http.HandleFunc("/health", health)

	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("  Pi5 Vault Authentication Proxy")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("✓ Running on http://localhost:3000")
	fmt.Println("  YubiKey ready for authentication")
	fmt.Println("  Press Ctrl+C to stop")
	fmt.Println()

	// Check if ykman is available
	cmd := exec.Command("ykman", "--version")
	if err := cmd.Run(); err != nil {
		fmt.Println("⚠ Warning: ykman command not found")
		fmt.Println("  Install YubiKey Manager from: https://www.yubico.com/support/download/yubikey-manager/")
		fmt.Println()
	}

	log.Fatal(http.ListenAndServe(":3000", nil))
}
