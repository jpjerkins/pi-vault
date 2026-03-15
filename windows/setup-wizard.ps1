# Pi5 Vault Setup Wizard for Windows
# This script helps you verify prerequisites and set up your YubiKeys

$ErrorActionPreference = "Stop"

function Write-Header {
    param([string]$Text)
    Write-Host ""
    Write-Host "═══════════════════════════════════════════════════════" -ForegroundColor Cyan
    Write-Host "  $Text" -ForegroundColor Cyan
    Write-Host "═══════════════════════════════════════════════════════" -ForegroundColor Cyan
    Write-Host ""
}

function Write-Step {
    param([string]$Text)
    Write-Host "➤ $Text" -ForegroundColor Yellow
}

function Write-Success {
    param([string]$Text)
    Write-Host "✓ $Text" -ForegroundColor Green
}

function Write-Error {
    param([string]$Text)
    Write-Host "✗ $Text" -ForegroundColor Red
}

function Write-Info {
    param([string]$Text)
    Write-Host "  $Text" -ForegroundColor Gray
}

function Test-YkmanInstalled {
    try {
        $null = Get-Command ykman -ErrorAction Stop
        return $true
    }
    catch {
        return $false
    }
}

function Get-YkmanVersion {
    try {
        $output = ykman --version 2>&1 | Out-String
        return $output.Trim()
    }
    catch {
        return "unknown"
    }
}

function Test-YubiKeyPresent {
    try {
        $null = & ykman list 2>&1
        return $LASTEXITCODE -eq 0
    }
    catch {
        return $false
    }
}

function Get-YubiKeyAccounts {
    try {
        $accounts = & ykman oath accounts list 2>&1
        return $accounts
    }
    catch {
        return @()
    }
}

# Main script
Clear-Host
Write-Header "Pi5 Vault Setup Wizard"

Write-Info "This wizard will help you:"
Write-Info "  1. Verify prerequisites"
Write-Info "  2. Program your YubiKeys"
Write-Info "  3. Test the setup"
Write-Host ""

# Step 1: Check YubiKey Manager
Write-Step "Checking for YubiKey Manager (ykman)..."

if (Test-YkmanInstalled) {
    $version = Get-YkmanVersion
    Write-Success "YubiKey Manager installed: $version"
}
else {
    Write-Error "YubiKey Manager not found!"
    Write-Info "Download from: https://www.yubico.com/support/download/yubikey-manager/"
    Write-Info "After installing, re-run this script."
    exit 1
}

# Step 2: Check for YubiKey
Write-Step "Checking for YubiKey..."

if (Test-YubiKeyPresent) {
    Write-Success "YubiKey detected"

    # Show YubiKey info
    $info = & ykman info 2>&1
    Write-Info "YubiKey Info:"
    $info | ForEach-Object { Write-Info "  $_" }
}
else {
    Write-Error "No YubiKey detected!"
    Write-Info "Please insert your YubiKey and try again."
    exit 1
}

# Step 3: Check for existing Pi5 Vault credential
Write-Step "Checking for existing 'Pi5 Vault' credential..."

$accounts = Get-YubiKeyAccounts
if ($accounts -match "Pi5 Vault") {
    Write-Success "Pi5 Vault credential already exists on this YubiKey"
    Write-Host ""
    $response = Read-Host "Do you want to delete and re-program it? (y/N)"

    if ($response -eq 'y' -or $response -eq 'Y') {
        Write-Step "Deleting existing credential..."
        & ykman oath accounts delete "Pi5 Vault" --force
        Write-Success "Deleted"
    }
    else {
        Write-Info "Keeping existing credential. Moving to test..."
        $programNew = $false
    }
}
else {
    Write-Info "No existing Pi5 Vault credential found."
    $programNew = $true
}

# Step 4: Program YubiKey (if needed)
if ($programNew -or ($response -eq 'y' -or $response -eq 'Y')) {
    Write-Host ""
    Write-Header "Programming YubiKey"

    Write-Info "You have two options:"
    Write-Info "  1. Auto-generate secret (easy, but can't copy to backup YubiKey)"
    Write-Info "  2. Enter your own secret (recommended for backup YubiKey)"
    Write-Host ""

    $choice = Read-Host "Choose option (1 or 2)"

    if ($choice -eq "2") {
        Write-Host ""
        Write-Info "Generate a random base32 secret or use an existing one."
        Write-Info "Valid characters: A-Z, 2-7 (no lowercase, no 0,1,8,9)"
        Write-Info "Recommended length: 32 characters"
        Write-Host ""

        Write-Info "Quick secret generator:"

        # Generate a random base32 secret
        $bytes = New-Object byte[] 20
        $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
        $rng.GetBytes($bytes)

        # Convert to base32 (simplified, not perfect but works for demo)
        $base32chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
        $secret = ""
        for ($i = 0; $i -lt $bytes.Length; $i++) {
            $secret += $base32chars[$bytes[$i] % 32]
        }

        Write-Info "  Suggested: $secret"
        Write-Host ""
        Write-Host "⚠ SAVE THIS SECRET SECURELY! You'll need it for your backup YubiKey!" -ForegroundColor Yellow
        Write-Host ""

        $userSecret = Read-Host "Enter secret (or press Enter to use suggested)"
        if ([string]::IsNullOrWhiteSpace($userSecret)) {
            $userSecret = $secret
        }

        Write-Step "Programming YubiKey with your secret..."
        $userSecret | & ykman oath accounts add "Pi5 Vault" --oath-type TOTP --touch
    }
    else {
        Write-Step "Programming YubiKey with auto-generated secret..."
        Write-Info "Press Enter when prompted for secret (auto-generates)."
        & ykman oath accounts add "Pi5 Vault" --oath-type TOTP --touch
    }

    if ($LASTEXITCODE -eq 0) {
        Write-Success "YubiKey programmed successfully!"
    }
    else {
        Write-Error "Failed to program YubiKey"
        exit 1
    }
}

