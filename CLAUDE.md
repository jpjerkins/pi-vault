# Pi5 Vault - Claude Code Context

This is a YubiKey-based secret management system for Raspberry Pi 5.

## Design Documentation
The documentation for this project is in the `1 Projects/Pi5 Vault` folder in my notes.

**Full design and architecture documentation:**

**Windows (Nextcloud synced):**
```
Pi5 Secret Management - On-Demand Design.md
```

**Pi5 (if Nextcloud synced to ~/Nextcloud):**
```
Pi5 Secret Management - On-Demand Design.md
```

**Direct path (Windows):**
```
Pi5 Secret Management - On-Demand Design.md
```

## Quick Overview

**What this is:**
- On-demand secret decryption using YubiKey authentication
- Secrets encrypted on disk (AES-256-GCM)
- Session keys derived from YubiKey TOTP
- No container runtime needed
- Works with multiple Windows laptops + SSH

**Components:**
1. `pi5/vault.go` - Go binary for pi5 (decrypt/encrypt secrets)
2. `windows/powershell/vault-auth-proxy.ps1` - PowerShell auth proxy for Windows
3. `windows/go/vault-auth-proxy.go` - Go auth proxy alternative

**How it works:**
1. App on pi5 calls `vault-get secret_name`
2. vault-get requests session key from Windows laptop via SSH tunnel
3. Auth proxy on laptop prompts for YubiKey tap
4. YubiKey generates TOTP → session key derived
5. Session key cached for 30 minutes
6. Secret decrypted and returned

## Development Guidelines

**When modifying this codebase:**
- Read the design doc first (linked above)
- Follow the security model (no secrets in plaintext on disk)
- Maintain session key caching (30min TTL)
- Keep audit logging for all operations
- Test with both YubiKeys (primary and backup)
- Verify works from multiple Windows laptops

**Key security properties to maintain:**
- AES-256-GCM for all encryption
- Session keys expire after 30 minutes
- File permissions: 0600 for all secret files
- Audit log all access (success and failure)
- No long-running processes with secrets in memory

**Architecture principles:**
- Keep it simple (no unnecessary abstractions)
- Fast startup (binary execution, not container)
- Platform-agnostic where possible
- Windows-native auth proxy (no WSL required)
- Multi-laptop support (SSH tunnel follows active session)

## Building

**On Pi5:**
```bash
cd pi5
./build.sh
sudo install -m 755 vault* /usr/local/bin/
```

**On Windows (Go auth proxy):**
```powershell
cd windows\go
.\build.ps1
```

**On Windows (PowerShell auth proxy):**
No build needed - just run the .ps1 script

## Testing

**Test session key derivation:**
```bash
# On laptop (start auth proxy)
.\vault-auth-proxy.ps1

# On pi5 (via SSH)
curl http://localhost:3000/derive-key -X POST
# Should prompt for YubiKey tap and return session key
```

**Test secret encryption/decryption:**
```bash
# Set a test secret
echo "test_value" | vault-set test_secret

# Get it back
vault-get test_secret
# Should return: test_value

# List secrets
vault-list
# Should show: test_secret

# Delete test secret
vault-delete test_secret
```

## Implementation Status

**✅ Tier 1 (complete):**
- [x] Go binary for pi5 (vault.go)
- [x] PowerShell auth proxy
- [x] Go auth proxy
- [x] Session key caching
- [x] AES-256-GCM encryption
- [x] Audit logging
- [x] CLI commands (get, set, list, delete)

**✅ Tier 2 — Phase 1 (complete):**
- [x] `pi5/vault-t2/internal/crypto.go` — seal/unseal, encrypt/decrypt (4-byte length prefix), audit log
- [x] `pi5/vault-t2/internal/hardware.go` — hardware fingerprint from machine-id + CPU serial
- [x] `pi5/vault-t2/cmd/vault-t2/main.go` — CLI with symlink dispatch (t2-provision, t2-get, t2-set, t2-list, t2-delete)

**✅ Tier 2 — Phase 2 (complete):**
- [x] `pi5/vault-t2/vaultfs/fs.go` — FUSE filesystem with UID ACL enforcement
- [x] `pi5/vault-t2/cmd/vault-t2-fuse/main.go` — daemon: unseal → load ACL → mount → signal handling
- [x] `pi5/vault-t2/acl.yaml.example` — ACL config template with UID range documentation
- [x] `/etc/fuse.conf` `user_allow_other` configured on pi5

