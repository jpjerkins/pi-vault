#!/bin/bash
# Pi5 Vault Setup Script
# Run this on your Raspberry Pi 5 to set up the vault

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
GRAY='\033[0;90m'
NC='\033[0m' # No Color

function print_header() {
    echo ""
    echo -e "${CYAN}═══════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}  $1${NC}"
    echo -e "${CYAN}═══════════════════════════════════════════════════════${NC}"
    echo ""
}

function print_step() {
    echo -e "${YELLOW}➤ $1${NC}"
}

function print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

function print_error() {
    echo -e "${RED}✗ $1${NC}"
}

function print_info() {
    echo -e "${GRAY}  $1${NC}"
}

# Main script
clear
print_header "Pi5 Vault Setup"

print_info "This script will:"
print_info "  1. Check prerequisites"
print_info "  2. Build vault binary"
print_info "  3. Install to /usr/local/bin"
print_info "  4. Create secrets directory"
print_info "  5. Test the installation"
echo ""

# Step 1: Check for Go
print_step "Checking for Go compiler..."

if command -v go &> /dev/null; then
    GO_VERSION=$(go version | awk '{print $3}')
    print_success "Go installed: $GO_VERSION"
else
    print_error "Go compiler not found!"
    print_info "Install with: sudo apt install golang"
    exit 1
fi

# Step 2: Check we're in the right directory
print_step "Checking script location..."

if [ ! -f "vault.go" ]; then
    print_error "vault.go not found in current directory!"
    print_info "Please run this script from the pi5/ directory:"
    print_info "  cd /path/to/pi5-vault/pi5"
    print_info "  ./setup.sh"
    exit 1
fi

print_success "In correct directory"

# Step 3: Build vault binary
print_step "Building vault binary..."

if [ -f "build.sh" ]; then
    chmod +x build.sh
    ./build.sh
    print_success "Build complete"
else
    print_error "build.sh not found!"
    exit 1
fi

# Step 4: Install binaries
print_step "Installing vault binaries to /usr/local/bin..."

if [ -f "vault-get" ]; then
    sudo install -m 755 vault-* /usr/local/bin/
    print_success "Installed:"
    print_info "  /usr/local/bin/vault-get"
    print_info "  /usr/local/bin/vault-set"
    print_info "  /usr/local/bin/vault-list"
    print_info "  /usr/local/bin/vault-delete"
else
    print_error "vault-get not found! Build may have failed."
    exit 1
fi

# Step 5: Create secrets directory
print_step "Creating secrets directory..."

SECRETS_DIR="/mnt/data/secrets"

if [ ! -d "$SECRETS_DIR" ]; then
    sudo mkdir -p "$SECRETS_DIR"
    print_success "Created $SECRETS_DIR"
else
    print_info "Directory already exists: $SECRETS_DIR"
fi

# Set ownership and permissions
print_step "Setting ownership and permissions..."

sudo chown $USER:$USER "$SECRETS_DIR"
chmod 700 "$SECRETS_DIR"

print_success "Ownership: $USER:$USER"
print_success "Permissions: 700 (owner only)"

# Step 6: Check if secrets dir is writable
if [ -w "$SECRETS_DIR" ]; then
    print_success "Secrets directory is writable"
else
    print_error "Cannot write to secrets directory!"
    print_info "Check permissions: ls -ld $SECRETS_DIR"
    exit 1
fi

# Step 7: Test vault commands
print_step "Testing vault commands..."

if command -v vault-list &> /dev/null; then
    print_success "vault-list command available"

    # Try running vault-list
    if vault-list &> /dev/null; then
        print_success "vault-list executed successfully"
        print_info "Current secrets: $(vault-list | wc -l) secrets"
    else
        print_info "vault-list executed (no secrets yet)"
    fi
else
    print_error "vault-list not in PATH!"
    exit 1
fi

# Step 8: Check SSH tunnel capability
print_header "SSH Tunnel Setup"

print_info "The vault needs to reach your Windows laptop's auth proxy."
print_info "This requires an SSH reverse tunnel:"
echo ""
print_info "  ssh -R 3000:localhost:3000 YOUR_USER@YOUR_LAPTOP"
echo ""

