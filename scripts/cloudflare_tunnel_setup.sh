#!/bin/bash

set -e

# Function to display progress with a spinner
show_progress() {
    local pid=$!
    local delay=0.1
    local spinstr='|/-\'
    echo -n " "
    while [ "$(ps a | awk '{print $1}' | grep $pid)" ]; do
        local temp=${spinstr#?}
        printf " [%c]  " "$spinstr"
        local spinstr=$temp${spinstr%"$temp"}
        sleep $delay
        printf "\b\b\b\b\b\b"
    done
    echo " [✔]"
}

# Check for required arguments
if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <tunnel_name> <domain>"
    echo "Example: $0 mytunnel korotole.com"
    exit 1
fi

# Assign arguments to variables
TUNNEL_NAME=$1
DOMAIN=$2

echo "Cloudflare Tunnel Setup Script for Ubuntu Server"
echo "================================================"
echo "Tunnel Name: $TUNNEL_NAME"
echo "Domain: $DOMAIN"
echo

# Step 1: Update & Upgrade System
echo "Updating and upgrading system packages..."
sudo apt-get update & show_progress
sudo apt-get upgrade -y & show_progress

# Step 2: Install Prerequisites
echo "Installing prerequisites (curl, apt-transport-https)..."
sudo apt-get install -y curl apt-transport-https & show_progress

# Step 3: Add Cloudflare's GPG Key
echo "Adding Cloudflare GPG key..."
sudo mkdir -p --mode=0755 /usr/share/keyrings
curl -fsSL https://pkg.cloudflare.com/cloudflare-main.gpg | sudo tee /usr/share/keyrings/cloudflare-main.gpg >/dev/null & show_progress

# Step 4: Add Cloudflare Repository
echo "Adding Cloudflare repository to APT sources..."
echo 'deb [signed-by=/usr/share/keyrings/cloudflare-main.gpg] https://pkg.cloudflare.com/cloudflared jammy main' | sudo tee /etc/apt/sources.list.d/cloudflared.list & show_progress

# Step 5: Install Cloudflared
echo "Installing Cloudflared..."
sudo apt-get update & show_progress
sudo apt-get install -y cloudflared & show_progress

# Step 6: Authenticate with Cloudflare
echo "Authenticating with Cloudflare..."
cloudflared tunnel login & show_progress

# Step 7: Create a Tunnel
echo "Creating a new Cloudflare tunnel named '$TUNNEL_NAME'..."
cloudflared tunnel create "$TUNNEL_NAME" & show_progress

# Step 8: Configure Tunnel
CONFIG_DIR="$HOME/.cloudflared"
CONFIG_FILE="$CONFIG_DIR/config.yml"
TUNNEL_ID=$(cloudflared tunnel list | grep "$TUNNEL_NAME" | awk '{print $1}')

echo "Setting up tunnel configuration..."
mkdir -p "$CONFIG_DIR"
cat <<EOF | tee "$CONFIG_FILE"
tunnel: $TUNNEL_ID
credentials-file: $CONFIG_DIR/$TUNNEL_ID.json

ingress:
  - hostname: $DOMAIN
    service: http://localhost:8080
  - service: http_status:404
EOF

# Step 9: Configure DNS Routing
echo "Configuring DNS routing for $DOMAIN..."
cloudflared tunnel route dns "$TUNNEL_ID" "$DOMAIN" & show_progress

# Step 10: Run the Tunnel
echo "Starting the Cloudflare tunnel..."
cloudflared tunnel --config "$CONFIG_FILE" run "$TUNNEL_NAME" & show_progress

echo "Cloudflare Tunnel setup completed successfully!"
