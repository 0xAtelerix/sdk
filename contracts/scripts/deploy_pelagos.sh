#!/bin/bash

# Pelagos & AppChain Deployment Script
# Usage: ./deploy_pelagos.sh [options]
# Options:
#   --private-key KEY       Private key for deployment
#   --rpc-url URL          RPC URL
#   --no-verify            Skip contract verification
#   --config-file FILE     Path to config.json file (default: config.json)
#   --source-chain-id ID   Source chain ID (default: 42)

set -e

# Default values
PRIVATE_KEY=""
RPC_URL=""
VERIFY=true
CONFIG_FILE="config/config.json"
APP_CHAIN_ID=42

# Load config.json file if it exists
load_config_json() {
    local config_file="$1"
    if [ -f "$config_file" ]; then
        echo "📄 Loading configuration from $config_file"
        if command -v jq >/dev/null 2>&1; then
            RPC_URL=$(jq -r '.rpcUrl // empty' "$config_file")
            PRIVATE_KEY=$(jq -r '.privateKey // empty' "$config_file")
            export ETHERSCAN_API_KEY=$(jq -r '.etherscanApiKey // empty' "$config_file")
            SOURCE_CHAIN_ID=$(jq -r '.appChainId // empty' "$config_file")
        else
            echo "❌ jq is required to parse config.json. Please install jq."
            exit 1
        fi
        echo "✅ Configuration loaded"
        echo ""
    fi
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --private-key)
            PRIVATE_KEY="$2"
            shift 2
            ;;
        --rpc-url)
            RPC_URL="$2"
            shift 2
            ;;
        --config-file)
            CONFIG_FILE="$2"
            shift 2
            ;;
        --source-chain-id)
            APP_CHAIN_ID="$2"
            shift 2
            ;;
        --no-verify)
            VERIFY=false
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [options]"
            echo "Options:"
            echo "  --private-key KEY       Private key for deployment"
            echo "  --rpc-url URL          RPC URL"
            echo "  --no-verify            Skip contract verification"
            echo "  --config-file FILE     Path to config.json file (default: config/config.json)"
            echo "  --source-chain-id ID   Appchain ID for registration (default: 42, change for production)"
            echo "  --help                 Show this help message"
            echo ""
            echo "Configuration (JSON config file):"
            echo "  {"
            echo "    \"rpcUrl\": \"https://eth-sepolia.g.alchemy.com/v2/...\","
            echo "    \"privateKey\": \"0xyour_private_key\","
            echo "    \"etherscanApiKey\": \"your_etherscan_api_key\","
            echo "    \"appChainId\": \"42\""
            echo "  }"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Navigate to contracts root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONTRACTS_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$CONTRACTS_ROOT"

# Load configuration file
load_config_json "$CONFIG_FILE"

# Check if private key is provided
if [ -z "$PRIVATE_KEY" ]; then
    echo "❌ Error: Private key is required"
    echo "Provide it via --private-key, or set privateKey in config file"
    echo "Use --help for more information"
    exit 1
fi

# Check if RPC URL is provided
if [ -z "$RPC_URL" ]; then
    echo "❌ Error: RPC URL is required"
    echo "Provide it via --rpc-url, or set rpcUrl in config file"
    echo "Use --help for more information"
    exit 1
fi

echo "🚀 Deploying Pelagos & AppChain..."
echo "RPC URL: $RPC_URL"
echo "App Chain ID: $APP_CHAIN_ID"
echo ""

# Check if Foundry is available
if ! command -v forge &> /dev/null; then
    echo "❌ Error: Foundry (forge) is required. Install it first:"
    echo "   curl -L https://foundry.paradigm.xyz | bash"
    echo "   foundryup"
    exit 1
fi

# Clean and compile contracts
echo "📦 Compiling contracts..."
forge clean

# Step 1: Deploy Pelagos contract
echo ""
echo "📦 Step 1: Deploying Pelagos contract..."

# Build forge command
FORGE_CMD="forge create --private-key \"$PRIVATE_KEY\" --rpc-url \"$RPC_URL\" --broadcast --force"
if [ "$VERIFY" = true ]; then
    FORGE_CMD="$FORGE_CMD --verify"
    echo "🔍 Contract verification enabled"
fi
FORGE_CMD="$FORGE_CMD pelacli/Pelagos.sol:Pelagos"

# Execute the command
DEPLOY_OUTPUT=$(eval "$FORGE_CMD")

