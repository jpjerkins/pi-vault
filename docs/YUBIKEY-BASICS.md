# YubiKey Basics for Pi5 Vault

New to YubiKeys? This guide explains what you need to know.

## What is a YubiKey?

A YubiKey is a **hardware security key** - a physical device that stores cryptographic secrets and can generate authentication codes. Think of it as a USB key that generates passwords, but the secrets never leave the device.

## How We Use It in Pi5 Vault

We use the **OATH-TOTP** feature of your YubiKey:

1. **OATH** = Initiative for Open Authentication (a standard)
2. **TOTP** = Time-based One-Time Password (changes every 30 seconds)

The YubiKey stores a secret and generates 6-digit codes based on:
- The secret (stored on the YubiKey)
- The current time (30-second windows)

**Why this is secure:**
- The secret NEVER leaves the YubiKey
- You can't extract the secret even if you have the YubiKey
- Each code is only valid for 30 seconds
- Requires physical touch to generate codes

## YubiKey Manager (ykman)

The `ykman` command-line tool is how we interact with YubiKeys on your computer.

### Common Commands

**List all TOTP credentials on your YubiKey:**
```powershell
ykman oath accounts list
```

Output example:
```
Pi5 Vault
Gmail:yourname@gmail.com
GitHub:yourname
```

**Get a TOTP code (requires touch):**
```powershell
ykman oath accounts code "Pi5 Vault"
```

Touch the YubiKey when it blinks, and you'll see:
```
Pi5 Vault    123456
```

That 6-digit code changes every 30 seconds.

**Add a new TOTP credential:**
```powershell
ykman oath accounts add "My Service" --oath-type TOTP --touch-required
```

**Delete a credential:**
```powershell
ykman oath accounts delete "My Service"
```

**Check YubiKey info:**
```powershell
ykman info
```

Shows serial number, firmware version, enabled features.

## Understanding Touch-Required

When you add a credential with `--touch-required`, the YubiKey will:

1. **Blink** when a code is requested
2. **Wait** for you to physically touch the gold contact
3. **Generate** the code only after touch

**Why use touch-required?**
- Malware can't silently read codes
- You must be physically present
- Provides confirmation before sensitive operations

**How to touch:**
- Lightly tap the gold contact area
- Don't hold it (just a quick tap)
- The LED will stop blinking when touch is registered

## Programming Multiple YubiKeys (Backup)

YubiKeys can store **multiple TOTP credentials** (typically 32 slots).

For Pi5 Vault, you want **two YubiKeys with the SAME credential** so either can decrypt your secrets.

### Method 1: Same Secret (Recommended)

When you program the first YubiKey:
```powershell
ykman oath accounts add "Pi5 Vault" --oath-type TOTP --touch-required
```

**If you enter your own secret**, save it! You'll use it again for the backup.

Then for the second YubiKey:
```powershell
ykman oath accounts add "Pi5 Vault" --oath-type TOTP --touch-required
# Enter THE SAME SECRET you used for the first key
```

Now both YubiKeys generate **identical codes** at the same time.

### Method 2: Auto-Generated Secret

If you press Enter at the "Enter a secret key" prompt, the YubiKey auto-generates a random secret.

**Problem:** You can't easily copy this to a second YubiKey.

**Solution:** Use Method 1 and provide your own secret.

### Generating a Secret

If you want to generate a strong random secret yourself:

**On Linux/Mac:**
```bash
openssl rand -base32 20
```

**On Windows (PowerShell):**
```powershell
$bytes = New-Object byte[] 20
$rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
$rng.GetBytes($bytes)
[Convert]::ToBase32String($bytes)
```

Or just use a password manager to generate a random base32 string (A-Z, 2-7, no padding).

## What Happens in Pi5 Vault?

Here's the flow:

1. **You run:** `vault-get my_secret` on Pi5
2. **Pi5 checks:** Is session key cached and valid?
   - If YES: Use cached key, decrypt secret, done!
   - If NO: Request new session key from Windows laptop...
3. **Windows auth proxy:** Calls `ykman oath accounts code "Pi5 Vault"`
4. **YubiKey blinks:** You touch it
5. **YubiKey generates:** 6-digit TOTP code (e.g., "123456")
6. **Auth proxy derives:** Session key = SHA256(TOTP + time_window)
   - Example: SHA256("123456-123456") → 32-byte session key
7. **Session key sent** to Pi5 via SSH tunnel
8. **Pi5 caches** session key for 30 minutes
9. **Pi5 decrypts** secret using session key (AES-256-GCM)
10. **Secret returned** to you

**Next time** (within 30 minutes):
- Step 2 finds cached key → skip steps 3-8
- No YubiKey touch needed!

