# Pi5 Vault - YubiKey-Based Secret Management

On-demand secret decryption system for Raspberry Pi using YubiKey authentication. Secrets stay encrypted on disk and are decrypted only when needed using session keys derived from your YubiKey.

## 🚀 First Time Here?

**Got your YubiKeys today?** Start here:

📘 **[START-HERE.md](START-HERE.md)** - Your complete quick-start guide

**New to YubiKeys?** Read this first:

🔑 **[docs/YUBIKEY-BASICS.md](docs/YUBIKEY-BASICS.md)** - YubiKey primer for beginners

**Ready to set up?** Use these guides:

- 📋 **[docs/SETUP-CHECKLIST.md](docs/SETUP-CHECKLIST.md)** - Simple checkbox format
- 📖 **[docs/SETUP-WALKTHROUGH.md](docs/SETUP-WALKTHROUGH.md)** - Detailed step-by-step
- 🪟 **[windows/setup-wizard.ps1](windows/setup-wizard.ps1)** - Interactive Windows setup
- 🥧 **[pi5/setup.sh](pi5/setup.sh)** - Interactive Pi5 setup

**Or just read on** for the complete user manual...

## Features

- 🔐 **YubiKey Authentication** - Physical security key required for access
- 💾 **Encrypted at Rest** - Secrets stored as AES-256-GCM encrypted files
- ⚡ **On-Demand Decryption** - No long-running container, instant access
- 🔄 **Session Caching** - Tap YubiKey once per 30 minutes
- 🪟 **Windows Native** - PowerShell or Go auth proxy (no WSL needed)
- 🔁 **Backup YubiKey** - Two YubiKeys with identical functionality
- 📝 **Audit Logging** - Track all secret access
- 🌐 **Multi-Platform** - Works from any Windows laptop via SSH

## Architecture

```
[Windows Laptop]              [Raspberry Pi 5]
     |                              |
  YubiKey -----> Auth Proxy -------|----> vault-get binary
     |           (PowerShell        |         |
     |            or Go)             |         v
     |                              |    Encrypted Secrets
     |<-------- SSH Tunnel ---------|    (.enc files)
```

**How it works:**
1. App on pi5 needs a secret → calls `vault-get db_password`
2. vault-get checks for cached session key (30min TTL)
3. If expired, requests new key from Windows auth proxy via SSH tunnel
4. Auth proxy prompts: "Touch YubiKey"
5. You tap YubiKey → session key derived and cached
6. vault-get decrypts secret using session key
7. Secret returned to app

## Quick Start

### Prerequisites

