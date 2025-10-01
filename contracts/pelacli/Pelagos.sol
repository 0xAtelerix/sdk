// SPDX-License-Identifier: MIT
pragma solidity ^0.7.6;

/**
 * @title Pelagos Contract
 * @notice DEMO CONTRACT - FOR TESTING AND EXAMPLE PURPOSES ONLY
 * @dev This is a demonstration contract showing the Atelerix cross-chain architecture.
 *      DO NOT USE IN PRODUCTION without proper security audits and modifications.
 *      Handles external transactions from appchains through consensus.
 */
contract Pelagos {

    // Event emitted when an external transaction is processed
    event ExternalTransactionProcessed(
        uint256 indexed fromChainID,
        uint256 indexed sequenceNumber,
        bytes32 indexed nonceHash,
        bytes payload
    );

    // Event emitted when ownership is transferred
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    // Event emitted when an appchain is registered
    event AppchainRegistered(uint256 indexed sourceChainID, address indexed appchainContract);

    // Event emitted when an appchain is unregistered
    event AppchainUnregistered(uint256 indexed sourceChainID);

    // Contract owner (should be set to the consensus system)
    address public owner;
    
    // Mapping to track processed transactions to prevent replay attacks
    mapping(bytes32 => bool) public processedTransactions;
    
    // Counter for total processed transactions
    uint256 public totalProcessedTransactions;
    
    // Mapping from source chain ID to appchain contract address
    mapping(uint256 => address) public appchainContracts;

    modifier onlyOwner() {
        require(msg.sender == owner, "Pelagos: caller is not the owner");
        _;
    }

    constructor() {
        owner = msg.sender;
        emit OwnershipTransferred(address(0), msg.sender);
    }

    /**
     * @dev Transfer ownership of the contract
     * @param newOwner The address of the new owner
     */
    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "Pelagos: new owner is the zero address");
        emit OwnershipTransferred(owner, newOwner);
        owner = newOwner;
    }

    /**
     * @dev Process an external transaction from an appchain
     * @param fromChainID The chain ID of the source appchain  
     * @param nonceHash The deterministic hash for replay protection (includes sourceChain, targetChain, block, payload, txIndex)
     * @param payload The encoded transaction payload
     */
    function processExternalTransaction(
        uint256 fromChainID,
        bytes32 nonceHash,
        bytes calldata payload
    ) external onlyOwner {
        require(fromChainID > 0, "Pelagos: invalid chain ID");
        require(nonceHash != bytes32(0), "Pelagos: invalid nonce hash");
        require(payload.length > 0, "Pelagos: empty payload");

        // Prevent replay attacks using deterministic nonce hash
        require(!processedTransactions[nonceHash], "Pelagos: transaction already processed");
        processedTransactions[nonceHash] = true;

        // Increment counter
        totalProcessedTransactions++;

        // Emit event for indexing and monitoring
        emit ExternalTransactionProcessed(fromChainID, totalProcessedTransactions, nonceHash, payload);

        // Route to appropriate appchain contract based on fromChainID
        _processPayload(fromChainID, nonceHash, payload);
    }

    /**
     * @dev Internal function to process the payload by routing to appropriate appchain contract
     * @param fromChainID The source chain ID
     * @param nonceHash The nonce hash
     * @param payload The transaction payload
     */
    function _processPayload(uint256 fromChainID, bytes32 nonceHash, bytes calldata payload) internal {
        address appchainContract = appchainContracts[fromChainID];
        
        if (appchainContract != address(0)) {
            // Route to specific appchain contract
            (bool success,) = appchainContract.call(
                abi.encodeWithSignature(
                    "processExternalTransaction(uint256,bytes32,bytes)",
                    fromChainID,
                    nonceHash,
                    payload
                )
            );
            
            require(success, "Pelagos: appchain contract call failed");
        } else {
            // No specific appchain contract registered, transaction is processed but not routed
            // This allows for future registration of appchain contracts
        }
    }

    /**
     * @dev Register an appchain contract for a specific source chain ID
     * @param sourceChainID The source chain ID to register
     * @param appchainContract The appchain contract address
     */
    function registerAppchainContract(uint256 sourceChainID, address appchainContract) external onlyOwner {
        require(sourceChainID > 0, "Pelagos: invalid source chain ID");
        require(appchainContract != address(0), "Pelagos: invalid appchain contract address");
        
        appchainContracts[sourceChainID] = appchainContract;
        emit AppchainRegistered(sourceChainID, appchainContract);
    }

    /**
     * @dev Unregister an appchain contract for a specific source chain ID
     * @param sourceChainID The source chain ID to unregister
     */
    function unregisterAppchainContract(uint256 sourceChainID) external onlyOwner {
        delete appchainContracts[sourceChainID];
        emit AppchainUnregistered(sourceChainID);
    }

}
