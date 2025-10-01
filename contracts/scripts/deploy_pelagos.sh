#!/bin/bash

# Pelagos & AppChain Deployment Script (Chain-Agnostic)
# This script follows the deployment pattern:
# 1. Deploy Pelagos registry contract
# 2. Deploy AppChain contract
# 3. Register AppChain on Pelagos

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${GREEN}🚀 Pelagos & AppChain Deployment${NC}"
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

# Get deployer address
DEPLOYER_ADDRESS=$(cast wallet address --private-key "$PRIVATE_KEY")
echo -e "${BLUE}Deployer Address: $DEPLOYER_ADDRESS${NC}"
echo ""

# Step 1: Deploy Pelagos contract
echo -e "${BLUE}📦 Step 1: Deploying Pelagos contract...${NC}"
PELAGOS_ADDRESS=$(forge create pelacli/Pelagos.sol:Pelagos \
    --rpc-url "$RPC_URL" \
    --private-key "$PRIVATE_KEY" \
    $VERIFY_FLAG \
    --json | jq -r '.deployedTo')

if [ $? -ne 0 ] || [ -z "$PELAGOS_ADDRESS" ]; then
    echo -e "${RED}❌ Pelagos deployment failed${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Pelagos deployed at: $PELAGOS_ADDRESS${NC}"
echo ""

# Step 2: Deploy AppChain contract
echo -e "${BLUE}📦 Step 2: Deploying AppChain contract...${NC}"
APPCHAIN_ADDRESS=$(forge create pelacli/AppChain.sol:AppChain \
    --rpc-url "$RPC_URL" \
    --private-key "$PRIVATE_KEY" \
    --constructor-args "$PELAGOS_ADDRESS" \
    $VERIFY_FLAG \
    --json | jq -r '.deployedTo')

if [ $? -ne 0 ] || [ -z "$APPCHAIN_ADDRESS" ]; then
    echo -e "${RED}❌ AppChain deployment failed${NC}"
    exit 1
fi

echo -e "${GREEN}✅ AppChain deployed at: $APPCHAIN_ADDRESS${NC}"
echo ""

# Step 3: Register AppChain in Pelagos
echo -e "${BLUE}📦 Step 3: Registering AppChain in Pelagos (source chain ID: 42)...${NC}"
cast send "$PELAGOS_ADDRESS" \
    "registerAppchainContract(uint256,address)" \
    42 \
    "$APPCHAIN_ADDRESS" \
    --rpc-url "$RPC_URL" \
    --private-key "$PRIVATE_KEY"

if [ $? -ne 0 ]; then
    echo -e "${RED}❌ AppChain registration failed${NC}"
    exit 1
fi

echo -e "${GREEN}✅ AppChain registered successfully${NC}"
echo ""

# Step 4: Verify registration
echo -e "${BLUE}📦 Step 4: Verifying registration...${NC}"
REGISTERED_ADDRESS=$(cast call "$PELAGOS_ADDRESS" \
    "appchainContracts(uint256)(address)" \
    42 \
    --rpc-url "$RPC_URL")

echo "Registered contract: $REGISTERED_ADDRESS"
echo "Expected contract:   $APPCHAIN_ADDRESS"

if [ "$REGISTERED_ADDRESS" = "$APPCHAIN_ADDRESS" ]; then
    echo -e "${GREEN}✅ Registration verified!${NC}"
else
    echo -e "${RED}❌ Registration verification failed${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}🎉 Deployment Complete!${NC}"
echo "============================================="
echo -e "${YELLOW}Contract Addresses:${NC}"
echo "Pelagos:  $PELAGOS_ADDRESS"
echo "AppChain: $APPCHAIN_ADDRESS"
echo ""
echo -e "${YELLOW}Deployment Summary:${NC}"
echo "1. ✅ Pelagos registry contract deployed"
echo "2. ✅ AppChain contract deployed"
echo "3. ✅ AppChain registered on Pelagos (chain ID: 42)"
echo ""
echo -e "${YELLOW}Next Steps:${NC}"
echo "1. Update your application config with these addresses"
echo "2. Test the deployment with: cast call $PELAGOS_ADDRESS 'owner()(address)' --rpc-url $RPC_URL"
echo ""

echo -e "${GREEN}✅ Deployment completed successfully!${NC}"
echo ""

# Find the latest run file
LATEST_RUN=$(find broadcast/DeployComplete.s.sol/11155111/ -name "run-*.json" | sort | tail -1)

if [ ! -f "$LATEST_RUN" ]; then
    echo -e "${RED}❌ Could not find deployment artifacts${NC}"
    exit 1
fi

# Extract addresses using jq
PELAGOS_ADDRESS=$(jq -r '.transactions[] | select(.contractName == "Pelagos") | .contractAddress' "$LATEST_RUN" | head -1)
APPCHAIN_ADDRESS=$(jq -r '.transactions[] | select(.contractName == "AppChain") | .contractAddress' "$LATEST_RUN" | head -1)

# Convert to lowercase for consistency
PELAGOS_ADDRESS=$(echo "$PELAGOS_ADDRESS" | tr '[:upper:]' '[:lower:]')
APPCHAIN_ADDRESS=$(echo "$APPCHAIN_ADDRESS" | tr '[:upper:]' '[:lower:]')

if [ "$PELAGOS_ADDRESS" = "null" ] || [ "$APPCHAIN_ADDRESS" = "null" ]; then
    echo -e "${RED}❌ Could not extract contract addresses${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}🎉 Deployment Complete!${NC}"
echo "========================="
echo -e "${YELLOW}Contract Addresses:${NC}"
echo "Pelagos:  $PELAGOS_ADDRESS"
echo "AppChain: $APPCHAIN_ADDRESS"
echo ""
echo -e "${YELLOW}Etherscan Links:${NC}"
echo "Pelagos:  https://sepolia.etherscan.io/address/$PELAGOS_ADDRESS"
echo "AppChain: https://sepolia.etherscan.io/address/$APPCHAIN_ADDRESS"
echo ""
echo -e "${YELLOW}Next Steps:${NC}"
echo "1. Update your .env file with the contract addresses above"
echo "2. Run tests: cd .. && go test ./external -v"
echo "3. Test external transactions"
