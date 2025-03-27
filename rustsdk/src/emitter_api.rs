//! Module implementing the gRPC Emitter API service.
//!
//! This version uses dummy implementations for DB operations using `libmdbx-rs`.
//! It returns simulated data for checkpoints and external transactions.
//!
//! Note: In a production implementation, you would replace these dummy functions
//! with actual MDBX transaction logic.

use async_trait::async_trait;
use libmdbx::{Database, NoWriteMap};
use prost::Message as _;
use std::{collections::BTreeMap, sync::Arc};
use tokio::sync::Mutex;
use tonic::{Request, Response, Status};

use crate::{
    appchain::AppchainError,
    buckets::{CHECKPOINT_BUCKET, EXTERNAL_TX_BUCKET},
    proto,
    proto::{
        CheckpointResponse, ExternalTransaction as ProtoExternalTransaction, GetChainIdResponse,
        GetCheckpointsRequest, GetExternalTransactionsRequest, GetExternalTransactionsResponse,
        emitter_server::Emitter,
    },
    types::Checkpoint,
};

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

    async fn read_checkpoints(
        &self,
        start_block: u64,
        limit: u32,
    ) -> Result<Vec<Checkpoint>, AppchainError> {
        let db_guard = self.db.lock().await;
        let txn = db_guard.begin_ro_txn()?;
        let table = txn.open_table(Some(CHECKPOINT_BUCKET))?;

        let mut checkpoints = Vec::new();
        let mut cursor = txn.cursor(&table)?;

        let start_key = start_block.to_be_bytes();
        for entry in cursor.iter_from::<Vec<u8>, Vec<u8>>(start_key.as_slice()) {
            if checkpoints.len() >= limit as usize {
                break;
            }

            let (_, value) = entry?;
            let checkpoint: Checkpoint = serde_json::from_slice(value.as_slice())
                .map_err(|e| AppchainError::Conversion(e.to_string()))?;
            checkpoints.push(checkpoint);
        }

        Ok(checkpoints)
    }

    async fn read_external_transactions(
        &self,
        start_block: u64,
        limit: u32,
    ) -> Result<BTreeMap<u64, Vec<ProtoExternalTransaction>>, AppchainError> {
        let db_guard = self.db.lock().await;
        let txn = db_guard.begin_ro_txn()?;
        let table = txn.open_table(Some(EXTERNAL_TX_BUCKET))?;

        let mut tx_map = BTreeMap::new();
        let mut cursor = txn.cursor(&table)?;

        let start_key = start_block.to_be_bytes();
        for entry in cursor.iter_from::<Vec<u8>, Vec<u8>>(start_key.as_slice()) {
            if tx_map.len() >= limit as usize {
                break;
            }

            let (key_bytes, value) = entry?;
            let block_number = u64::from_be_bytes(
                key_bytes[..8]
                    .try_into()
                    .map_err(|e: std::array::TryFromSliceError| AppchainError::Conversion(e.to_string()))?,
            );

            let block_tx = proto::get_external_transactions_response::BlockTransactions::decode(
                value.as_slice(),
            )?;
            tx_map.insert(block_number, block_tx.external_transactions);
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
            .await
            .map_err(|e| Status::internal(format!("DB error: {:?}", e)))?;

        let proto_cps = checkpoints.iter().map(Self::checkpoint_to_proto).collect();

        Ok(Response::new(CheckpointResponse {
            checkpoints: proto_cps,
        }))
    }

    async fn create_internal_transactions_batch(
        &self,
        _request: Request<()>,
    ) -> Result<Response<crate::proto::CreateInternalTransactionsBatchResponse>, Status> {
        // Dummy implementation: return an empty batch hash and empty internal transactions.
        Ok(Response::new(
            crate::proto::CreateInternalTransactionsBatchResponse {
                batch_hash: vec![],
                internal_transactions: vec![],
            },
        ))
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
            .await
            .map_err(|e| Status::internal(format!("DB error: {:?}", e)))?;

        let blocks = tx_map
            .into_iter()
            .map(|(block_number, txs)| {
                crate::proto::get_external_transactions_response::BlockTransactions {
                    block_number,
                    transactions_root_hash: b"dummy_tx_hash".to_vec(), // dummy hash
                    external_transactions: txs,
                }
            })
            .collect();

        Ok(Response::new(GetExternalTransactionsResponse { blocks }))
    }

    async fn get_chain_id(
        &self,
        _request: Request<()>,
    ) -> Result<Response<GetChainIdResponse>, Status> {
        Ok(Response::new(GetChainIdResponse {
            chain_id: self.chain_id,
        }))
    }
}
