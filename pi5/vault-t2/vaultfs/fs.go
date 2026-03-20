// Package vaultfs implements the FUSE read-only virtual filesystem for vault-t2.
//
// Mount point: /run/vault-t2-fs/
// Each file in the mount corresponds to a .enc file in the data directory.
// Files are decrypted on demand when read; plaintext never touches disk.
//
// Access control: every Open() checks the caller's UID against the ACL loaded
// from /etc/vault-t2/acl.yaml. Callers whose UID is not listed for the
// requested secret receive EACCES. Read() re-checks as defense in depth.
//
// The ACL file is root:root 0644 — no user-mode process can modify it.
// The daemon reads it once at startup; ACL changes require a daemon restart.
package vaultfs

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"gopkg.in/yaml.v3"

	"github.com/philj/vault-t2/internal"
)

// ─── ACL ──────────────────────────────────────────────────────────────────────

// ACL maps secret names to the set of UIDs permitted to read them.
// The zero value (empty map) denies all access — callers must use LoadACL.
type ACL struct {
	entries map[string]map[uint32]struct{}
}

// LoadACL reads and parses the ACL config file.
//
// Expected YAML format:
//
//	db_password:
//	  - 65001
//	api_key_openai:
//	  - 65002
//	  - 65003   # shared secret example
func LoadACL(path string) (ACL, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ACL{}, fmt.Errorf("reading ACL file %s: %w", path, err)
	}

	var raw map[string][]uint32
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return ACL{}, fmt.Errorf("parsing ACL file %s: %w", path, err)
	}

	acl := ACL{entries: make(map[string]map[uint32]struct{}, len(raw))}
	for secret, uids := range raw {
		set := make(map[uint32]struct{}, len(uids))
		for _, uid := range uids {
			set[uid] = struct{}{}
		}
		acl.entries[secret] = set
	}
	return acl, nil
}

// EmptyACL returns an ACL that denies all access.
// Used as a safe fallback when acl.yaml cannot be loaded.
func EmptyACL() ACL {
	return ACL{entries: make(map[string]map[uint32]struct{})}
}

// Allowed reports whether uid is permitted to read the named secret.
func (a ACL) Allowed(secret string, uid uint32) bool {
	set, ok := a.entries[secret]
	if !ok {
		return false
	}
	_, ok = set[uid]
	return ok
}

// ─── Root node ────────────────────────────────────────────────────────────────

// VaultRoot is the root inode of the FUSE virtual filesystem.
// Fields are exported so cmd/vault-t2-fuse/main.go can set them.
type VaultRoot struct {
	fs.Inode

	DataDir string // /mnt/data/vault-t2
	Seed    []byte // unsealed tier2_seed (32 bytes, AES-256 key)
	ACL     ACL
}

var (
	_ fs.NodeGetattrer = (*VaultRoot)(nil)
	_ fs.NodeReaddirer = (*VaultRoot)(nil)
	_ fs.NodeLookuper  = (*VaultRoot)(nil)
)

// Getattr returns root directory metadata.
func (v *VaultRoot) Getattr(_ context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = syscall.S_IFDIR | 0755
	return fs.OK
}

// Readdir lists all secret names (without .enc extension).
// All secrets are visible regardless of ACL — permissions are enforced at Open.
func (v *VaultRoot) Readdir(_ context.Context) (fs.DirStream, syscall.Errno) {
	des, err := os.ReadDir(v.DataDir)
	if err != nil {
		return nil, syscall.EIO
	}

	var entries []fuse.DirEntry
	for _, de := range des {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".enc") {
			continue
		}
		entries = append(entries, fuse.DirEntry{
			Name: strings.TrimSuffix(de.Name(), ".enc"),
			Mode: syscall.S_IFREG | 0400,
		})
	}
	return fs.NewListDirStream(entries), fs.OK
}

