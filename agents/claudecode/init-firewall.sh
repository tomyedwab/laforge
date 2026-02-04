#!/bin/bash
# Firewall initialization script for Claude Code agent containers
# Based on Anthropic's reference implementation but simplified for Laforge

set -e

# Fix workspace permissions for both agent (claudecode) and git (root) containers
# Use group permissions so both users can access the workspace
echo "Fixing workspace permissions..."

# Change group ownership to claudecode's group while keeping root as owner
chown -R root:claudecode /workspace
# Set group write permissions so claudecode user can write
chmod -R g+w /workspace
# Ensure new files created inherit the group
chmod g+s /workspace/repo

# Check for DEBUG mode - if set, skip firewall setup entirely
if [ "${FIREWALL_DEBUG:-0}" = "1" ]; then
    echo "FIREWALL_DEBUG mode enabled - skipping firewall initialization"
    exit 0
fi

echo "Initializing firewall rules..."

# Preserve Docker DNS rules before flushing
DOCKER_DNS_RULES=$(iptables-save -t nat | grep "127\.0\.0\.11" || true)

# Flush existing rules
iptables -F
iptables -X
iptables -t nat -F
iptables -t nat -X

# Restore Docker DNS
if [ -n "$DOCKER_DNS_RULES" ]; then
    echo "Restoring Docker DNS rules..."
    # Recreate Docker chains that were flushed
    iptables -t nat -N DOCKER_OUTPUT 2>/dev/null || true
    iptables -t nat -N DOCKER_POSTROUTING 2>/dev/null || true
    # Restore the saved rules
    echo "$DOCKER_DNS_RULES" | xargs -L 1 iptables -t nat
fi

# Allow loopback traffic
iptables -A INPUT -i lo -j ACCEPT
iptables -A OUTPUT -o lo -j ACCEPT

# Allow established and related connections
iptables -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT
iptables -A OUTPUT -m state --state ESTABLISHED,RELATED -j ACCEPT

# Allow DNS (UDP and TCP on port 53)
iptables -A OUTPUT -p udp --dport 53 -j ACCEPT
iptables -A OUTPUT -p tcp --dport 53 -j ACCEPT

# Allow Docker host network
# Detect the host network via the default route
HOST_NETWORK=$(ip route get 1.1.1.1 | grep -oP 'src \K[\d.]+' || echo "")
if [ -n "$HOST_NETWORK" ]; then
    # Extract the /24 network from the IP
    HOST_SUBNET=$(echo "$HOST_NETWORK" | sed 's/\.[0-9]*$/\.0\/24/')
    echo "Allowing Docker host network: $HOST_SUBNET"
    iptables -A OUTPUT -d "$HOST_SUBNET" -j ACCEPT
else
    echo "Warning: Could not detect host network, allowing common Docker ranges"
    # Fallback to common Docker network ranges
    iptables -A OUTPUT -d 172.17.0.0/16 -j ACCEPT
    iptables -A OUTPUT -d 172.18.0.0/16 -j ACCEPT
fi

# Set default policies to DROP
iptables -P INPUT DROP
iptables -P FORWARD DROP
iptables -P OUTPUT DROP

echo "Firewall initialization complete"
echo "Allowed traffic:"
echo "  - Loopback (127.0.0.1)"
echo "  - DNS (port 53)"
echo "  - Docker host network (for orchestrator communication)"
echo "  - Established/related connections"
echo ""
echo "All other outbound traffic is blocked by default"
