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
        string token,
        uint256 indexed sourceChainId,
        bytes32 indexed txHash
    );

    // State variables
    mapping(address => mapping(string => uint256)) public tokenBalances; // user => token => balance
    address public pelagosContract;
    uint256 public totalMints;

    modifier onlyPelagos() {
        require(msg.sender == pelagosContract, "AppChain: caller is not Pelagos contract");
        _;
    }

    constructor(address _pelagosContract) {
        pelagosContract = _pelagosContract;
    }

    /**
     * @dev Process external transaction from Pelagos contract
     * @param sourceChainId The chain ID where the transaction originated
     * @param txHash The transaction hash for tracking
     * @param payload The encoded transaction data
     */
    function processExternalTransaction(
        uint256 sourceChainId,
        bytes32 txHash,
        bytes calldata payload
    ) external onlyPelagos {
        require(payload.length > 0, "AppChain: empty payload");
        
        // Simplified: Only handle token minting
        _processTokenMint(sourceChainId, txHash, payload);
    }

    /**
     * @dev Process token mint operation
     * Payload format: [recipient:20bytes][amount:32bytes][tokenName:variable]
     */
    function _processTokenMint(
        uint256 sourceChainId,
        bytes32 txHash,
        bytes calldata payload
    ) internal {
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
        totalMints++;
        
        emit TokenMinted(recipient, amount, tokenName, sourceChainId, txHash);
    }

    /**
     * @dev Get user's token balance for a specific token
     * @param user The user address
     * @param token The token name
     */
    function getTokenBalance(address user, string calldata token) external view returns (uint256) {
        return tokenBalances[user][token];
    }

    /**
     * @dev Update Pelagos contract address (for upgrades)
     */
    function updatePelagosContract(address _pelagosContract) external {
        require(msg.sender == pelagosContract, "AppChain: only current Pelagos can update");
        pelagosContract = _pelagosContract;
    }
}
