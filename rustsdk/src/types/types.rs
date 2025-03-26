//! Module defining shared types for the appchain.
//!
//! This module includes definitions for application transactions, batches, external blocks,
//! appchain blocks, checkpoints, and a state root calculator trait.

use serde::{Serialize, Deserialize};
use std::fmt::Debug;

/// A marker trait for application transactions.
/// All types used as appchain transactions must implement Serialize, Deserialize, and Debug.
pub trait AppTransaction: Serialize + for<'de> Deserialize<'de> + Debug + bincode::Decode<()> + bincode::Encode {}

impl<T> AppTransaction for T where T: Serialize + for<'de> Deserialize<'de> + Debug {}

/// A batch containing both transactions and external blocks.
#[derive(Debug, Serialize, Deserialize, Clone)]
#[serde(bound(deserialize = "T: for<'d> Deserialize<'d>"))]
pub struct Batch<T>
where
    T: AppTransaction,
{
    /// List of internal transactions.
    pub transactions: Vec<T>,
    /// List of external blocks.
    pub external_blocks: Vec<ExternalBlock>,
}

/// An external block structure.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct ExternalBlock {
    /// Chain ID of the external block.
    pub chain_id: u64,
    /// Block number of the external block.
    pub block_number: u64,
    /// Block hash (32 bytes).
    pub block_hash: [u8; 32],
}

/// Trait for appchain blocks.
/// An appchain block must be serializable and provide methods to retrieve key block attributes.
pub trait AppchainBlock: Serialize + for<'de> Deserialize<'de> + Debug {
    /// Returns the block number.
    fn number(&self) -> u64;
    /// Returns the block hash.
    fn hash(&self) -> [u8; 32];
    /// Returns the state root.
    fn state_root(&self) -> [u8; 32];
    /// Returns a serialized byte representation of the block.
    fn bytes(&self) -> Vec<u8>;
}

/// A checkpoint represents a finalized state of the appchain.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Checkpoint {
    /// Chain identifier.
    pub chain_id: u64,
    /// Block number at which the checkpoint was made.
    pub block_number: u64,
    /// Hash of the block.
    pub block_hash: [u8; 32],
    /// The state root at the checkpoint.
    pub state_root: [u8; 32],
    /// The root of external transactions at the checkpoint.
    pub external_transactions_root: [u8; 32],
}

/// Trait for calculating the state root.
/// This abstraction allows swapping out the state root calculation implementation.
pub trait RootCalculator {
    /// Calculates and returns the state root.
    fn state_root_calculator(&self) -> Result<[u8; 32], Box<dyn std::error::Error>>;
}

/// Trait defining a generic transaction pool interface.
pub trait TxPoolInterface<T: AppTransaction> {
    /// Adds a transaction to the pool.
    fn add_transaction(&self, hash: &str, tx: T) -> Result<(), Box<dyn std::error::Error>>;
    /// Retrieves a transaction by its hash.
    fn get_transaction(&self, hash: &str) -> Result<Option<T>, Box<dyn std::error::Error>>;
    /// Removes a transaction from the pool.
    fn remove_transaction(&self, hash: &str) -> Result<(), Box<dyn std::error::Error>>;
    /// Retrieves all transactions from the pool.
    fn get_all_transactions(&self) -> Result<Vec<T>, Box<dyn std::error::Error>>;
}

pub struct ExternalTransaction {
    pub chain_id: u64,
	pub tx:       Vec<u8>,
}