# Step 5: Test TOTP generation
Write-Host ""
Write-Step "Testing TOTP code generation..."
Write-Info "Your YubiKey will blink - touch the gold contact when it does."
Write-Host ""

Start-Sleep -Seconds 1

try {
    $code = & ykman oath accounts code "Pi5 Vault" --single 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Success "TOTP code generated: $code"
        Write-Info "This code changes every 30 seconds."
    }
    else {
        Write-Error "Failed to generate code: $code"
        exit 1
    }
}
catch {
    Write-Error "Error generating code: $_"
    exit 1
}

# Step 6: Offer to program backup YubiKey
Write-Host ""
Write-Host "════════════════════════════════════════════════════════" -ForegroundColor Cyan
$response = Read-Host "Do you want to program your BACKUP YubiKey now? (y/N)"

if ($response -eq 'y' -or $response -eq 'Y') {
    Write-Host ""
    Write-Info "Remove your PRIMARY YubiKey and insert your BACKUP YubiKey."
    Read-Host "Press Enter when ready"

    # Wait for YubiKey change
    Write-Step "Detecting YubiKey..."
    Start-Sleep -Seconds 2

    if (Test-YubiKeyPresent) {
        Write-Success "YubiKey detected"

        # Check if already has credential
        $accounts = Get-YubiKeyAccounts
        if ($accounts -match "Pi5 Vault") {
            Write-Info "This YubiKey already has a Pi5 Vault credential."
            $delete = Read-Host "Delete and re-program? (y/N)"
            if ($delete -eq 'y' -or $delete -eq 'Y') {
                & ykman oath accounts delete "Pi5 Vault" --force
            }
            else {
                Write-Info "Skipping backup programming."
                $programBackup = $false
            }
        }

        if ($programBackup -ne $false) {
            Write-Host ""
            Write-Info "⚠ IMPORTANT: Use the SAME secret as your primary YubiKey!" -ForegroundColor Yellow
            Write-Info "This ensures both YubiKeys can decrypt your secrets."
            Write-Host ""

            $backupSecret = Read-Host "Enter the same secret you used for primary YubiKey"

            if (![string]::IsNullOrWhiteSpace($backupSecret)) {
                Write-Step "Programming backup YubiKey..."
                $backupSecret | & ykman oath accounts add "Pi5 Vault" --oath-type TOTP --touch

                if ($LASTEXITCODE -eq 0) {
                    Write-Success "Backup YubiKey programmed!"

                    # Test it
                    Write-Step "Testing backup YubiKey..."
                    Write-Info "Touch when it blinks..."
                    $backupCode = & ykman oath accounts code "Pi5 Vault" --single 2>&1
                    Write-Success "Backup code: $backupCode"

                    Write-Host ""
                    Write-Info "✓ Both YubiKeys should generate the SAME code at the same time!"
                }
                else {
                    Write-Error "Failed to program backup YubiKey"
                }
            }
            else {
                Write-Info "Skipped backup programming."
            }
        }
    }
    else {
        Write-Error "No YubiKey detected. Skipping backup programming."
    }
}

# Step 7: Next steps
Write-Host ""
Write-Header "Setup Complete!"

Write-Success "Your YubiKey(s) are ready for Pi5 Vault!"
Write-Host ""
Write-Info "Next steps:"
Write-Info "  1. Build and deploy vault binary on Pi5 (see SETUP-WALKTHROUGH.md)"
Write-Info "  2. Start the auth proxy on this laptop:"
Write-Info "     - PowerShell: .\windows\powershell\vault-auth-proxy.ps1"
Write-Info "     - Go: cd windows\go && .\build.ps1 && .\vault-auth-proxy.exe"
Write-Info "  3. Set up SSH tunnel from Pi5 to this laptop"
Write-Info "  4. Test with: vault-get / vault-set commands"
Write-Host ""
Write-Info "📖 Full instructions: docs\SETUP-WALKTHROUGH.md"
Write-Info "📋 Quick checklist: docs\SETUP-CHECKLIST.md"
Write-Info "🔑 YubiKey help: docs\YUBIKEY-BASICS.md"
Write-Host ""

# Offer to start auth proxy
$startProxy = Read-Host "Start the PowerShell auth proxy now? (y/N)"

if ($startProxy -eq 'y' -or $startProxy -eq 'Y') {
    Write-Host ""
    Write-Info "Starting auth proxy..."
    Write-Info "Keep this window open! Press Ctrl+C to stop."
    Write-Host ""
    Start-Sleep -Seconds 2

    & "$PSScriptRoot\powershell\vault-auth-proxy.ps1"
}
else {
    Write-Info "You can start it later with:"
    Write-Info "  .\windows\powershell\vault-auth-proxy.ps1"
}

Write-Host ""
Write-Host "Good luck! 🔐" -ForegroundColor Green
