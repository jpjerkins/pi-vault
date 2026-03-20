// vault-t2 — Tier 2 secret management CLI for pi5.local
//
// Dispatches on os.Args[0] (when invoked via symlink) or os.Args[1] (subcommand):
//
//	t2-provision [--generate]   Seal a new or recovered tier-2 seed
//	t2-get    <name>            Decrypt and print a secret
//	t2-set    <name> [value]    Encrypt and store a secret
//	t2-list                     List all secret names
//	t2-delete <name>            Delete a secret
package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/philj/vault-t2/internal"
)

const (
	defaultDataDir   = "/mnt/data/vault-t2"
	defaultConfigDir = "/etc/vault-t2"
	sealedSeedFile   = ".sealed_seed"
)

// binaryName is the name of the running binary, computed once at startup.
// Used to distinguish symlink invocation (t2-get etc.) from direct invocation
// (vault-t2 get). Avoids repeating filepath.Base(os.Args[0]) across functions.
var binaryName = filepath.Base(os.Args[0])

func main() {
	cmd := commandName()
	switch cmd {
	case "t2-provision", "provision":
		runProvision()
	case "t2-get", "get":
		runGet()
	case "t2-set", "set":
		runSet()
	case "t2-list", "list":
		runList()
	case "t2-delete", "delete":
		runDelete()
	default:
		fmt.Fprintf(os.Stderr, "vault-t2: unknown command %q\n", cmd)
		fmt.Fprintf(os.Stderr, "Usage: vault-t2 {provision|get|set|list|delete}\n")
		fmt.Fprintf(os.Stderr, "       or invoke via symlink: t2-get, t2-set, t2-list, t2-delete, t2-provision\n")
		os.Exit(1)
	}
}

// commandName returns the effective subcommand, either from the binary name
// (symlink dispatch) or from os.Args[1] (direct invocation as "vault-t2 <cmd>").
func commandName() string {
	if binaryName != "vault-t2" {
		return binaryName // invoked as t2-get, t2-set, etc.
	}
	if len(os.Args) < 2 {
		return ""
	}
	// Strip leading "t2-" prefix if present so both "vault-t2 get" and
	// "vault-t2 t2-get" work.
	return strings.TrimPrefix(os.Args[1], "t2-")
}

// ─── provision ────────────────────────────────────────────────────────────────

func runProvision() {
	generate := false
	for _, arg := range os.Args[1:] {
		if arg == "--generate" {
			generate = true
		}
	}

	dataDir := defaultDataDir

	// Create data dir if it doesn't exist.
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		fatal("creating data dir: %v", err)
	}
	checkDataDirPerms(dataDir)

	// Create config dir if it doesn't exist.
	if err := os.MkdirAll(defaultConfigDir, 0755); err != nil {
		// Non-fatal — may require sudo; warn and continue.
		fmt.Fprintf(os.Stderr, "vault-t2: warning: could not create config dir %s: %v\n", defaultConfigDir, err)
	}

	var seed []byte

	if generate {
		// Generate a fresh cryptographically-random 32-byte seed.
		seed = make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, seed); err != nil {
			fatal("generating seed: %v", err)
		}
		encoded := base64.StdEncoding.EncodeToString(seed)
		fmt.Println("╔══════════════════════════════════════════════════════════════╗")
		fmt.Println("║              SAVE THIS SEED IN TIER 1 (vault-set)           ║")
		fmt.Println("╠══════════════════════════════════════════════════════════════╣")
		fmt.Printf( "║  %s  ║\n", encoded)
		fmt.Println("╚══════════════════════════════════════════════════════════════╝")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Run: vault-set tier2_seed <the base64 above>")
		fmt.Fprintln(os.Stderr, "")
	} else {
		// Read seed from stdin (recovery path — base64-encoded).
		fmt.Fprint(os.Stderr, "Enter base64-encoded seed (from Tier 1): ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			fatal("no seed provided")
		}
		var err error
		seed, err = base64.StdEncoding.DecodeString(line)
		if err != nil {
			fatal("decoding base64 seed: %v", err)
		}
		if len(seed) != 32 {
			fatal("seed must be 32 bytes, got %d (wrong base64?)", len(seed))
		}
	}

	fingerprint, err := internal.ReadHardwareFingerprint()
	if err != nil {
		fatal("reading hardware fingerprint: %v", err)
	}

	sealed, err := internal.SealSeed(seed, fingerprint)
	if err != nil {
		fatal("sealing seed: %v", err)
	}

	sealedPath := filepath.Join(dataDir, sealedSeedFile)
	if err := os.WriteFile(sealedPath, sealed, 0600); err != nil {
		fatal("writing sealed seed: %v", err)
	}

	internal.AuditLog(dataDir, "provision", "", os.Getpid(), true, "")
	fmt.Fprintf(os.Stderr, "✓ Tier 2 provisioned. Sealed seed written to %s\n", sealedPath)
}

// ─── get ──────────────────────────────────────────────────────────────────────

