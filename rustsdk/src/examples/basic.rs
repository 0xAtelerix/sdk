use rustsdk::{
    Appchain, AppchainConfig, DefaultStateProcessor, 
    RootCalculator, BlockBuilder
};

struct SimpleRootCalc;
impl RootCalculator for SimpleRootCalc {
    fn calculate_root(&self, tx: &mdbx::RwTransaction) -> Result<[u8; 32], String> {
        // Implementation using tx to calculate state root
    }
}

struct SimpleBlockBuilder;
impl BlockBuilder<String> for SimpleBlockBuilder {
    fn build_block(
        &self,
        number: u64,
        state_root: [u8; 32],
        prev_hash: [u8; 32],
        txs: Batch<String>,
    ) -> Result<Vec<u8>, String> {
        // Simple JSON serialization
        serde_json::to_vec(&txs).map_err(|e| e.to_string())
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let config = AppchainConfig::new(1);
    let processor = DefaultStateProcessor {
        root_calculator: Box::new(SimpleRootCalc),
        block_builder: Box::new(SimpleBlockBuilder),
    };
    
    let appchain = Appchain::new(processor, config)?;
    
    tokio::join!(
        appchain.run(),
        async {
            // Block processing loop
            appchain.process_blocks().unwrap();
        }
    );
    
    Ok(())
}