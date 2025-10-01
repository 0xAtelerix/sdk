#!/bin/bash

# Example Contract Deployment Script
# Usage: ./deploy_example.sh [options]
# Options:
#   --private-key KEY   Private key for deployment
#   --rpc-url URL      RPC URL
#   --no-verify        Skip contract verification
#   --env-file FILE    Path to .env file (default: .env)

set -e

# Default values
PRIVATE_KEY=""
RPC_URL=""
VERIFY=true
ENV_FILE=".env"

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
            echo "  --env-file FILE    Path to .env file (default: .env)"
            echo "  --help             Show this help message"
            echo ""
            echo "Environment Variables (.env file):"
            echo "  PRIVATE_KEY=0xyour_private_key"
            echo "  RPC_URL=https://eth-sepolia.g.alchemy.com/v2/..."
            echo "  ETHERSCAN_API_KEY=your_etherscan_api_key"
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