func runGet() {
	args := cliArgs(1)
	name := args[0]

	dataDir := defaultDataDir
	seed := unsealOrDie(dataDir)

	encPath := filepath.Join(dataDir, name+".enc")
	payload, err := os.ReadFile(encPath)
	if err != nil {
		internal.AuditLog(dataDir, "get", name, os.Getpid(), false, err.Error())
		fatal("reading secret %q: %v", name, err)
	}

	plaintext, err := internal.DecryptSecret(payload, seed)
	if err != nil {
		internal.AuditLog(dataDir, "get", name, os.Getpid(), false, err.Error())
		fatal("decrypting secret %q: %v", name, err)
	}

	internal.AuditLog(dataDir, "get", name, os.Getpid(), true, "")
	os.Stdout.Write(plaintext)
	// Ensure trailing newline for shell compatibility when piping.
	if len(plaintext) == 0 || plaintext[len(plaintext)-1] != '\n' {
		fmt.Println()
	}
}

// ─── set ──────────────────────────────────────────────────────────────────────

func runSet() {
	// Accept: t2-set <name> [value]
	// If value is omitted, read from stdin.
	var name string
	var plaintext []byte

	// Strip the subcommand arg when invoked as "vault-t2 set <name>".
	rawArgs := os.Args[1:]
	if binaryName == "vault-t2" {
		rawArgs = rawArgs[1:] // skip "set" / "t2-set"
	}

	if len(rawArgs) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: t2-set <name> [value]\n")
		os.Exit(1)
	}
	name = rawArgs[0]

	if len(rawArgs) >= 2 {
		plaintext = []byte(strings.Join(rawArgs[1:], " "))
	} else {
		var err error
		plaintext, err = io.ReadAll(os.Stdin)
		if err != nil {
			fatal("reading secret value from stdin: %v", err)
		}
	}
	// Strip trailing newline that shells commonly append.
	plaintext = []byte(strings.TrimRight(string(plaintext), "\n"))

	if len(plaintext) == 0 {
		fatal("secret value is empty")
	}

	dataDir := defaultDataDir
	seed := unsealOrDie(dataDir)

	ciphertext, err := internal.EncryptSecret(plaintext, seed)
	if err != nil {
		internal.AuditLog(dataDir, "set", name, os.Getpid(), false, err.Error())
		fatal("encrypting secret %q: %v", name, err)
	}

	encPath := filepath.Join(dataDir, name+".enc")
	if err := os.WriteFile(encPath, ciphertext, 0600); err != nil {
		internal.AuditLog(dataDir, "set", name, os.Getpid(), false, err.Error())
		fatal("writing secret %q: %v", name, err)
	}

	internal.AuditLog(dataDir, "set", name, os.Getpid(), true, "")
	fmt.Fprintf(os.Stderr, "✓ Secret %q stored.\n", name)
}

// ─── list ─────────────────────────────────────────────────────────────────────

func runList() {
	dataDir := defaultDataDir
	checkDataDirPerms(dataDir)

	entries, err := os.ReadDir(dataDir)
	if err != nil {
		fatal("reading data dir: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".enc") {
			fmt.Println(strings.TrimSuffix(name, ".enc"))
			found = true
		}
	}
	if !found {
		fmt.Fprintln(os.Stderr, "(no secrets stored)")
	}
}

// ─── delete ───────────────────────────────────────────────────────────────────

func runDelete() {
	args := cliArgs(1)
	name := args[0]

	dataDir := defaultDataDir
	encPath := filepath.Join(dataDir, name+".enc")

	if err := os.Remove(encPath); err != nil {
		internal.AuditLog(dataDir, "delete", name, os.Getpid(), false, err.Error())
		fatal("deleting secret %q: %v", name, err)
	}

	internal.AuditLog(dataDir, "delete", name, os.Getpid(), true, "")
	fmt.Fprintf(os.Stderr, "✓ Secret %q deleted.\n", name)
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// unsealOrDie reads the hardware fingerprint and decrypts the sealed seed.
// Exits with a clear error message on failure.
func unsealOrDie(dataDir string) []byte {
	checkDataDirPerms(dataDir)

	sealedPath := filepath.Join(dataDir, sealedSeedFile)
	sealed, err := os.ReadFile(sealedPath)
	if err != nil {
		fatal("reading sealed seed (has t2-provision been run?): %v", err)
	}

	fingerprint, err := internal.ReadHardwareFingerprint()
	if err != nil {
		fatal("reading hardware fingerprint: %v", err)
	}

	seed, err := internal.UnsealSeed(sealed, fingerprint)
	if err != nil {
		fatal("unsealing seed: %v", err)
	}
	return seed
}

// checkDataDirPerms warns if the data directory has loose permissions.
func checkDataDirPerms(dataDir string) {
	info, err := os.Stat(dataDir)
	if err != nil {
		return // doesn't exist yet, provision will create it
	}
	mode := info.Mode().Perm()
	if mode&0077 != 0 {
		fmt.Fprintf(os.Stderr,
			"vault-t2: WARNING: data dir %s has loose permissions %04o (expected 0700)\n",
			dataDir, mode)
	}
}

// cliArgs returns the positional arguments after the command name,
// exiting with a usage message if fewer than min are present.
func cliArgs(min int) []string {
	// When invoked as a symlink (t2-get), all of os.Args[1:] are positional.
	// When invoked as "vault-t2 get <args>", os.Args[1] is the subcommand.
	args := os.Args[1:]
	if binaryName == "vault-t2" && len(args) > 0 {
		args = args[1:] // drop the subcommand word
	}
	if len(args) < min {
		fmt.Fprintf(os.Stderr, "vault-t2: not enough arguments (expected %d)\n", min)
		os.Exit(1)
	}
	return args
}

// fatal prints an error message and exits with code 1.
func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "vault-t2: error: "+format+"\n", args...)
	os.Exit(1)
}
