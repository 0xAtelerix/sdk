// SPDX-License-Identifier: MIT
pragma solidity ^0.7.6;

/**
 * @title Example
 * @notice DEMO CONTRACT - FOR TESTING AND EXAMPLE PURPOSES ONLY
 * @dev This is a simple demonstration contract showing cross-chain bridge and swap events.
 *      DO NOT USE IN PRODUCTION. Real contracts would include proper business logic,
 *      validation, access controls, and security measures.
 */
contract Example {
    // Event emitted when a user deposits tokens (for cross-chain bridge)
    event Deposit(address indexed user, string token, uint256 amount);

    // Event emitted when a user initiates a cross-chain swap
    event Swap(address indexed user, string tokenIn, string tokenOut, uint256 amountIn);

    event WithdrawToSolana(uint256 amount);

    // Deposit function with token specification
    function deposit(string memory token, uint256 amount) external {
        require(amount > 0, "Must deposit some amount");
        require(bytes(token).length > 0, "Token name required");

        // This is just for demo, Real contract will have business login to verify business logic
        // Emit deposit event that will be caught by the appchain
        emit Deposit(msg.sender, token, amount);
    }

    // Cross-chain swap function
    function swap(string memory tokenIn, string memory tokenOut, uint256 amountIn) external {
        require(amountIn > 0, "Must swap some amount");
        require(bytes(tokenIn).length > 0, "Input token name required");
        require(bytes(tokenOut).length > 0, "Output token name required");
        require(keccak256(bytes(tokenIn)) != keccak256(bytes(tokenOut)), "Cannot swap same token");

        // Emit swap event that will trigger cross-chain transfer
        emit Swap(msg.sender, tokenIn, tokenOut, amountIn);
    }

    function withdrawToSolana(uint256 amount) external {
        require(amount > 0, "Mist withdraw some amount");
    
        emit WithdrawToSolana(amount);
    }
}
