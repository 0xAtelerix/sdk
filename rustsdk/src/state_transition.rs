use crate::{Batch, ExternalTransaction};
use mdbx::RwTransaction;
use serde::{de::DeserializeOwned, Serialize};

pub trait StateTransition {
    type Transaction;
    
    fn process_batch(
        &self,
        batch: Batch<Self::Transaction>,
        tx: &mut RwTransaction,
    ) -> Result<Vec<ExternalTransaction>, String>;
}

pub struct DefaultStateProcessor<T> {
    root_calculator: Box<dyn RootCalculator>,
    block_builder: Box<dyn BlockBuilder<T>>,
}

impl<T> StateTransition for DefaultStateProcessor<T> 
where
    T: Serialize + DeserializeOwned,
{
    type Transaction = T;

    fn process_batch(
        &self,
        batch: Batch<T>,
        tx: &mut RwTransaction,
    ) -> Result<Vec<ExternalTransaction>, String> {
        let mut ext_txs = Vec::new();
        
        // Process external blocks
        for block in batch.external_blocks {
            let txs = self.process_external_block(block, tx)?;
            ext_txs.extend(txs);
        }
        
        // Process transactions
        for app_tx in batch.transactions {
            let txs = self.process_transaction(app_tx, tx)?;
            ext_txs.extend(txs);
        }
        
        Ok(ext_txs)
    }
}

// Helper traits
pub trait RootCalculator {
    fn calculate_root(&self, tx: &RwTransaction) -> Result<[u8; 32], String>;
}

pub trait BlockBuilder<T> {
    fn build_block(
        &self,
        number: u64,
        state_root: [u8; 32],
        prev_hash: [u8; 32],
        txs: Batch<T>,
    ) -> Result<Vec<u8>, String>;
}