read -p "Do you want to test the tunnel connection now? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    print_step "Testing connection to localhost:3000..."

    if curl -s http://localhost:3000/health > /dev/null 2>&1; then
        print_success "Auth proxy is reachable!"
        print_info "Response: $(curl -s http://localhost:3000/health)"
    else
        print_error "Cannot reach auth proxy on localhost:3000"
        print_info "Make sure:"
        print_info "  1. Auth proxy is running on your Windows laptop"
        print_info "  2. SSH tunnel is active: ssh -R 3000:localhost:3000 user@laptop"
    fi
fi

# Step 9: Offer to set up autossh
echo ""
read -p "Do you want to set up persistent SSH tunnel with autossh? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    print_step "Checking for autossh..."

    if command -v autossh &> /dev/null; then
        print_success "autossh already installed"
    else
        print_step "Installing autossh..."
        sudo apt install -y autossh
        print_success "autossh installed"
    fi

    echo ""
    print_info "Creating systemd service for persistent tunnel..."
    echo ""
    read -p "Enter your laptop username: " LAPTOP_USER
    read -p "Enter your laptop hostname or IP: " LAPTOP_HOST

    SERVICE_FILE="/etc/systemd/system/vault-tunnel.service"

    print_step "Creating service file: $SERVICE_FILE"

    sudo tee "$SERVICE_FILE" > /dev/null <<EOF
[Unit]
Description=Pi5 Vault SSH Tunnel to Laptop
After=network.target

[Service]
Type=simple
User=$USER
ExecStart=/usr/bin/autossh -M 0 -N -R 3000:localhost:3000 ${LAPTOP_USER}@${LAPTOP_HOST} -o ServerAliveInterval=30 -o ServerAliveCountMax=3
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

    print_success "Service file created"

    print_step "Enabling service..."
    sudo systemctl daemon-reload
    sudo systemctl enable vault-tunnel

    echo ""
    print_info "Service configured but NOT started yet."
    print_info "Make sure SSH key authentication is set up first!"
    print_info "Then start with: sudo systemctl start vault-tunnel"
fi

# Step 10: Summary
print_header "Setup Complete!"

print_success "Pi5 Vault is installed and ready!"
echo ""
print_info "Next steps:"
print_info "  1. Make sure auth proxy is running on your Windows laptop"
print_info "  2. Set up SSH tunnel (manual or autossh):"
print_info "     Manual: ssh -R 3000:localhost:3000 user@laptop"
print_info "     AutoSSH: sudo systemctl start vault-tunnel"
print_info "  3. Test with a secret:"
print_info "     echo \"test123\" | vault-set test_secret"
print_info "     vault-get test_secret"
print_info "     vault-delete test_secret"
echo ""
print_info "📖 Full instructions: docs/SETUP-WALKTHROUGH.md"
print_info "📋 Quick checklist: docs/SETUP-CHECKLIST.md"
echo ""

# Offer to run a quick test
echo ""
read -p "Do you want to run a quick end-to-end test now? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo ""
    print_header "End-to-End Test"

    print_step "Checking auth proxy connection..."
    if ! curl -s http://localhost:3000/health > /dev/null 2>&1; then
        print_error "Cannot reach auth proxy!"
        print_info "Make sure:"
        print_info "  1. Auth proxy is running on Windows"
        print_info "  2. SSH tunnel is active"
        print_info "Then re-run this test manually."
        exit 1
    fi

    print_success "Auth proxy is reachable"

    echo ""
    print_step "Setting a test secret..."
    print_info "You'll need to touch your YubiKey when prompted on Windows!"

    if echo "test123" | vault-set test_secret 2>&1; then
        print_success "Secret stored!"

        echo ""
        print_step "Retrieving the secret..."
        RESULT=$(vault-get test_secret 2>/dev/null)

        if [ "$RESULT" = "test123" ]; then
            print_success "Secret retrieved correctly: $RESULT"

            echo ""
            print_step "Listing secrets..."
            vault-list

            echo ""
            print_step "Deleting test secret..."
            vault-delete test_secret
            print_success "Test secret deleted"

            echo ""
            print_header "All Tests Passed! 🎉"
            print_success "Your Pi5 Vault is fully operational!"
        else
            print_error "Secret retrieved but value is wrong!"
            print_info "Expected: test123"
            print_info "Got: $RESULT"
        fi
    else
        print_error "Failed to store secret"
        print_info "Check the error message above"
    fi
fi

echo ""
print_info "Setup complete. Happy secret managing! 🔐"
echo ""
