//! Module implementing state transition logic.
//!
//! This module defines traits for state transitions and provides a default batch processor
//! that processes both external blocks and individual transactions, returning a combined list
//! of external transactions.

use crate::types::{AppTransaction, Batch, ExternalBlock, ExternalTransaction};
use thiserror::Error;

/// Custom error type for state transition operations.
#[derive(Debug, Error)]
pub enum StateTransitionError {
    #[error("Error processing transaction: {0}")]
    TransactionError(String),
    #[error("Error processing block: {0}")]
    BlockError(String),
}

/// Trait defining a simplified state transition interface.
/// Implementers provide methods to process individual transactions and external blocks.
pub trait StateTransitionSimplified<T: AppTransaction> {
    /// Processes a single transaction and returns a list of external transactions.
    fn process_tx(&self, tx: &T) -> Result<Vec<ExternalTransaction>, StateTransitionError>;
    /// Processes an external block and returns a list of external transactions.
    fn process_block(&self, block: &ExternalBlock) -> Result<Vec<ExternalTransaction>, StateTransitionError>;
}

/// Trait defining the state transition interface for processing a batch.
pub trait StateTransitionInterface<T: AppTransaction> {
    /// Processes a batch containing transactions and external blocks,
    /// returning a combined list of external transactions.
    fn process_batch(&self, batch: &Batch<T>) -> Result<Vec<ExternalTransaction>, StateTransitionError>;
}

/// A default batch processor that uses a simplified state transition implementation
/// to process a batch.
pub struct BatchProcessor<S> {
    pub simplified: S,
}

impl<S, T> StateTransitionInterface<T> for BatchProcessor<S>
where
    S: StateTransitionSimplified<T>,
    T: AppTransaction,
{
    fn process_batch(&self, batch: &Batch<T>) -> Result<Vec<ExternalTransaction>, StateTransitionError> {
        let mut ext_txs = Vec::new();

        for external_block in &batch.external_blocks {
            let mut txs = self.simplified.process_block(external_block)?;
            ext_txs.append(&mut txs);
        }

        for tx in &batch.transactions {
            let mut txs = self.simplified.process_tx(tx)?;
            ext_txs.append(&mut txs);
        }

        Ok(ext_txs)
    }
}
