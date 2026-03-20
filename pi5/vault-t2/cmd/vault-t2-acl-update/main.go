// vault-t2-acl-update — privileged helper that validates and installs /etc/vault-t2/acl.yaml
//
// Reads a proposed acl.yaml from stdin, validates the structure, then
// atomically writes it to /etc/vault-t2/acl.yaml.  Must be run as root.
//
// Usage (typically via dcm):
//
//	dcm secrets acl-generate | sudo vault-t2-acl-update
//
// After updating the ACL, restart the FUSE daemon to pick up the change:
//
//	sudo systemctl restart vault-t2-fuse
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	aclPath    = "/etc/vault-t2/acl.yaml"
	aclDirPerm = 0755
	aclPerm    = 0644
)

func main() {
	if os.Getuid() != 0 {
		fatalf("must be run as root (use: dcm secrets acl-generate | sudo vault-t2-acl-update)")
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fatalf("reading stdin: %v", err)
	}

	// Validate: must be parseable as map[string][]uint32.
	var raw map[string][]uint32
	if err := yaml.Unmarshal(data, &raw); err != nil {
		fatalf("invalid ACL YAML: %v", err)
	}

	// Validate entries: no empty secret names, no secret with zero UIDs.
	for secret, uids := range raw {
		if secret == "" {
			fatalf("ACL contains an empty secret name")
		}
		if len(uids) == 0 {
			fatalf("secret %q has no UIDs — remove it or add at least one UID", secret)
		}
	}

	// Ensure the config directory exists.
	aclDir := filepath.Dir(aclPath)
	if err := os.MkdirAll(aclDir, aclDirPerm); err != nil {
		fatalf("creating config dir %s: %v", aclDir, err)
	}

	// Atomic write: temp file → chmod → rename into place.
	tmp := aclPath + ".tmp"
	if err := os.WriteFile(tmp, data, aclPerm); err != nil {
		fatalf("writing temp file: %v", err)
	}
	if err := os.Rename(tmp, aclPath); err != nil {
		os.Remove(tmp)
		fatalf("installing ACL file: %v", err)
	}

	fmt.Fprintf(os.Stderr, "✓ %s updated (%d secret(s))\n", aclPath, len(raw))
	fmt.Fprintf(os.Stderr, "  Restart daemon to apply: sudo systemctl restart vault-t2-fuse\n")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "vault-t2-acl-update: "+format+"\n", args...)
	os.Exit(1)
}
