# Quick Start Guide

Get up and running with Pi5 Vault in 15 minutes.

## Prerequisites Checklist

- [ ] YubiKey 5 (x2 recommended - primary + backup)
- [ ] Windows 10/11 laptop
- [ ] Raspberry Pi 5 with SSH access
- [ ] YubiKey Manager installed on Windows

## 5-Minute Setup

### Step 1: Install YubiKey Manager (Windows)

```powershell
# Download from: https://www.yubico.com/support/download/yubikey-manager/
# Install and verify:
ykman --version
```

### Step 2: Program YubiKeys

**IMPORTANT: You need TWO YubiKeys with the SAME seed!**

```powershell
# Program PRIMARY YubiKey:
ykman oath accounts add "Pi5 Vault"
# Let it generate a seed, or enter your own
# SAVE THE SEED - you need it for backup YubiKey!

# Remove primary, insert BACKUP YubiKey:
ykman oath accounts add "Pi5 Vault"
# Enter SAME seed as primary (critical!)

# Verify both programmed:
ykman oath accounts list
# Should show: Pi5 Vault

# Test they generate same codes (swap YubiKeys quickly):
ykman oath accounts code "Pi5 Vault"
# Both should show same 6-digit code (within 30-second window)
```

**After setup:**
- Primary YubiKey: Keep with you (keychain/laptop)
- Backup YubiKey: Store in safe
- Both work identically

**Optional - Google Authenticator (skip this if you want maximum security):**
```powershell
# ONLY for disaster recovery (programming new YubiKeys if both lost)
# NOT for daily vault access - stays on your phone, never on pi5
ykman oath accounts uri "Pi5 Vault"
# Scan QR code with Google Authenticator app on phone
```

**For detailed instructions with verification and recovery setup:**
See full design doc (linked in CLAUDE.md) → Initialization section

### Step 3: Setup Pi5

```bash
# SSH to pi5
ssh pi5.local

# Clone/copy this repo
# Build vault binary
cd pi5-vault/pi5
chmod +x build.sh
./build.sh

# Install
sudo install -m 755 vault* /usr/local/bin/

# Create secrets directory
sudo mkdir -p /mnt/data/secrets
sudo chown $USER:$USER /mnt/data/secrets
chmod 700 /mnt/data/secrets
```

### Step 4: Configure SSH (Windows)

Edit `C:\Users\YourName\.ssh\config`:

```
Host pi5
  HostName pi5.local
  User your-username
  RemoteForward 3000 localhost:3000
  ServerAliveInterval 60
```

### Step 5: Test It!

**Terminal 1 (Windows):**
```powershell
cd pi5-vault\windows\powershell
.\vault-auth-proxy.ps1
# Should show: "✓ Running on http://localhost:3000"
# Leave this running
```

**Terminal 2 (Windows → SSH to pi5):**
```bash
ssh pi5

# Create a test secret
echo "hello_world" | vault-set test
Requesting session key from laptop YubiKey...
# [Touch YubiKey when prompted]
✓ Session key cached (valid for 30 minutes)
✓ Secret encrypted and stored

# Retrieve it
vault-get test
# Should output: hello_world

# Clean up
vault-delete test
```

## Success!

You now have a working YubiKey-based secret management system.

## Next Steps

1. **Add your real secrets:**
   ```bash
   echo "your_db_password" | vault-set db_password
   echo "sk-abc123..." | vault-set openai_api_key
   ```

2. **Update your scripts to use vault:**
   ```bash
   # Old way:
   DB_PASSWORD="hardcoded_secret"

   # New way:
   DB_PASSWORD=$(vault-get db_password)
   ```

3. **Setup additional laptops:**
   - Repeat steps 1, 2, 4 on each laptop (~10 minutes)
   - Each laptop can access secrets identically

4. **Store backup YubiKey safely:**
   - Put in safe/secure location
   - Only needed if primary is lost

## Common First-Time Issues

**"ykman: command not found"**
- Install YubiKey Manager from Yubico website
- Restart PowerShell after installation

**"cannot reach auth proxy"**
- Is auth proxy running? Check Terminal 1
- Are you SSH'd from the same laptop?
- Test: `curl http://localhost:3000/health` on pi5

**"secret not found"**
- Did you create it? Run `vault-list` to see all secrets
- Check permissions: `ls -la /mnt/data/secrets/`

**SSH tunnel not working**
- Verify SSH config has `RemoteForward 3000 localhost:3000`
- Reconnect SSH: `exit` then `ssh pi5` again
- Check on pi5: `ss -tlnp | grep 3000` should show port listening

## Help

- Full documentation: See `README.md`
- Troubleshooting: See README.md troubleshooting section
- Design details: See `CLAUDE.md` for link to full design doc
