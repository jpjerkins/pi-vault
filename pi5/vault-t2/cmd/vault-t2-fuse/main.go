// vault-t2-fuse — FUSE daemon for vault-t2 Tier 2 secret storage.
//
// Mounts a read-only virtual filesystem at /run/vault-t2-fs/ (or --mountpoint).
// Each file in the mount corresponds to a secret stored as a .enc file under
// --data-dir. Files are decrypted on demand; plaintext never touches disk.
//
// The daemon unseals the tier-2 seed once at startup using the Pi's hardware
// fingerprint, then holds the seed in process memory for its lifetime.
//
// Access is controlled by the UID ACL at /etc/vault-t2/acl.yaml (root:root 0644).
// The ACL is read once at startup; changes require a daemon restart.
//
// This daemon is intended to be managed by systemd and started before Docker
// services. See vault-t2-fuse.service for the unit file.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"github.com/philj/vault-t2/internal"
	"github.com/philj/vault-t2/vaultfs"
)

const (
	defaultMountpoint = "/run/vault-t2-fs"
	defaultDataDir    = "/mnt/data/vault-t2"
	defaultACLPath    = "/etc/vault-t2/acl.yaml"
)

func main() {
	mountpoint := flag.String("mountpoint", defaultMountpoint, "FUSE mount point")
	dataDir := flag.String("data-dir", defaultDataDir, "Encrypted secrets directory")
	aclPath := flag.String("acl", defaultACLPath, "Path to acl.yaml")
	debug := flag.Bool("debug", false, "Enable FUSE debug logging")
	flag.Parse()

	logf("starting (data-dir=%s, mountpoint=%s, acl=%s)", *dataDir, *mountpoint, *aclPath)

	// ── Unseal ────────────────────────────────────────────────────────────────

	sealedPath := filepath.Join(*dataDir, ".sealed_seed")
	sealed, err := os.ReadFile(sealedPath)
	if err != nil {
		die("reading sealed seed (has t2-provision been run?): %v", err)
	}

	fingerprint, err := internal.ReadHardwareFingerprint()
	if err != nil {
		die("reading hardware fingerprint: %v", err)
	}

	seed, err := internal.UnsealSeed(sealed, fingerprint)
	if err != nil {
		die("unsealing seed (wrong hardware?): %v", err)
	}
	logf("seed unsealed")

	// ── Load ACL ──────────────────────────────────────────────────────────────

	acl, err := vaultfs.LoadACL(*aclPath)
	if err != nil {
		// Non-fatal: warn loudly, but start with empty ACL (deny-all).
		// Services can still mount but all reads will fail with EACCES.
		// This is preferable to refusing to start and blocking all Docker services.
		logf("WARNING: could not load ACL from %s: %v", *aclPath, err)
		logf("WARNING: all secret reads will be denied until ACL is corrected and daemon restarted")
		acl = vaultfs.EmptyACL()
	} else {
		logf("ACL loaded from %s", *aclPath)
	}

	// ── Mount ─────────────────────────────────────────────────────────────────

	if err := os.MkdirAll(*mountpoint, 0755); err != nil {
		die("creating mount point %s: %v", *mountpoint, err)
	}

	root := &vaultfs.VaultRoot{
		DataDir: *dataDir,
		Seed:    seed,
		ACL:     acl,
	}

	server, err := fs.Mount(*mountpoint, root, &fs.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: true,
			FsName:     "vault-t2",
			Name:       "vault-t2-fuse",
			Debug:      *debug,
		},
	})
	if err != nil {
		die("mounting FUSE filesystem at %s: %v", *mountpoint, err)
	}
	logf("mounted at %s", *mountpoint)
	internal.AuditLog(*dataDir, "fuse-mount", "", os.Getpid(), true, "")

	// ── Wait for signal ───────────────────────────────────────────────────────

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		s := <-sig
		logf("received %s — unmounting", s)
		if err := server.Unmount(); err != nil {
			logf("unmount error: %v", err)
		}
	}()

	server.Wait()
	internal.AuditLog(*dataDir, "fuse-unmount", "", os.Getpid(), true, "")
	logf("exited cleanly")
}

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "vault-t2-fuse: "+format+"\n", args...)
}

func die(format string, args ...any) {
	logf("error: "+format, args...)
	os.Exit(1)
}