# Extract contract address from output
PELAGOS_ADDRESS=$(echo "$DEPLOY_OUTPUT" | grep -o "Deployed to: 0x[0-9a-fA-F]*" | cut -d' ' -f3)

if [ -z "$PELAGOS_ADDRESS" ]; then
    # Fallback: look for any Ethereum address (but skip the first one which is usually the sender)
    PELAGOS_ADDRESS=$(echo "$DEPLOY_OUTPUT" | grep -o "0x[0-9a-fA-F]\{40\}" | sed -n '2p')
fi

if [ -z "$PELAGOS_ADDRESS" ]; then
    echo "❌ Pelagos deployment failed!"
    echo "Output: $DEPLOY_OUTPUT"
    exit 1
fi

echo ""
echo "✅ Pelagos deployed successfully!"
echo "📋 Pelagos Address: $PELAGOS_ADDRESS"
echo ""

# Step 2: Deploy AppChain contract
echo "📦 Step 2: Deploying AppChain contract..."

# Build forge command
FORGE_CMD="forge create --private-key \"$PRIVATE_KEY\" --rpc-url \"$RPC_URL\" --broadcast --force"
if [ "$VERIFY" = true ]; then
    FORGE_CMD="$FORGE_CMD --verify"
fi
FORGE_CMD="$FORGE_CMD pelacli/AppChain.sol:AppChain --constructor-args \"$PELAGOS_ADDRESS\""

# Execute the command
DEPLOY_OUTPUT=$(eval "$FORGE_CMD")

# Extract contract address from output
APPCHAIN_ADDRESS=$(echo "$DEPLOY_OUTPUT" | grep -o "Deployed to: 0x[0-9a-fA-F]*" | cut -d' ' -f3)

if [ -z "$APPCHAIN_ADDRESS" ]; then
    # Fallback: look for any Ethereum address (but skip the first one which is usually the sender)
    APPCHAIN_ADDRESS=$(echo "$DEPLOY_OUTPUT" | grep -o "0x[0-9a-fA-F]\{40\}" | sed -n '2p')
fi

if [ -z "$APPCHAIN_ADDRESS" ]; then
    echo "❌ AppChain deployment failed!"
    echo "Output: $DEPLOY_OUTPUT"
    exit 1
fi

echo ""
echo "✅ AppChain deployed successfully!"
echo "📋 AppChain Address: $APPCHAIN_ADDRESS"
echo ""

# Step 3: Register AppChain in Pelagos
echo "📦 Step 3: Registering AppChain in Pelagos..."
cast send "$PELAGOS_ADDRESS" \
    "registerAppchainContract(uint256,address)" \
    "$APP_CHAIN_ID" \
    "$APPCHAIN_ADDRESS" \
    --rpc-url "$RPC_URL" \
    --private-key "$PRIVATE_KEY"

if [ $? -ne 0 ]; then
    echo "❌ AppChain registration failed"
    exit 1
fi

echo "✅ AppChain registered successfully"
echo ""

# Step 4: Verify registration
echo "📦 Step 4: Verifying registration..."
REGISTERED_ADDRESS=$(cast call "$PELAGOS_ADDRESS" \
    "appchainContracts(uint256)(address)" \
    "$APP_CHAIN_ID" \
    --rpc-url "$RPC_URL")

echo "Registered contract: $REGISTERED_ADDRESS"
echo "Expected contract:   $APPCHAIN_ADDRESS"

if [ "$REGISTERED_ADDRESS" = "$APPCHAIN_ADDRESS" ]; then
    echo "✅ Registration verified!"
else
    echo "❌ Registration verification failed"
    exit 1
fi

if [ "$VERIFY" = true ]; then
    echo ""
    echo "🔍 Contract verification: Enabled"
    echo "   💡 Note: For verification, etherscanApiKey must be set in config file"
else
    echo ""
    echo "🔍 Contract verification: Disabled (use without --no-verify to enable)"
fi

echo ""
echo "🎉 Deployment Complete!"
echo "============================================="
echo "Contract Addresses:"
echo "  Pelagos:  $PELAGOS_ADDRESS"
echo "  AppChain: $APPCHAIN_ADDRESS"
echo ""
echo "Deployment Summary:"
echo "  1. ✅ Pelagos registry contract deployed"
echo "  2. ✅ AppChain contract deployed"
echo "  3. ✅ AppChain registered on Pelagos (chain ID: $APP_CHAIN_ID)"

