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
if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <tunnel_name>"
    echo "Example: $0 mytunnel"
    exit 1
fi

# Assign argument to variable
TUNNEL_NAME=$1

echo "Cloudflare Tunnel Cleanup Script for Ubuntu Server"
echo "=================================================="
echo "Tunnel Name: $TUNNEL_NAME"
echo

# Step 1: Stop any running Cloudflare tunnels
echo "Stopping any running Cloudflare tunnels..."
sudo pkill -f cloudflared || echo "No running Cloudflare tunnel found."

# Step 2: Delete the Cloudflare tunnel
echo "Fetching tunnel ID for '$TUNNEL_NAME'..."
TUNNEL_ID=$(cloudflared tunnel list 2>/dev/null | grep "$TUNNEL_NAME" | awk '{print $1}')

if [ -n "$TUNNEL_ID" ]; then
    echo "Deleting the Cloudflare tunnel '$TUNNEL_NAME' with ID $TUNNEL_ID..."
    cloudflared tunnel delete "$TUNNEL_ID" & show_progress
else
    echo "Tunnel '$TUNNEL_NAME' not found. Skipping deletion."
fi

# Step 3: Remove Cloudflare DNS route if present
echo "Removing DNS routes associated with the tunnel..."
cloudflared tunnel route dns show 2>/dev/null | grep "$TUNNEL_ID" | awk '{print $1}' | xargs -r -I{} cloudflared tunnel route dns delete {}

# Step 4: Remove cloudflared configuration files
CONFIG_DIR="$HOME/.cloudflared"
echo "Removing Cloudflare configuration files in $CONFIG_DIR..."
sudo rm -rf "$CONFIG_DIR" & show_progress

# Step 5: Uninstall Cloudflared
echo "Uninstalling Cloudflared..."
sudo apt-get remove --purge -y cloudflared & show_progress
sudo apt-get autoremove -y & show_progress

# Step 6: Remove Cloudflare repository and GPG key
echo "Removing Cloudflare APT repository and GPG key..."
sudo rm -f /etc/apt/sources.list.d/cloudflared.list & show_progress
sudo rm -f /usr/share/keyrings/cloudflare-main.gpg & show_progress
sudo apt-get update & show_progress

# Step 7: Clean up any remaining files
echo "Cleaning up remaining files..."
sudo rm -rf /var/log/cloudflared & show_progress

echo "Cloudflare Tunnel cleanup completed successfully!"
