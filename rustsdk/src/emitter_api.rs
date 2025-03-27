//! Module implementing the gRPC Emitter API service.
//!
//! This version uses dummy implementations for DB operations using `libmdbx-rs`.
//! It returns simulated data for checkpoints and external transactions.
//!
//! Note: In a production implementation, you would replace these dummy functions
//! with actual MDBX transaction logic.

use async_trait::async_trait;
use libmdbx::{Database, NoWriteMap, Error};
use std::collections::BTreeMap;
use std::sync::Arc;
use tokio::sync::Mutex;
use tonic::{Request, Response, Status};

use crate::proto::{
    emitter_server::Emitter,
    CheckpointResponse,
    GetCheckpointsRequest,
    GetExternalTransactionsRequest,
    GetExternalTransactionsResponse,
    ExternalTransaction as ProtoExternalTransaction,
    GetChainIdResponse,
};
use crate::types::Checkpoint;

// Type alias for our DB; we use NoWriteMap mode as in the examples.
type DB = Database<NoWriteMap>;

/// AppchainEmitterService is our gRPC service backed by a (dummy) MDBX database.
#[derive(Debug)]
pub struct AppchainEmitterService {
    /// Shared MDBX database.
    pub db: Arc<Mutex<DB>>,
    /// Chain identifier.
    pub chain_id: u64,
}

impl AppchainEmitterService {
    /// Constructs a new AppchainEmitterService.
    pub fn new(db: Arc<Mutex<DB>>, chain_id: u64) -> Self {
        Self { db, chain_id }
    }

    /// Helper to convert an internal Checkpoint into its gRPC proto representation.
    fn checkpoint_to_proto(cp: &Checkpoint) -> crate::proto::checkpoint_response::Checkpoint {
        crate::proto::checkpoint_response::Checkpoint {
            latest_block_number: cp.block_number,
            state_root: cp.state_root.to_vec(),
            block_hash: cp.block_hash.to_vec(),
            external_tx_root_hash: cp.external_transactions_root.to_vec(),
        }
    }

    /// Dummy implementation: reads checkpoints from the DB.
    ///
    /// In a real implementation, this would open a read-only transaction,
    /// access the table named by `CHECKPOINT_BUCKET`, deserialize stored JSON (or CBOR)
    /// into a `Checkpoint`, etc.
    fn read_checkpoints(
        &self,
        start_block: u64,
        limit: u32,
    ) -> Result<Vec<Checkpoint>, Error> {
        // Dummy: return a vector of checkpoints with sequential block numbers.
        let mut checkpoints = Vec::new();
        for bn in start_block..(start_block + limit as u64) {
            checkpoints.push(Checkpoint {
                chain_id: self.chain_id,
                block_number: bn,
                block_hash: [0u8; 32],
                state_root: [0u8; 32],
                external_transactions_root: [0u8; 32],
            });
        }
        Ok(checkpoints)
    }

    /// Dummy implementation: reads external transactions from the DB.
    ///
    /// In a real implementation, this would open a transaction, read from the table
    /// named by `EXTERNAL_TX_BUCKET`, deserialize the data, and group transactions
    /// by block number.
    fn read_external_transactions(
        &self,
        start_block: u64,
        limit: u32,
    ) -> Result<BTreeMap<u64, Vec<ProtoExternalTransaction>>, Error> {
        let mut tx_map = BTreeMap::new();
        for bn in start_block..(start_block + limit as u64) {
            let tx = ProtoExternalTransaction {
                chain_id: self.chain_id,
                tx: vec![], // dummy empty transaction data
            };
            tx_map.insert(bn, vec![tx]);
        }
        Ok(tx_map)
    }
}

#[async_trait]
impl Emitter for AppchainEmitterService {
    async fn get_checkpoints(
        &self,
        request: Request<GetCheckpointsRequest>,
    ) -> Result<Response<CheckpointResponse>, Status> {
        let req = request.into_inner();
        let start_block = req.latest_previous_checkpoint_block_number;
        let limit = req.limit.unwrap_or(10);

        // In a real scenario, you'd acquire a read transaction on the DB.
        // Here we use the dummy implementation.
        let checkpoints = self
            .read_checkpoints(start_block, limit)
            .map_err(|e| Status::internal(format!("DB error: {:?}", e)))?;

        let proto_cps = checkpoints
            .iter()
            .map(Self::checkpoint_to_proto)
            .collect();

        Ok(Response::new(CheckpointResponse { checkpoints: proto_cps }))
    }

    async fn create_internal_transactions_batch(
        &self,
        _request: Request<()>,
    ) -> Result<Response<crate::proto::CreateInternalTransactionsBatchResponse>, Status> {
        // Dummy implementation: return an empty batch hash and empty internal transactions.
        Ok(Response::new(crate::proto::CreateInternalTransactionsBatchResponse {
            batch_hash: vec![],
            internal_transactions: vec![],
        }))
    }

    async fn get_external_transactions(
        &self,
        request: Request<GetExternalTransactionsRequest>,
    ) -> Result<Response<GetExternalTransactionsResponse>, Status> {
        let req = request.into_inner();
        let start_block = req.latest_previous_block_number;
        let limit = req.limit.unwrap_or(10);

        let tx_map = self
            .read_external_transactions(start_block, limit)
            .map_err(|e| Status::internal(format!("DB error: {:?}", e)))?;

        let blocks = tx_map
            .into_iter()
            .map(|(block_number, txs)| crate::proto::get_external_transactions_response::BlockTransactions {
                block_number,
                transactions_root_hash: b"dummy_tx_hash".to_vec(), // dummy hash
                external_transactions: txs,
            })
            .collect();

        Ok(Response::new(GetExternalTransactionsResponse { blocks }))
    }

    async fn get_chain_id(
        &self,
        _request: Request<()>,
    ) -> Result<Response<GetChainIdResponse>, Status> {
        Ok(Response::new(GetChainIdResponse { chain_id: self.chain_id }))
    }
}