**✅ Tier 2 — Phase 3 (complete):**
- [x] `pi5/vault-t2/cmd/vault-t2-acl-update/main.go` — privileged helper: validates and installs `/etc/vault-t2/acl.yaml` (run via sudo)
- [x] `~/dcm/lib/registry.py` — `vault_uid` and `vault_secrets` fields added to `ServiceConfig`
- [x] `~/dcm/lib/secrets.py` — `generate_vault_acl()` and `vault_acl_drift()` functions
- [x] `~/dcm/dcm.py` — `dcm secrets acl-generate` command; drift warnings in `dcm secrets sync` and `dcm secrets check`

**✅ Tier 2 — Phase 4 (complete):**
- [x] `pi5/vault-t2/vaultfs/fs.go` — `EnvFiles` type, `LoadEnvFiles()`, `envfilesDirNode`, `envfileNode` (KEY=VALUE generation on read, UID ACL)
- [x] `pi5/vault-t2/cmd/vault-t2-fuse/main.go` — `--envfiles` flag, loads `envfiles.yaml` at startup (non-fatal if missing)
- [x] `pi5/vault-t2/envfiles.yaml.example` — example config template

**✅ Tier 2 — Phase 5 (complete):**
- [x] `pi5/vault-t2/vault-t2-fuse.service` — systemd unit (Before=docker.service, RequiresMountsFor=/mnt/data)
- [x] `pi5/vault-t2/vault-t2.tmpfiles.conf` — recreates `/run/vault-t2-fs` on boot
- [x] `pi5/vault-t2/docs/install.md` — full first-time setup guide

**🚧 Tier 1 TODO (Future):**
- [ ] Initialization wizard (YubiKey programming)
- [ ] HTTP API for web apps
- [ ] Recovery passphrase system
- [ ] Secret versioning
- [ ] Web UI for secret management
- [ ] Automated testing suite

## Project Structure

```
pi5-vault/
├── pi5/                          # Pi5 binaries
│   ├── vault.go                  # Tier 1 main implementation
│   ├── build.sh                  # Tier 1 build script
│   └── vault-t2/                 # Tier 2 Go module
│       ├── cmd/
│       │   ├── vault-t2/
│       │   │   └── main.go       # Tier 2 CLI entrypoint
│       │   ├── vault-t2-fuse/
│       │   │   └── main.go       # FUSE daemon entrypoint
│       │   └── vault-t2-acl-update/
│       │       └── main.go       # Privileged ACL update helper
│       ├── internal/
│       │   ├── crypto.go         # Seal/unseal/encrypt/decrypt/audit
│       │   └── hardware.go       # Hardware fingerprint derivation
│       ├── vaultfs/
│       │   └── fs.go             # FUSE filesystem + UID ACL
│       ├── acl.yaml.example      # ACL config template
│       ├── envfiles.yaml.example # envfiles config template
│       ├── build-t2.sh           # Tier 2 build script
│       ├── go.mod
│       └── go.sum
├── windows/                      # Windows auth proxies
│   ├── powershell/
│   │   └── vault-auth-proxy.ps1  # PowerShell version
│   └── go/
│       ├── vault-auth-proxy.go   # Go version
│       └── build.ps1             # Build script
├── docs/                         # Additional documentation
├── README.md                     # User documentation
├── CLAUDE.md                     # This file (dev context)
└── .gitignore                    # Git ignore rules
```

## Related Documentation

- **User Guide:** `README.md`
- **Full Design:** See design doc link above
- **Troubleshooting:** See README.md troubleshooting section

## Notes for Claude Code

When working on this project:
1. Always check the design doc for architectural context
2. Security is critical - review threat model before changes
3. Test with actual YubiKey if making auth proxy changes
4. Maintain backward compatibility with existing encrypted secrets
5. Update README.md if user-facing behavior changes
6. Keep code simple and readable (no unnecessary complexity)

## Contact

Project owner: Phil Jerkins
Infrastructure: pi5.local (Raspberry Pi 5)
