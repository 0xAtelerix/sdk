//! Module defining the database bucket names and keys.

/// Bucket for storing checkpoints.
pub const CHECKPOINT_BUCKET: &str = "checkpoints";

/// Bucket for storing external transactions.
pub const EXTERNAL_TX_BUCKET: &str = "external_transactions";

/// Bucket for storing blocks.
pub const BLOCKS_BUCKET: &str = "blocks";

/// Bucket for storing configuration data (e.g., last block number, hash, chainID, etc.).
pub const CONFIG_BUCKET: &str = "config";

/// Key for the last block stored in the DB.
pub const LAST_BLOCK_KEY: &str = "last_block";

/// Bucket for storing state (if needed).
pub const STATE_BUCKET: &str = "state";

/// Optional transaction pool persistence bucket.
pub const TX_POOL: &str = "tx_pool";