**On Windows Laptop:**
- YubiKey 5 (or compatible) - [Buy here](https://www.yubico.com/products/)
- YubiKey Manager - [Download](https://www.yubico.com/support/download/yubikey-manager/)
- SSH client (built into Windows 10/11)
- PowerShell (built-in) OR Go 1.19+ (for Go auth proxy)

**On Raspberry Pi 5:**
- Go 1.19+ - `sudo apt install golang-go`
- SSH server - `sudo apt install openssh-server`

### Installation

#### 1. Setup YubiKey

**You need TWO YubiKeys:**
- Primary (daily use, keep with you)
- Backup (emergency use, store in safe)

**Both must have the SAME TOTP seed** to work interchangeably.

**Quick setup:**

```powershell
# Verify ykman is installed
ykman --version

# Program PRIMARY YubiKey
ykman oath accounts add "Pi5 Vault"
# Save the seed shown - you'll need it for backup!

# Remove primary, insert BACKUP YubiKey
ykman oath accounts add "Pi5 Vault" <same-seed-as-primary>

# Verify both work
ykman oath accounts code "Pi5 Vault"
```

**⚠️ CRITICAL:** Store backup YubiKey in safe. If both are lost, you're locked out!

**Optional: Google Authenticator (Disaster Recovery Only)**

```powershell
# OPTIONAL: Add TOTP seed to phone for disaster recovery
ykman oath accounts uri "Pi5 Vault"
# Scan QR code with Google Authenticator app

# ⚠️ IMPORTANT: Google Authenticator is ONLY for:
#   - Programming new YubiKeys if both are lost
#   - NOT for daily vault access
#   - NOT for accessing vault from phone
#   - Stays on your phone, never on pi5
```

**Security note:** Google Authenticator is optional. Skip it if you want maximum security. The TOTP seed never goes on pi5 - only on YubiKeys (in your possession) and optionally on your phone.

**For detailed step-by-step instructions with verification and recovery setup, see:**
- Full design doc (linked in CLAUDE.md) → "Initialization" section
- Includes: verification steps, recovery document, security model explanation

#### 2. Build Pi5 Vault Binary

On your Raspberry Pi:

```bash
# Clone or copy this repo
cd ~/pi5-vault/pi5

# Build
./build.sh

# Install system-wide
sudo install -m 755 vault* /usr/local/bin/

# Create secrets directory
sudo mkdir -p /mnt/data/secrets
sudo chown $USER:$USER /mnt/data/secrets
chmod 700 /mnt/data/secrets
```

#### 3. Setup Windows Auth Proxy

On your Windows laptop:

**Option A: PowerShell (Easiest)**

```powershell
# No build needed - just run the script
cd windows\powershell
.\vault-auth-proxy.ps1

# Optional: Create desktop shortcut
# Target: powershell.exe -ExecutionPolicy Bypass -NoExit -File "C:\path\to\vault-auth-proxy.ps1"
```

**Option B: Go Binary**

```powershell
# Build the .exe
cd windows\go
.\build.ps1

# Run
.\vault-auth-proxy.exe
```

#### 4. Configure SSH Tunnel

On your Windows laptop, edit `C:\Users\YourName\.ssh\config`:

```
Host pi5
  HostName pi5.local
  User phil
  RemoteForward 3000 localhost:3000
  ServerAliveInterval 60
```

## Usage

### Daily Workflow

**Morning:**
```powershell
# 1. Plug in YubiKey

# 2. Start auth proxy
.\vault-auth-proxy.ps1
# Leave this running (minimize window if desired)

# 3. SSH to pi5 in another terminal
ssh pi5
```

**Working with secrets:**
```bash
# First access in 30-minute window
pi5$ vault-get db_password
Requesting session key from laptop YubiKey...

# [Laptop shows: "🔐 Touch YubiKey to derive session key..."]
# [You tap YubiKey]

✓ Session key cached (valid for 30 minutes)
postgres_secret_123

# Subsequent accesses (within 30min) - no YubiKey tap needed
pi5$ vault-get api_key
sk-abc123def456...

pi5$ vault-get github_token
ghp_xyz789...
```

### Managing Secrets

```bash
# Set a secret (from stdin)
pi5$ echo "my_secret_value" | vault-set db_password
Requesting session key from laptop YubiKey...
[Tap YubiKey]
✓ Secret encrypted and stored

# Set a secret (as argument)
pi5$ vault-set api_key "sk-abc123def456"
✓ Secret encrypted and stored

# List all secrets
pi5$ vault-list
db_password
api_key
github_token

# Delete a secret
pi5$ vault-delete old_secret
✓ Secret deleted
```

### Using in Scripts

**Bash:**
```bash
#!/bin/bash
# deploy.sh

DB_PASSWORD=$(vault-get db_password)
API_KEY=$(vault-get api_key)

export DATABASE_URL="postgresql://user:$DB_PASSWORD@localhost/mydb"
export OPENAI_API_KEY="$API_KEY"

docker-compose up -d
```

**Python:**
```python
#!/usr/bin/env python3
import subprocess

def get_secret(name):
    result = subprocess.run(['vault-get', name],
                          capture_output=True, text=True, check=True)
    return result.stdout.strip()

db_password = get_secret('db_password')
api_key = get_secret('api_key')
```

**Node.js:**
```javascript
const { execSync } = require('child_process');

function getSecret(name) {
    return execSync(`vault-get ${name}`).toString().trim();
}

const dbPassword = getSecret('db_password');
const apiKey = getSecret('api_key');
```

## Multiple Laptop Support

The system works seamlessly with multiple Windows laptops:

**Laptop A (today):**
```powershell
# Start auth proxy
.\vault-auth-proxy.ps1

# SSH to pi5
ssh pi5

# Apps use Laptop A's YubiKey
```

**Laptop B (tomorrow):**
```powershell
# Start auth proxy on Laptop B
.\vault-auth-proxy.ps1

# SSH to pi5
ssh pi5

# Apps now use Laptop B's YubiKey
# Everything works identically
```

Setup each laptop once:
1. Install YubiKey Manager
2. Copy auth proxy script/binary
3. Configure SSH config
4. Done! (~10 minutes per laptop)

## Backup YubiKey

Both YubiKeys are programmed with the same TOTP seed, so they generate identical codes and derive identical session keys.

**If primary YubiKey is lost:**
1. Get backup YubiKey from safe
2. Use it exactly like the primary
3. Optional: Buy new YubiKey and program with same seed

**Backup YubiKey storage:**
- Keep in physical safe
- Store with recovery passphrase
- Only needed if primary is lost/damaged

## Security

### Core Security Model

**What's on pi5:**
- ✅ Encrypted secrets (`.enc` files)
- ✅ Cached session key (30min TTL, auto-expires)
- ✅ Audit log

**What's NOT on pi5:**
- ❌ TOTP seed (never stored anywhere on pi5)
- ❌ YubiKey secrets
- ❌ Master password
- ❌ Way to derive session keys without YubiKey

**This is critical:** A root attacker on pi5 cannot derive new session keys after the 30-minute cache expires. They would need physical access to your YubiKey + laptop + SSH tunnel.

### Where TOTP Seed Exists

**The TOTP seed is the master secret. It exists ONLY in your possession:**

1. ✅ Primary YubiKey (on your keychain/laptop)
2. ✅ Backup YubiKey (in your safe)
3. ✅ Optional: Google Authenticator (on your phone - disaster recovery only)
4. ✅ Optional: Printed recovery document (in your safe)

**NEVER:**
- ❌ On pi5 (no files, no config, nowhere)
- ❌ In the cloud
- ❌ In source code
- ❌ In any automated system

### Encryption
- **Algorithm:** AES-256-GCM (authenticated encryption)
- **Key Derivation:** SHA256(TOTP_from_YubiKey + time_window)
- **Session Keys:** Derived on laptop, cached on pi5 for 30min, auto-expire
- **File Permissions:** 0600 (owner read/write only)

### Threat Model

✅ **Protected Against:**
- Disk/backup theft (secrets encrypted, need YubiKey TOTP)
- Physical pi5 access (need YubiKey + laptop + SSH tunnel)
- Root access on pi5 (no TOTP seed = can't derive keys after cache expires)
- Offline brute force (TOTP changes every 30 sec)
- Stolen laptop alone (YubiKey can be removed)
- Stolen phone alone (need laptop + SSH access)

⚠️ **Partial Protection:**
- Root access on pi5 + within 30min window (cached session key might be fresh)
- Memory dumps during active use (secrets briefly in memory)
- SSH tunnel hijacking (could intercept session key derivation)

❌ **Not Protected Against:**
- Attacker with: YubiKey + laptop access + SSH credentials (all three)
- Sophisticated real-time memory monitoring during secret access

### Google Authenticator Security FAQ

**Q: If I add TOTP seed to Google Authenticator, doesn't that weaken security?**

A: Slightly, but it doesn't compromise the pi5 security model. Here's why:

**What Google Auth is for:**
- ✅ Programming new YubiKeys if both are lost (disaster recovery)
- ❌ NOT for accessing vault from phone
- ❌ NOT on pi5 (stays on your phone)

**Attack scenario - Stolen phone:**
```
Attacker has: Your phone with Google Auth
Attacker needs to also compromise:
  1. SSH access to pi5 (your SSH key or password)
  2. Your laptop (to run auth proxy)
  3. Active SSH tunnel

Without all three, TOTP seed alone is useless
```

**Attack scenario - Root on pi5:**
```
Attacker has: Root access on pi5
Can find: Encrypted secrets, maybe cached session key
Cannot find: TOTP seed (not on pi5, only on your devices)
Cannot do: Derive new session keys after 30min expires

Google Auth doesn't change this - TOTP seed still not on pi5
```

**Recommendation:**
- Use Google Auth if: You want disaster recovery + have strong phone security
- Skip Google Auth if: You want absolute maximum security

**Remember:** Google Auth is optional. The system is secure with or without it because the TOTP seed never goes on pi5.

### Best Practices

1. **Remove YubiKey when not in use** - Store securely, don't leave plugged in
2. **Keep backup YubiKey in safe** - Physical security, fireproof if possible
3. **Strong phone security** - If using Google Auth, use biometric + PIN
4. **Monitor audit log** - Review `/mnt/data/secrets/.audit.log` for unexpected access
5. **Use SSH key authentication** - Don't use passwords for SSH to pi5
6. **Keep session TTL short** - 30min is good balance (configurable in code)
7. **Minimize secret restarts** - Load secrets at app startup, keep apps running

## Troubleshooting

### "Cannot reach auth proxy"

**Check:**
- Is auth proxy running on laptop? (PowerShell window open)
- Are you SSH'd from the laptop running the proxy?
- Test: `curl http://localhost:3000/health` on pi5 should return `{"status":"running"}`

**Fix:**
```powershell
# On laptop, verify auth proxy is running
# If not, start it:
.\vault-auth-proxy.ps1
```

### "YubiKey error"

**Check:**
- Is YubiKey plugged into laptop?
- Does `ykman --version` work?
- Does YubiKey have "Pi5 Vault" OATH account?

**Fix:**
```powershell
# List OATH accounts
ykman oath accounts list

# If "Pi5 Vault" missing, add it:
ykman oath accounts add "Pi5 Vault" <your-seed>
```

### "Secret not found"

**Check:**
- Does the secret exist? Run `vault-list`
- File permissions on /mnt/data/secrets/

**Fix:**
```bash
# Check if secret file exists
ls -la /mnt/data/secrets/*.enc

# If missing, create it:
echo "secret_value" | vault-set secret_name
```

### SSH Tunnel Not Working

**Check:**
- Is `RemoteForward 3000 localhost:3000` in SSH config?
- On pi5: `netstat -tuln | grep 3000` should show port listening

**Fix:**
```bash
# On pi5, check if port 3000 is listening
ss -tlnp | grep 3000

# If not, reconnect SSH with verbose mode:
ssh -v pi5
# Look for "remote forward success" in output
```

## File Structure

```
/mnt/data/secrets/
├── db_password.enc           # Encrypted secrets
├── api_key_openai.enc
├── github_token.enc
├── .session_key              # Cached session key (30min TTL)
├── .session_expiry           # Expiry timestamp
└── .audit.log                # Access audit log
```

## Audit Log

All secret access is logged to `/mnt/data/secrets/.audit.log`:

```json
{"timestamp":"2026-03-12T10:30:15Z","action":"get","secret":"db_password","success":true}
{"timestamp":"2026-03-12T10:30:20Z","action":"set","secret":"api_key","success":true}
{"timestamp":"2026-03-12T10:31:00Z","action":"get","secret":"db_password","success":true}
```

View recent access:
```bash
tail -f /mnt/data/secrets/.audit.log | jq
```

## Advanced Usage

### Custom Session Key TTL

Edit `vault.go` and change:
```go
const SessionKeyTTL = 30 * time.Minute  // Change to desired duration
```

### Different Secrets Directory

Set environment variable:
```bash
export VAULT_SECRETS_DIR="/custom/path/secrets"
vault-get db_password
```

### HTTP API for Web Apps

See `docs/http-api.md` for building a REST API server that web apps can call.

## Architecture Details

For complete technical documentation, see:
- Design document: `CLAUDE.md` (links to full design in notes)
- Implementation details: See source code comments

## License

MIT License - see LICENSE file

## Contributing

This is a personal project for pi5.local infrastructure. Not accepting external contributions, but feel free to fork for your own use.

## Support

For issues or questions:
- **Security questions:** See `docs/SECURITY-FAQ.md` (addresses common concerns)
- **Troubleshooting:** See troubleshooting section above
- **Design details:** Review design document (linked in CLAUDE.md)
- **Implementation:** Check source code comments (simple Go, ~500 lines)

## Common Questions

**Q: Does Google Authenticator weaken security? Doesn't that put secrets on my phone?**

**A:** No. Google Auth is ONLY for disaster recovery (programming new YubiKeys if both are lost). It's NOT used for daily vault access. The TOTP seed never goes on pi5 - only on YubiKeys (in your possession) and optionally on your phone. See `docs/SECURITY-FAQ.md` for detailed explanation.

**Q: What if someone has root access to my pi5?**

**A:** They can only decrypt secrets for 30 minutes (cached session key). After that, they need your YubiKey + laptop + SSH tunnel to derive new keys. TOTP seed is never on pi5. See `docs/SECURITY-FAQ.md` for details.

**Q: Why not use HashiCorp Vault / Bitwarden / SOPS?**

**A:** Each has trade-offs. This vault is optimized for: single-node (pi5), physical security (YubiKey), service automation, and simplicity. See `docs/SECURITY-FAQ.md` for comparison.
