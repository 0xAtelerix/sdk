// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

/**
 * @title AppchainReceiver
 * @dev Example contract that receives and decodes data from Pelagos contract
 * This demonstrates how appchain developers can decode the same data they encoded
 */
contract AppchainReceiver {
    
    // Events to track operations
    event TokenTransfer(address indexed to, uint256 amount, bytes32 txHash);
    event SwapExecuted(address indexed tokenIn, address indexed tokenOut, uint256 amountIn, uint256 amountOut);
    event CustomOperation(string action, bytes data);
    
    // Mock token balances for demonstration
    mapping(address => uint256) public balances;
    
    // Address of the Pelagos contract (only it can call this contract)
    address public pelagosContract;
    
    modifier onlyPelagos() {
        require(msg.sender == pelagosContract, "Only Pelagos contract can call");
        _;
    }
    
    constructor(address _pelagosContract) {
        pelagosContract = _pelagosContract;
    }
    
    /**
     * @dev Main function called by Pelagos contract with decoded data
     * @param data The encoded payload from appchain (same format as encoded)
     */
    function processAppchainData(bytes calldata data) external onlyPelagos {
        // Decode the operation type first
        (string memory action) = abi.decode(data, (string));
        
        if (keccak256(bytes(action)) == keccak256(bytes("transfer"))) {
            _handleTransfer(data);
        } else if (keccak256(bytes(action)) == keccak256(bytes("swap"))) {
            _handleSwap(data);
        } else {
            _handleCustomOperation(action, data);
        }
    }
    
    /**
     * @dev Handle token transfer operation
     * Decodes: (string action, address to, uint256 amount, bytes32 txHash)
     */
    function _handleTransfer(bytes calldata data) internal {
        (string memory action, address to, uint256 amount, bytes32 txHash) = 
            abi.decode(data, (string, address, uint256, bytes32));
        
        // Execute the transfer logic
        balances[to] += amount;
        
        emit TokenTransfer(to, amount, txHash);
    }
    
    /**
     * @dev Handle DeFi swap operation  
     * Decodes: (string action, address tokenIn, address tokenOut, uint256 amountIn, uint256 minAmountOut, uint256 deadline)
     */
    function _handleSwap(bytes calldata data) internal {
        (string memory action, address tokenIn, address tokenOut, uint256 amountIn, uint256 minAmountOut, uint256 deadline) = 
            abi.decode(data, (string, address, uint256, address, uint256, uint256));
        
        // Mock swap logic - in reality this would interact with DEX
        require(block.timestamp <= deadline, "Swap deadline exceeded");
        
        uint256 amountOut = (amountIn * 995) / 1000; // 0.5% fee
        require(amountOut >= minAmountOut, "Insufficient output amount");
        
        // Execute swap
        balances[tokenIn] -= amountIn;  // Remove input tokens
        balances[tokenOut] += amountOut; // Add output tokens
        
        emit SwapExecuted(tokenIn, tokenOut, amountIn, amountOut);
    }
    
    /**
     * @dev Handle custom operations
     */
    function _handleCustomOperation(string memory action, bytes calldata data) internal {
        emit CustomOperation(action, data);
    }
    
    // View functions for testing
    function getBalance(address token) external view returns (uint256) {
        return balances[token];
    }
}
