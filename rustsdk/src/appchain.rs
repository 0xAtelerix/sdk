//! Module implementing the core Appchain logic.
//!
//! This module provides the `Appchain` struct that ties together state transition logic,
//! state root calculation, block construction, event streaming, and an emitter API.
//!
//! It closely follows the original Go code structure and behavior.

use std::convert::TryInto;
use std::sync::Arc;
use tokio::net::TcpListener;
use tokio::sync::Mutex;
use tokio_stream::wrappers::TcpListenerStream;
use tonic::transport::Server;

use crate::buckets::*;
use crate::proto;
use crate::reader::ReaderError;
use crate::reader::{BatchReader, ReadBatch};
use crate::state_transition::StateTransitionInterface;
use crate::types::{AppTransaction, AppchainBlock, Batch, Checkpoint, RootCalculator};

use tokio_util::sync::CancellationToken;
use libmdbx::{Database, NoWriteMap, Transaction, RW, WriteFlags};

/// Appchain error type.
#[derive(Debug, thiserror::Error)]
pub enum AppchainError {
    #[error("Database error: {0}")]
    Db(#[from] libmdbx::Error),
    
    #[error("State transition error: {0}")]
    StateTransition(String),
    
    #[error("Root calculation error: {0}")]
    RootCalculation(String),
    
    #[error("Emitter error: {0}")]
    Emitter(String),
    
    #[error("I/O error: {0}")]
    Io(#[from] std::io::Error),
    
    #[error("Serialization error: {0}")]
    Serialization(#[from] bincode::Error),
    
    #[error("Conversion error: {0}")]
    Conversion(String),

    // Add missing variants
    #[error("Data not found: {0}")]
    NotFound(String),
    
    #[error("Corrupted data: {0}")]
    CorruptedData(String),
    
    #[error("ReaderError data: {0}")]
    ReadingDB(ReaderError),
}

/// Appchain configuration.
#[derive(Clone, Debug)]
pub struct AppchainConfig {
    pub chain_id: u64,
    pub emitter_port: String,
    pub appchain_db_path: String,
    pub tmp_db_path: String,
    pub event_stream_dir: String,
}

impl AppchainConfig {
    /// Returns a default configuration.
    pub fn new(chain_id: u64) -> Self {
        Self {
            chain_id,
            emitter_port: ":50051".to_string(),
            appchain_db_path: "./test".to_string(),
            tmp_db_path: "./test_tmp".to_string(),
            event_stream_dir: "".to_string(),
        }
    }
}

/// Type alias for our MDBX database.
type DB = Database<NoWriteMap>;

/// The Appchain struct.
pub struct Appchain<STI, Tx, Block, BR, E>
where
    STI: StateTransitionInterface<Tx> + Send + Sync + 'static,
    Tx: AppTransaction + Send + Sync + 'static,
    Block: AppchainBlock + Send + Sync + 'static,
    BR: BatchReader + Send + Sync + 'static,
    E: Clone + Send + Sync + 'static, // Emitter API server type
{
    pub state_transition: STI,
    pub root_calculator: Arc<dyn RootCalculator + Send + Sync>,
    /// Closure to build a block.
    pub block_builder: Arc<dyn Fn(u64, [u8; 32], [u8; 32], &Batch<Tx>) -> Block + Send + Sync>,
    pub event_stream: BR,
    /// Emitter API server.
    pub emitter_api: E,
    /// MDBX database environment.
    pub appchain_db: Arc<Mutex<DB>>,
    pub config: AppchainConfig,
}

impl<STI, Tx, Block, BR, E> Appchain<STI, Tx, Block, BR, E>
where
    STI: StateTransitionInterface<Tx> + Send + Sync + 'static,
    Tx: AppTransaction + Send + Sync + 'static,
    Block: AppchainBlock + Send + Sync + 'static,
    BR: BatchReader + Send + Sync + 'static,
    E: proto::emitter_server::Emitter + Clone + Send + Sync + 'static,
{
    /// Constructs a new Appchain.
    pub fn new(
        state_transition: STI,
        root_calculator: Arc<dyn RootCalculator + Send + Sync>,
        block_builder: Arc<dyn Fn(u64, [u8; 32], [u8; 32], &Batch<Tx>) -> Block + Send + Sync>,
        event_stream: BR,
        emitter_api: E,
        config: AppchainConfig,
    ) -> Result<Self, AppchainError> {
        let db = DB::open(&config.appchain_db_path)?;
        Ok(Self {
            state_transition,
            root_calculator,
            block_builder,
            event_stream,
            emitter_api,
            appchain_db: Arc::new(Mutex::new(db)),
            config,
        })
    }

    /// Runs the appchain.
    pub async fn run(&mut self, shutdown: CancellationToken) -> Result<(), AppchainError> {
        // Spawn the emitter API server.
        let emitter_port = self.config.emitter_port.clone();
        let db_clone = self.appchain_db.clone();
        let chain_id = self.config.chain_id;
        let emitter_api = self.emitter_api.clone();
        tokio::spawn(async move {
            if let Err(e) =
                Self::run_emitter_api(emitter_port, emitter_api).await
            {
                log::error!("Emitter API error: {:?}", e);
            }
        });

        // Retrieve previous block state.
        let (mut previous_block_number, mut previous_block_hash) = get_last_block(&self.appchain_db).await?;

        // Main processing loop.
        while !shutdown.is_cancelled() {
            let raw_batches = self
                .event_stream
                .get_new_batches_blocking(10)
                .await?;

            for raw_batch in raw_batches {
                let typed_batch = parse_read_batch::<Tx>(raw_batch)
                    .map_err(|e| AppchainError::Conversion(e.to_string()))?;

                let ext_txs = self
                    .state_transition
                    .process_batch(&typed_batch)
                    .map_err(|e| AppchainError::StateTransition(format!("{:?}", e)))?;

                let state_root = self
                    .root_calculator
                    .state_root_calculator()
                    .map_err(|e| AppchainError::RootCalculation(e.to_string()))?;

                let block_number = previous_block_number + 1;
                let block = (self.block_builder)(block_number, state_root, previous_block_hash, &typed_batch);

                let mut db_guard = self.appchain_db.lock().await;
                {
                    // Begin a write transaction.
                    let mut txn = db_guard.begin_rw_txn()?;
                    write_block(&mut txn, block.number(), &block.bytes())?;
                    let block_hash = block.hash();
                    let external_tx_root = write_external_transactions(&mut txn, block_number, &ext_txs)?;
                    let checkpoint = Checkpoint {
                        chain_id: self.config.chain_id,
                        block_number,
                        block_hash,
                        state_root,
                        external_transactions_root: external_tx_root,
                    };
                    write_checkpoint(&mut txn, &checkpoint)?;
                    write_last_block(&mut txn, block_number, block_hash)?;
                    txn.commit().map_err(|e| AppchainError::Db(e))?;
                    previous_block_number = block.number();
                    previous_block_hash = block.hash();
                }
            }
        }
        Ok(())
    }

    /// Runs the emitter API gRPC server.
    async fn run_emitter_api(
        emitter_port: String,
        emitter_api: E,
    ) -> Result<(), AppchainError> {
        let addr = emitter_port
            .parse::<std::net::SocketAddr>()
            .map_err(|e| AppchainError::Emitter(e.to_string()))?;
        let listener = TcpListener::bind(addr)
            .await
            .map_err(|e| AppchainError::Io(e))?;
        let incoming = TcpListenerStream::new(listener);

        Server::builder()
            .add_service(proto::emitter_server::EmitterServer::new(emitter_api))
            .add_service(
                proto::health_server::HealthServer::new(HealthCheckServiceServer::default()),
            )
            .serve_with_incoming(incoming)
            .await
            .map_err(|e| AppchainError::Emitter(e.to_string()))?;
        Ok(())
    }
}

/// Internal conversion error.
#[derive(Debug, thiserror::Error)]
enum ConversionError {
    #[error("Serialization error: {0}")]
    Serialization(#[from] bincode::Error),
}

/// Converts a raw ReadBatch into a typed Batch<T>.
fn parse_read_batch<T: AppTransaction>(raw: ReadBatch) -> Result<crate::types::Batch<T>, ConversionError> {
    use crate::types::Batch;
    let mut transactions = Vec::new();
    for event in raw.events {
        // FIXME: won't work as far as it tries to decode event, not a transaction
        let tx: T = bincode::deserialize(&event)?;
        transactions.push(tx);
    }
    Ok(Batch {
        transactions,
        external_blocks: Vec::new(), // No external blocks parsed here.
    })
}

/// Writes a block into the "blocks" bucket.
/// Accepts a mutable reference to a write transaction.
fn write_block<E>(txn: &mut Transaction<RW, E>, block_number: u64, block_bytes: &[u8]) -> Result<(), Box<dyn std::error::Error>>
where
    E: libmdbx::DatabaseKind,
{
    let key = block_number.to_be_bytes();
    let table = txn.open_table(Some(BLOCKS_BUCKET))?;
    txn.put(&table, &key, block_bytes, WriteFlags::empty())?;
    Ok(())
}

/// Writes last block info into the "config" bucket.
fn write_last_block<E>(txn: &mut Transaction<RW, E>, number: u64, hash: [u8; 32]) -> Result<(), Box<dyn std::error::Error>>
where
    E: libmdbx::DatabaseKind,
{
    let mut value = [0u8; 8 + 32];
    value[..8].copy_from_slice(&number.to_be_bytes());
    value[8..].copy_from_slice(&hash);
    let table = txn.open_table(Some(CONFIG_BUCKET))?;
    txn.put(&table, LAST_BLOCK_KEY.as_bytes(), &value, WriteFlags::empty())?;
    Ok(())
}

async fn get_last_block(db: &Arc<Mutex<DB>>) -> Result<(u64, [u8; 32]), AppchainError> {
    let db_guard = db.lock().await;
    let txn = db_guard.begin_ro_txn()?;
    let table = txn.open_table(Some(CONFIG_BUCKET))?;
    
    let value = txn.get::<Vec<u8>>(&table, LAST_BLOCK_KEY.as_bytes())?
        .ok_or(AppchainError::NotFound("Last block data".into()))?;

    if value.len() != 8 + 32 {
        return Err(AppchainError::CorruptedData(format!(
            "Invalid last block data length: {} bytes", 
            value.len()
        )));
    }

    let number = u64::from_be_bytes(
        value[..8].try_into()
            .map_err(|_| AppchainError::CorruptedData("Block number conversion failed".into()))?
    );
    
    let mut hash = [0u8; 32];
    hash.copy_from_slice(&value[8..40]);
    
    Ok((number, hash))
}

/// Serializes external transactions using protobuf and writes them to the "external_transactions" bucket.
/// Returns the computed root.
fn write_external_transactions<E>(
    txn: &mut Transaction<RW, E>,
    block_number: u64,
    txs: &[crate::types::ExternalTransaction],
) -> Result<[u8; 32], Box<dyn std::error::Error>>
where
    E: libmdbx::DatabaseKind,
{
    let root = merklize(txs);
    let proto_bytes = transaction_to_proto(txs, block_number, root)?;
    let key = block_number.to_be_bytes();
    let table = txn.open_table(Some(EXTERNAL_TX_BUCKET))?;
    txn.put(&table, &key, &proto_bytes, WriteFlags::empty())?;
    Ok(root)
}

/// Converts a list of external transactions into a protobuf-encoded block transactions message.
fn transaction_to_proto(
    txs: &[crate::types::ExternalTransaction],
    block_number: u64,
    root: [u8; 32],
) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
    use crate::proto;
    let proto_txs: Vec<proto::ExternalTransaction> = txs
        .iter()
        .map(|tx| proto::ExternalTransaction {
            chain_id: tx.chain_id,
            tx: tx.tx.clone(),
        })
        .collect();
    let block_tx = proto::get_external_transactions_response::BlockTransactions {
        block_number,
        transactions_root_hash: root.to_vec(),
        external_transactions: proto_txs,
    };
    let bytes = prost::Message::encode_to_vec(&block_tx);
    Ok(bytes)
}

/// Serializes the checkpoint as JSON and writes it to the "checkpoints" bucket.
fn write_checkpoint<E>(
    txn: &mut Transaction<RW, E>,
    checkpoint: &Checkpoint,
) -> Result<(), AppchainError>
where
    E: libmdbx::DatabaseKind,
{
    let key = checkpoint.block_number.to_be_bytes();
    let value = serde_json::to_vec(checkpoint)?;
    let table = txn.open_table(Some(CHECKPOINT_BUCKET))?;
    txn.put(&table, &key, &value, WriteFlags::empty())?;
    Ok(())
}

/// Computes a Merkle root for the external transactions.
/// (This is a placeholder; implement the actual Merkleization as needed.)
fn merklize(_txs: &[crate::types::ExternalTransaction]) -> [u8; 32] {
    [0u8; 32]
}

#[derive(Default)]
pub struct HealthCheckServiceServer {}

#[tonic::async_trait]
impl proto::health_server::Health for HealthCheckServiceServer {
    async fn check(
        &self,
        _request: tonic::Request<()>,
    ) -> Result<tonic::Response<proto::HealthCheckResponse>, tonic::Status> {
        Ok(tonic::Response::new(proto::HealthCheckResponse {
            status: proto::health_check_response::ServingStatus::Serving as i32,
        }))
    }

    type WatchStream = tokio_stream::wrappers::ReceiverStream<Result<proto::HealthCheckResponse, tonic::Status>>;

    async fn watch(
        &self,
        _request: tonic::Request<()>,
    ) -> Result<tonic::Response<Self::WatchStream>, tonic::Status> {
        // Implement your stream here
        let (_tx, rx) = tokio::sync::mpsc::channel(4);
        Ok(tonic::Response::new(tokio_stream::wrappers::ReceiverStream::new(rx)))
    }
}