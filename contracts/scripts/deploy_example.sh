#!/bin/bash

# Example Contract Deployment Script
# Usage: ./deploy_example.sh [options]
# Options:
#   --private-key KEY   Private key for deployment
#   --rpc-url URL      RPC URL
#   --no-verify        Skip contract verification
#   --config-file FILE Path to config.json file (default: config.json)

set -e

# Default values
PRIVATE_KEY=""
RPC_URL=""
VERIFY=true
CONFIG_FILE="config/config.json"

# Load config.json file if it exists
load_config_json() {
    local config_file="$1"
    if [ -f "$config_file" ]; then
        echo "📄 Loading configuration from $config_file"
        if command -v jq >/dev/null 2>&1; then
            RPC_URL=$(jq -r '.rpcUrl // empty' "$config_file")
            PRIVATE_KEY=$(jq -r '.privateKey // empty' "$config_file")
            export ETHERSCAN_API_KEY=$(jq -r '.etherscanApiKey // empty' "$config_file")
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
        --no-verify)
            VERIFY=false
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [options]"
            echo "Options:"
            echo "  --private-key KEY   Private key for deployment"
            echo "  --rpc-url URL      RPC URL"
            echo "  --no-verify        Skip contract verification"
            echo "  --config-file FILE Path to config.json file (default: config/config.json)"
            echo "  --help             Show this help message"
            echo ""
            echo "Configuration (JSON config file):"
            echo "  {"
            echo "    \"rpcUrl\": \"https://eth-sepolia.g.alchemy.com/v2/...\","
            echo "    \"privateKey\": \"0xyour_private_key\","
            echo "    \"etherscanApiKey\": \"your_etherscan_api_key\""
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

echo "🚀 Deploying Example Contract..."
echo "RPC URL: $RPC_URL"
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

# Build forge command
FORGE_CMD="forge create --private-key \"$PRIVATE_KEY\" --rpc-url \"$RPC_URL\" --broadcast --force"
if [ "$VERIFY" = true ]; then
    FORGE_CMD="$FORGE_CMD --verify"
    echo "🔍 Contract verification enabled"
fi
FORGE_CMD="$FORGE_CMD example/Example.sol:Example"

# Execute the command
DEPLOY_OUTPUT=$(eval "$FORGE_CMD")

# Extract contract address from output
EXAMPLE_ADDRESS=$(echo "$DEPLOY_OUTPUT" | grep -o "Deployed to: 0x[0-9a-fA-F]*" | cut -d' ' -f3)

if [ -z "$EXAMPLE_ADDRESS" ]; then
    # Fallback: look for any Ethereum address (but skip the first one which is usually the sender)
    EXAMPLE_ADDRESS=$(echo "$DEPLOY_OUTPUT" | grep -o "0x[0-9a-fA-F]\{40\}" | sed -n '2p')
fi

if [ -z "$EXAMPLE_ADDRESS" ]; then
    echo "❌ Deployment failed!"
    echo "Output: $DEPLOY_OUTPUT"
    exit 1
fi

echo ""
echo "✅ Contract deployed successfully!"
echo "📋 Contract Address: $EXAMPLE_ADDRESS"

if [ "$VERIFY" = true ]; then
    echo ""
    echo "🔍 Contract verification: Enabled"
    echo "   💡 Note: Verification may take a few minutes to appear on the block explorer"
fi

echo ""
echo "📝 Next steps:"
echo "   • Update your application config with: $EXAMPLE_ADDRESS"
