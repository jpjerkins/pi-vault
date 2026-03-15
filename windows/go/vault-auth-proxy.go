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

func deriveSessionKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("🔐 Touch YubiKey to derive session key...")

	// Get TOTP from YubiKey
	cmd := exec.Command("ykman", "oath", "accounts", "code", "Pi5 Vault", "--single")
	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := fmt.Sprintf("YubiKey error: %v - %s", err, output)
		log.Printf("❌ %s\n", errMsg)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: errMsg})
		return
	}

	totp := strings.TrimSpace(string(output))

	// Calculate time window (30min buckets = 1800 seconds)
	window := time.Now().UTC().Unix() / 1800

	// Derive session key = SHA256(TOTP + window)
	keyInput := fmt.Sprintf("%s-%d", totp, window)
	hash := sha256.Sum256([]byte(keyInput))
	sessionKey := base64.StdEncoding.EncodeToString(hash[:])

	log.Println("✓ Session key derived (valid for 30min)")

	response := KeyResponse{
		SessionKey: sessionKey,
		ExpiresAt:  time.Now().Add(30 * time.Minute),
		Window:     window,
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
