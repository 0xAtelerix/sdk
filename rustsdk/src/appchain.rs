use crate::{
    buckets::*, Checkpoint, Environment, ExternalTransaction, 
    StateTransition, TxPool, HealthService
};
use mdbx::{Commit, Transaction, RwTransaction};
use tonic::transport::Server;


pub struct Appchain<ST: StateTransition> {
    env: Environment,
    state_processor: ST,
    tx_pool: TxPool<ST::Transaction>,
    config: AppchainConfig,
}

impl<ST: StateTransition> Appchain<ST> {
    pub fn new(
        state_processor: ST,
        config: AppchainConfig,
    ) -> Result<Self, String> {
        let env = Environment::new()
            .set_max_dbs(10)
            .open(&config.db_path)
            .map_err(|e| format!("MDBX error: {}", e))?;

        initialize_db(&env)?;
        
        let tx_pool = TxPool::new(&config.txpool_path)?;

        Ok(Self {
            env,
            state_processor,
            tx_pool,
            config,
        })
    }

    pub async fn run(&self) -> Result<(), String> {
        let emitter_server = crate::emitter_api::AppchainEmitterServer::new(
            self.env.clone(),
            self.config.chain_id,
        );
        
        let health_server = HealthService::default();

        Server::builder()
            .add_service(EmitterServer::new(emitter_server))
            .add_service(HealthServer::new(health_server))
            .serve(self.config.grpc_addr.parse().map_err(|e| format!("{}", e))?)
            .await
            .map_err(|e| e.to_string())
    }

    fn process_blocks(&self) -> Result<(), String> {
        let mut last_block = self.get_last_block()?;
        
        loop {
            let batches = self.tx_pool.get_new_batches()?;
            
            for batch in batches {
                let mut tx = self.env.begin_rw_txn()?;
                
                let ext_txs = self.state_processor.process_batch(batch, &mut tx)?;
                let state_root = self.state_processor.root_calculator.calculate_root(&tx)?;
                
                // Build and store block
                let block_data = self.state_processor.block_builder.build_block(
                    last_block.number + 1,
                    state_root,
                    last_block.hash,
                    batch,
                )?;
                
                // Update checkpoint
                let checkpoint = Checkpoint {
                    chain_id: self.config.chain_id,
                    block_number: last_block.number + 1,
                    block_hash: blake3::hash(&block_data).into(),
                    state_root,
                    external_transactions_root: merkle_root(&ext_txs),
                };
                
                store_checkpoint(&mut tx, &checkpoint)?;
                tx.commit()?;
                
                last_block = checkpoint.into();
            }
        }
    }
}

// Helper database initialization
fn initialize_db(env: &Environment) -> Result<(), String> {
    let tx = env.begin_rw_txn()?;
    tx.create_db(Some(CHECKPOINT_BUCKET), None)?;
    tx.create_db(Some(EXTERNAL_TX_BUCKET), None)?;
    tx.create_db(Some(BLOCKS_BUCKET), None)?;
    tx.create_db(Some(CONFIG_BUCKET), None)?;
    tx.commit()?;
    Ok(())
}