# 🔐 Pi5 Vault - Start Here!

Congratulations on getting your YubiKeys! Here's how to get started.

## 📚 Documentation Overview

I've created complete guides for you:

1. **START-HERE.md** (this file) - Overview and quick start
2. **docs/YUBIKEY-BASICS.md** - New to YubiKeys? Read this first!
3. **docs/SETUP-CHECKLIST.md** - Simple checkbox format to follow
4. **docs/SETUP-WALKTHROUGH.md** - Detailed step-by-step instructions
5. **README.md** - Complete user manual and reference

## 🚀 Quick Start (Recommended Path)

### Step 1: Learn About YubiKeys (10 minutes)

If you're new to YubiKeys, start here:

```
docs/YUBIKEY-BASICS.md
```

This explains:
- What a YubiKey is
- How TOTP works
- Common commands (`ykman`)
- How Pi5 Vault uses your YubiKey
- Troubleshooting tips

### Step 2: Set Up Windows (15 minutes)

**Run the setup wizard on your Windows laptop:**

```powershell
cd "C:\Local-only PARA\1 Projects\pi5-vault\windows"
.\setup-wizard.ps1
```

This interactive script will:
- ✓ Check if YubiKey Manager is installed
- ✓ Detect your YubiKey
- ✓ Program both YubiKeys with "Pi5 Vault" credential
- ✓ Test TOTP code generation
- ✓ Optionally start the auth proxy

**What you'll need:**
- Both YubiKeys (primary and backup)
- YubiKey Manager installed (wizard will check)

### Step 3: Set Up Pi5 (10 minutes)

**SSH to your Pi5 and run the setup script:**

```bash
cd /path/to/pi5-vault/pi5
./setup.sh
```

This script will:
- ✓ Check for Go compiler
- ✓ Build the vault binary
- ✓ Install to /usr/local/bin
- ✓ Create /mnt/data/secrets directory
- ✓ Set correct permissions
- ✓ Optionally set up persistent SSH tunnel (autossh)
- ✓ Run end-to-end test

### Step 4: Test Everything (5 minutes)

**On Windows:**
```powershell
.\windows\powershell\vault-auth-proxy.ps1
# Keep this running!
```

**On Pi5 (in separate terminal):**
```bash
# Create SSH tunnel
ssh -R 3000:localhost:3000 YOUR_USER@YOUR_LAPTOP_IP

# Keep this running!
```

**On Pi5 (in another terminal):**
```bash
# Store a secret
echo "test123" | vault-set test_secret
# Touch YubiKey when prompted on Windows!

# Retrieve it
vault-get test_secret
# Should output: test123

# List secrets
vault-list

# Delete test secret
vault-delete test_secret
```

**Success!** If all of that worked, your vault is fully operational! 🎉

## 📋 Alternative: Manual Setup with Checklist

If you prefer step-by-step manual setup:

```
docs/SETUP-CHECKLIST.md
```

This is a printable checkbox format you can follow along with.

For detailed explanations of each step:

```
docs/SETUP-WALKTHROUGH.md
```

## 🆘 Troubleshooting

### Common Issues

**"YubiKey Manager not found"**
- Download from: https://www.yubico.com/support/download/yubikey-manager/
- Install and restart PowerShell

**"No YubiKey detected"**
- Unplug and re-plug the YubiKey
- Try a different USB port
- Restart your computer

**"cannot reach auth proxy"**
- Is auth proxy running on Windows?
- Is SSH tunnel active?
- Test: `curl http://localhost:3000/health` on Pi5

**"decryption failed"**
- Different YubiKey with different secret?
- Time drift between Windows and Pi5?
- Fix: `sudo ntpdate pool.ntp.org` on Pi5

**"Touch timeout"**
- You have 15 seconds to touch the YubiKey
- Just retry the command

See **docs/SETUP-WALKTHROUGH.md** for more troubleshooting.

## 📖 Full Documentation

Once you're up and running, check out the complete docs:

**User Manual:**
```
README.md
```

