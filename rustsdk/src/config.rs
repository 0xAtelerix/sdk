use std::path::PathBuf;

#[derive(Clone, Debug)]
pub struct AppchainConfig {
    pub chain_id: u64,
    pub grpc_addr: String,
    pub db_path: PathBuf,
    pub txpool_path: PathBuf,
    pub event_stream_dir: PathBuf,
}

impl AppchainConfig {
    pub fn new(chain_id: u64) -> Self {
        Self {
            chain_id,
            grpc_addr: "127.0.0.1:50051".into(),
            db_path: PathBuf::from("./chain_data"),
            txpool_path: PathBuf::from("./txpool_data"),
            event_stream_dir: PathBuf::from("./events"),
        }
    }
}