// Lookup resolves a filename to a SecretNode inode.
// Reads only the 4-byte plaintext-length prefix from the .enc file — no full
// decryption at Lookup time.
func (v *VaultRoot) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	encPath := filepath.Join(v.DataDir, name+".enc")

	// Open and read just the first 4 bytes (plaintext length prefix).
	f, err := os.Open(encPath)
	if err != nil {
		return nil, syscall.ENOENT
	}
	defer f.Close()

	var sizeBuf [4]byte
	if _, err := f.Read(sizeBuf[:]); err != nil {
		return nil, syscall.EIO
	}
	size := binary.BigEndian.Uint32(sizeBuf[:])

	sn := &secretNode{
		name:    name,
		dataDir: v.DataDir,
		seed:    v.Seed,
		acl:     v.ACL,
		size:    size,
	}

	out.Attr.Mode = syscall.S_IFREG | 0400
	out.Attr.Size = uint64(size)

	child := v.NewInode(ctx, sn, fs.StableAttr{Mode: syscall.S_IFREG})
	return child, fs.OK
}

// ─── Secret file node ─────────────────────────────────────────────────────────

// secretNode represents a single virtual secret file.
// It is created by VaultRoot.Lookup and lives only in the inode cache.
type secretNode struct {
	fs.Inode

	name    string // secret name, no .enc extension
	dataDir string
	seed    []byte
	acl     ACL
	size    uint32 // plaintext length, read from 4-byte prefix at Lookup time
}

var (
	_ fs.NodeGetattrer = (*secretNode)(nil)
	_ fs.NodeOpener    = (*secretNode)(nil)
	_ fs.NodeReader    = (*secretNode)(nil)
)

// Getattr returns file size and permissions without decrypting.
func (s *secretNode) Getattr(_ context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0400 // r--------
	out.Size = uint64(s.size)
	return fs.OK
}

// Open enforces the UID ACL. Callers whose UID is not listed for this secret
// receive EACCES. No file handle is returned — reads go to Read() on the node.
func (s *secretNode) Open(ctx context.Context, _ uint32) (fs.FileHandle, uint32, syscall.Errno) {
	caller, ok := fuse.FromContext(ctx)
	if !ok {
		// FUSE context unavailable — deny as a safe default.
		return nil, 0, syscall.EACCES
	}
	if !s.acl.Allowed(s.name, caller.Uid) {
		internal.AuditLog(s.dataDir, "fuse-denied", s.name, int(caller.Pid), false, "uid not in ACL")
		return nil, 0, syscall.EACCES
	}
	return nil, fuse.FOPEN_DIRECT_IO, fs.OK
}

// Read decrypts the secret and returns the requested byte range.
// FOPEN_DIRECT_IO (set in Open) means the kernel will not cache the result,
// so the caller always gets a fresh decrypt. This ensures updated secrets are
// served correctly without a daemon restart.
func (s *secretNode) Read(ctx context.Context, _ fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	caller, ok := fuse.FromContext(ctx)
	if !ok {
		// FUSE context unavailable — deny as a safe default.
		return nil, syscall.EACCES
	}
	pid := int(caller.Pid)
	// Re-check ACL as defense in depth.
	if !s.acl.Allowed(s.name, caller.Uid) {
		return nil, syscall.EACCES
	}

	encPath := filepath.Join(s.dataDir, s.name+".enc")
	payload, err := os.ReadFile(encPath)
	if err != nil {
		internal.AuditLog(s.dataDir, "fuse-read", s.name, pid, false, err.Error())
		return nil, syscall.EIO
	}

	plaintext, err := internal.DecryptSecret(payload, s.seed)
	if err != nil {
		internal.AuditLog(s.dataDir, "fuse-read", s.name, pid, false, err.Error())
		return nil, syscall.EIO
	}

	internal.AuditLog(s.dataDir, "fuse-read", s.name, pid, true, "")

	if off >= int64(len(plaintext)) {
		return fuse.ReadResultData([]byte{}), fs.OK
	}
	end := off + int64(len(dest))
	if end > int64(len(plaintext)) {
		end = int64(len(plaintext))
	}
	return fuse.ReadResultData(plaintext[off:end]), fs.OK
}