## Security Properties

**Why this is secure:**

1. **Secrets encrypted at rest** - Your secrets on Pi5 are encrypted with AES-256-GCM
2. **YubiKey required** - Can't decrypt without the YubiKey
3. **Touch required** - Malware can't silently steal codes
4. **Session keys expire** - 30-minute window limits exposure
5. **Audit trail** - All operations logged
6. **No plaintext secrets** - Secrets never stored unencrypted on disk

**Attack scenarios this prevents:**

- **Pi5 compromised?** Secrets are encrypted, attacker can't read them
- **Laptop compromised?** Auth proxy requires physical touch, malware can't automate
- **Network sniffer?** SSH tunnel is encrypted
- **Stolen backup?** Encrypted secrets are useless without YubiKey
- **Lost YubiKey?** Attacker still needs to know which TOTP credential to use, and physical access to Pi5

## Common YubiKey Gotchas

### 1. Time Sync Matters

TOTP codes are based on time. If your Windows laptop and Pi5 have different times (off by more than 30 seconds), **the same TOTP code will generate different session keys**.

**Fix:**
```bash
# On Pi5
sudo ntpdate pool.ntp.org

# On Windows
# Settings → Time & Language → Sync now
```

### 2. YubiKey Not Detected

**Symptoms:**
- `ykman` command not found
- "No YubiKey detected"

**Fixes:**
- Reinstall YubiKey Manager
- Try different USB port
- Remove and reinsert YubiKey
- Restart computer (Windows sometimes needs this)

### 3. Multiple YubiKeys Plugged In

If you have multiple YubiKeys plugged in, `ykman` may not know which to use.

**Fix:** Only plug in ONE YubiKey at a time.

### 4. Credential Name Typo

If you program your YubiKey with "Pi5Vault" (no space) but the code expects "Pi5 Vault" (with space), it won't work.

**The name must be EXACTLY:** `Pi5 Vault`

### 5. Touch Timeout

YubiKeys have a ~15-second timeout. If you don't touch it within 15 seconds of the request, it times out.

**Fix:** Just retry the operation.

## YubiKey Best Practices

1. **Buy two YubiKeys** (you did!) - One primary, one backup
2. **Store backup separately** - Different physical location (e.g., safe deposit box)
3. **Test backup regularly** - Verify it works, don't discover it's broken when you need it
4. **Label your YubiKeys** - Use a label maker to mark "Primary" and "Backup"
5. **Don't share secrets** - Never give someone your YubiKey TOTP secret
6. **Protect PINs** - YubiKeys can have PINs; never use default PINs
7. **Keep firmware updated** - Check Yubico's website for firmware updates
8. **Register serial numbers** - Yubico lets you register your keys for support

## Advanced: What's Inside a YubiKey?

A YubiKey has several independent applications:

1. **OTP** (Yubico OTP) - Generates 44-character codes
2. **OATH** (TOTP/HOTP) - What we use for Pi5 Vault
3. **PIV** (Smart Card) - For certificate-based auth
4. **OpenPGP** - For GPG keys
5. **FIDO U2F / FIDO2** - For web authentication (like passwordless login)

Each application is **completely separate** - you can use all of them on the same YubiKey without interference.

For Pi5 Vault, we **only use OATH-TOTP**.

## Troubleshooting YubiKey Issues

### "No YubiKey detected"

```powershell
# Check if Windows sees the device
Get-PnpDevice | Where-Object {$_.FriendlyName -like "*Yubico*"}
```

Should show Yubico device. If not, try different USB port or reboot.

### "Failed to connect to YubiKey"

YubiKey Manager might be running in the background.

```powershell
# Close YubiKey Manager GUI if open
# Then retry command
```

### "Touch timeout"

You didn't touch within 15 seconds.

**Fix:** Just retry, and touch faster this time.

### "Invalid base32 secret"

When programming, you entered an invalid secret.

**Valid characters:** A-Z, 2-7 (no lowercase, no 0, 1, 8, 9)

### "Account already exists"

You already have a credential with that name.

**Fix:**
```powershell
# Delete it first
ykman oath accounts delete "Pi5 Vault"

# Then add again
ykman oath accounts add "Pi5 Vault" --oath-type TOTP --touch-required
```

## Additional Resources

- **Yubico Documentation:** https://docs.yubico.com/
- **YubiKey Manager Download:** https://www.yubico.com/support/download/yubikey-manager/
- **OATH/TOTP Guide:** https://developers.yubico.com/OATH/
- **Support:** https://support.yubico.com/

## Questions?

See `SETUP-WALKTHROUGH.md` for complete setup instructions, or check the troubleshooting section in `README.md`.

Good luck with your YubiKeys! 🔑
