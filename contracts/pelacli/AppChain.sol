// SPDX-License-Identifier: MIT
pragma solidity ^0.7.6;

/**
 * @title AppChain Contract
 * @notice DEMO CONTRACT - FOR TESTING AND EXAMPLE PURPOSES ONLY
 * @dev This is a demonstration contract showing appchain integration with Atelerix.
 *      DO NOT USE IN PRODUCTION without proper security audits and modifications.
 *      Simple example showing cross-chain token minting/transfer.
 */
contract AppChain {
    // Event for token mint operation
    event TokenMinted(
        address indexed recipient,
        uint256 amount,
        string token
    );

    // State variables
    mapping(address => mapping(string => uint256)) public tokenBalances; // user => token => balance
    address public pelagosContract;
    address public owner;

    modifier onlyPelagos() {
        require(msg.sender == pelagosContract, "AppChain: caller is not Pelagos contract");
        _;
    }

    modifier onlyOwner() {
        require(msg.sender == owner, "AppChain: caller is not the owner");
        _;
    }

    constructor(address _pelagosContract) {
        pelagosContract = _pelagosContract;
        owner = msg.sender;
    }

    /**
     * @dev Execute transaction from Pelagos contract
     * @param payload The encoded transaction data
     */
    function executeTransaction(
        bytes calldata payload
    ) external onlyPelagos {
        require(payload.length > 0, "AppChain: empty payload");
        
        _processTokenMint(payload);
    }

    /**
     * @dev Process token mint operation
     * Payload format: [recipient:20bytes][amount:32bytes][tokenName:variable]
     */
    function _processTokenMint(bytes calldata payload) internal {
        require(payload.length >= 52, "AppChain: invalid token mint payload");
        
        // Extract recipient (bytes 0-19)
        address recipient;
        assembly {
            recipient := shr(96, calldataload(payload.offset))
        }
        
        // Extract amount (bytes 20-51)  
        uint256 amount;
        assembly {
            amount := calldataload(add(payload.offset, 20))
        }
        
        // Extract token name (remaining bytes)
        bytes memory tokenNameBytes = payload[52:];
        string memory tokenName = string(tokenNameBytes);
        
        // Mint tokens (update balance for specific token)
        tokenBalances[recipient][tokenName] += amount;
        
        emit TokenMinted(recipient, amount, tokenName);
    }

    /**
     * @dev Update Pelagos contract address (for upgrades)
     */
    function updatePelagosContract(address _pelagosContract) external onlyOwner {
        pelagosContract = _pelagosContract;
    }

    /**
     * @dev Transfer ownership to a new address
     */
    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "AppChain: new owner is the zero address");
        owner = newOwner;
    }
}