Covers:
- How the system works
- CLI commands reference
- Integration with apps
- Security model
- Advanced usage

**Security FAQ:**
```
docs/SECURITY-FAQ.md
```

Explains:
- Threat model
- Why it's secure
- What happens if...
- Best practices

## 🎯 What's Implemented

The core system is **complete and ready to use**:

✅ **Pi5 Vault Binary**
- AES-256-GCM encryption
- Session key caching (30min)
- Audit logging
- CLI commands: get, set, list, delete

✅ **Windows Auth Proxy**
- PowerShell version (no build needed)
- Go version (cross-platform)
- YubiKey TOTP integration
- Session key derivation

✅ **Documentation**
- Setup guides
- YubiKey basics
- Security FAQ
- Full user manual

## 🚧 Future Enhancements (Not Yet Implemented)

These are planned for future versions:

- [ ] Initialization wizard
- [ ] HTTP API for web apps
- [ ] Recovery passphrase system
- [ ] Secret versioning
- [ ] Web UI
- [ ] Automated test suite

The core system is fully functional without these!

## 🔄 Workflow After Setup

Once everything is set up, your typical workflow will be:

**Daily use:**

1. Start auth proxy on Windows (once per day)
   ```powershell
   .\vault-auth-proxy.ps1
   ```

2. Apps on Pi5 call vault commands:
   ```bash
   DB_PASSWORD=$(vault-get db_password)
   API_KEY=$(vault-get api_key)
   ```

3. First access of the day: Touch YubiKey once
4. Subsequent accesses: Automatic (cached for 30min)

**SSH tunnel:**
- Manual: Keep `ssh -R` connection alive
- Automatic: Use autossh service (set up in `pi5/setup.sh`)

## 💡 Tips for Success

1. **Program both YubiKeys with the SAME secret** so either can decrypt
2. **Test your backup YubiKey** before storing it away
3. **Use autossh** for persistent tunnel (don't manually maintain SSH connection)
4. **Label your YubiKeys** - "Primary" and "Backup"
5. **Store backup safely** - separate location from primary
6. **Review audit logs** periodically: `cat /mnt/data/secrets/.audit.log`

## 🎓 Learning Path

**New to YubiKeys?**
1. Read: `docs/YUBIKEY-BASICS.md`
2. Run: `windows/setup-wizard.ps1`
3. Experiment with `ykman` commands

**Ready to set up?**
1. Read: `docs/SETUP-CHECKLIST.md` (overview)
2. Follow: `docs/SETUP-WALKTHROUGH.md` (detailed steps)
3. Or just run the wizards: `setup-wizard.ps1` and `setup.sh`

**Want to understand the system?**
1. Read: `README.md` (user manual)
2. Read: `docs/SECURITY-FAQ.md` (security model)
3. Read: Design doc (linked in `CLAUDE.md`)

**Ready to use it?**
1. Start auth proxy: `vault-auth-proxy.ps1`
2. Store secrets: `vault-set secret_name`
3. Use in apps: `vault-get secret_name`

## ✅ Pre-Flight Checklist

Before you start, make sure you have:

- [ ] Two YubiKeys (primary + backup)
- [ ] Windows laptop with YubiKey Manager installed
- [ ] Raspberry Pi 5 with SSH access
- [ ] Go compiler on Pi5 (`go version`)
- [ ] `/mnt/data` directory exists on Pi5
- [ ] 30 minutes of uninterrupted time

## 🚀 Ready?

**Recommended starting point:**

```powershell
# On Windows
cd "C:\Local-only PARA\1 Projects\pi5-vault\windows"
.\setup-wizard.ps1
```

This will guide you through everything!

**Questions?**
- Check: `docs/SETUP-WALKTHROUGH.md`
- Review: `docs/YUBIKEY-BASICS.md`
- Read: `README.md`

Good luck! 🔐

---

**Project Status:** ✅ Core implementation complete and ready to use!

**Next Step:** Run `windows/setup-wizard.ps1` to program your YubiKeys!
