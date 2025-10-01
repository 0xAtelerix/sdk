#!/bin/bash

# Pelagos & AppChain Deployment Script
# Usage: ./deploy_pelagos.sh [options]
# Options:
#   --private-key KEY       Private key for deployment
#   --rpc-url URL          RPC URL
#   --no-verify            Skip contract verification
#   --env-file FILE        Path to .env file (default: .env)
#   --source-chain-id ID   Source chain ID (default: 42)

set -e

# Default values
PRIVATE_KEY=""
RPC_URL=""
VERIFY=true
ENV_FILE=".env"
SOURCE_CHAIN_ID=42

# Load .env file if it exists
load_env_file() {
    local env_file="$1"
    if [ -f "$env_file" ]; then
        echo "📄 Loading environment from $env_file"
        while IFS='=' read -r key value; do
            # Skip empty lines and comments
            [[ -z "$key" || "$key" =~ ^[[:space:]]*# ]] && continue
            # Remove quotes from value if present
            value=$(echo "$value" | sed 's/^"\(.*\)"$/\1/' | sed "s/^'\(.*\)'$/\1/")
            # Set our script variables if they match
            case $key in
                RPC_URL) RPC_URL="$value" ;;
                PRIVATE_KEY) PRIVATE_KEY="$value" ;;
                ETHERSCAN_API_KEY) export ETHERSCAN_API_KEY="$value" ;;
                SOURCE_CHAIN_ID) SOURCE_CHAIN_ID="$value" ;;
            esac
        done < "$env_file"
        echo "✅ Environment loaded"
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
        --env-file)
            ENV_FILE="$2"
            shift 2
            ;;
        --source-chain-id)
            SOURCE_CHAIN_ID="$2"
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
            echo "  --env-file FILE        Path to .env file (default: .env)"
            echo "  --source-chain-id ID   Source chain ID for registration (default: 42)"
            echo "  --help                 Show this help message"
            echo ""
            echo "Environment Variables (.env file):"
            echo "  PRIVATE_KEY=0xyour_private_key"
            echo "  RPC_URL=https://eth-sepolia.g.alchemy.com/v2/..."
            echo "  ETHERSCAN_API_KEY=your_etherscan_api_key"
            echo "  SOURCE_CHAIN_ID=42"
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

# Load environment file
load_env_file "$ENV_FILE"

# Check if private key is provided
if [ -z "$PRIVATE_KEY" ]; then
    echo "❌ Error: Private key is required"
    echo "Provide it via --private-key, or set PRIVATE_KEY in .env file"
    echo "Use --help for more information"
    exit 1
fi

# Check if RPC URL is provided
if [ -z "$RPC_URL" ]; then
    echo "❌ Error: RPC URL is required"
    echo "Provide it via --rpc-url, or set RPC_URL in .env file"
    echo "Use --help for more information"
    exit 1
fi

echo "🚀 Deploying Pelagos & AppChain..."
echo "RPC URL: $RPC_URL"
echo "Source Chain ID: $SOURCE_CHAIN_ID"
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
    "$SOURCE_CHAIN_ID" \
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
    "$SOURCE_CHAIN_ID" \
    --rpc-url "$RPC_URL")

echo "Registered contract: $REGISTERED_ADDRESS"
echo "Expected contract:   $APPCHAIN_ADDRESS"

if [ "$REGISTERED_ADDRESS" = "$APPCHAIN_ADDRESS" ]; then
    echo "✅ Registration verified!"
else
    echo "❌ Registration verification failed"
    exit 1
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
echo "  3. ✅ AppChain registered on Pelagos (chain ID: $SOURCE_CHAIN_ID)"

if [ "$VERIFY" = true ]; then
    echo ""
    echo "🔍 Contract verification: Enabled"
    echo "   💡 Note: For verification, ETHERSCAN_API_KEY must be set in .env"
else
    echo ""
    echo "🔍 Contract verification: Disabled (use without --no-verify to enable)"
fi

echo ""
echo "📝 Next steps:"
echo "   • Update AppchainContractAddress in your application config"
echo "   • Example: const AppchainContractAddress = \"$APPCHAIN_ADDRESS\""

