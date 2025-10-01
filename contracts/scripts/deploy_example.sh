#!/bin/bash

# Example Contract Deployment Script (Chain-Agnostic)
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${GREEN}🚀 Example Contract Deployment${NC}"
echo "============================================="

# Navigate to contracts root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONTRACTS_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$CONTRACTS_ROOT"

# Check environment file
if [ ! -f ".env" ]; then
    echo -e "${RED}❌ Error: .env file not found${NC}"
    echo "Copy .env.example to .env and fill in your values"
    exit 1
fi

# Load environment variables
source .env

# Validate required environment variables
if [ -z "$RPC_URL" ] || [ -z "$PRIVATE_KEY" ]; then
    echo -e "${RED}❌ Error: Missing required environment variables${NC}"
    echo "Required: RPC_URL, PRIVATE_KEY"
    exit 1
fi

# Optional block explorer verification (Etherscan, etc.)
VERIFY_FLAG=""
if [ -n "$EXPLORER_API_KEY" ]; then
    VERIFY_FLAG="--verify --etherscan-api-key $EXPLORER_API_KEY"
fi

echo -e "${YELLOW}📋 Configuration:${NC}"
echo "RPC URL: $RPC_URL"
echo "Contracts Root: $CONTRACTS_ROOT"
echo ""

# Deploy Example
echo -e "${BLUE}📦 Deploying Example contract...${NC}"
EXAMPLE_ADDRESS=$(forge create example/Example.sol:Example \
    --rpc-url "$RPC_URL" \
    --private-key "$PRIVATE_KEY" \
    $VERIFY_FLAG \
    --json | jq -r '.deployedTo')

if [ $? -ne 0 ] || [ -z "$EXAMPLE_ADDRESS" ]; then
    echo -e "${RED}❌ Example deployment failed${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Example deployed at: $EXAMPLE_ADDRESS${NC}"
echo ""
echo -e "${GREEN}🎉 Deployment Complete!${NC}"
echo "============================================="
echo -e "${YELLOW}Contract Address:${NC}"
echo "Example: $EXAMPLE_ADDRESS"
echo ""
echo -e "${YELLOW}Next Steps:${NC}"
echo "1. Update your application config with this address"
echo "2. Test deposit: cast send $EXAMPLE_ADDRESS 'deposit(string,uint256)' 'ETH' 1000000000000000000 --rpc-url $RPC_URL --private-key $PRIVATE_KEY"
echo "3. Test swap: cast send $EXAMPLE_ADDRESS 'swap(string,string,uint256)' 'ETH' 'USDC' 1000000000000000000 --rpc-url $RPC_URL --private-key $PRIVATE_KEY"
echo ""
