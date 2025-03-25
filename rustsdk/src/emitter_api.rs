use crate::{appchain::*, buckets::*, Checkpoint, ExternalTransaction};
use mdbx::{Environment, Transaction, RwTransaction};
use tonic::{Request, Response, Status};

#[derive(Clone)]
pub struct AppchainEmitterServer {
    env: Environment,
    chain_id: u64,
}

impl AppchainEmitterServer {
    pub fn new(env: Environment, chain_id: u64) -> Self {
        Self { env, chain_id }
    }

    fn get_checkpoints(
        &self,
        start_block: u64,
        limit: u32,
    ) -> Result<Vec<CheckpointResponseCheckpoint>, Error> {
        let tx = self.env.begin_ro_txn()?;
        let mut cursor = tx.cursor(CHECKPOINT_BUCKET)?;
        
        let mut checkpoints = Vec::with_capacity(limit as usize);
        for (key, value) in cursor.iter_from(start_block.to_be_bytes()) {
            if checkpoints.len() >= limit as usize {
                break;
            }
            let checkpoint: Checkpoint = serde_json::from_slice(value)?;
            checkpoints.push(checkpoint.into());
        }
        
        Ok(checkpoints)
    }
}

#[tonic::async_trait]
impl Emitter for AppchainEmitterServer {
    async fn get_checkpoints(
        &self,
        request: Request<GetCheckpointsRequest>,
    ) -> Result<Response<CheckpointResponse>, Status> {
        let req = request.into_inner();
        let checkpoints = self.get_checkpoints(
            req.latest_previous_checkpoint_block_number,
            req.limit.unwrap_or(10),
        ).map_err(map_error)?;

        Ok(Response::new(CheckpointResponse { checkpoints }))
    }

    async fn get_external_transactions(
        &self,
        request: Request<GetExternalTransactionsRequest>,
    ) -> Result<Response<GetExternalTransactionsResponse>, Status> {
        let req = request.into_inner();
        let tx = self.env.begin_ro_txn().map_err(map_error)?;
        let mut cursor = tx.cursor(EXTERNAL_TX_BUCKET).map_err(map_error)?;

        let mut blocks = Vec::new();
        for (key, value) in cursor.iter_from(req.latest_previous_block_number.to_be_bytes()) {
            let block: ExternalTransactionsBlock = bincode::deserialize(value)
                .map_err(|e| Status::invalid_argument(e.to_string()))?;
            
            blocks.push(GetExternalTransactionsResponseBlockTransactions {
                block_number: block.number,
                transactions_root_hash: block.root_hash.to_vec(),
                external_transactions: block.transactions.into_iter()
                    .map(|tx| tx.into())
                    .collect(),
            });
            
            if blocks.len() >= req.limit.unwrap_or(10) as usize {
                break;
            }
        }

        Ok(Response::new(GetExternalTransactionsResponse { blocks }))
    }
}

fn map_error<E: std::error::Error>(e: E) -> Status {
    Status::internal(format!("Operation failed: {}", e))
}