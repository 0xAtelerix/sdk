#[derive(thiserror::Error, Debug)]
pub enum AppchainError {
    #[error("Database error: {0}")]
    Database(#[from] mdbx::Error),
    
    #[error("Serialization error: {0}")]
    Serialization(#[from] serde_json::Error),
    
    #[error("gRPC transport error: {0}")]
    GrpcTransport(#[from] tonic::transport::Error),
    
    #[error("Invalid configuration: {0}")]
    Config(String),
    
    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),
    
    #[error("Invalid data format: {0}")]
    DataFormat(String),
}

impl From<AppchainError> for tonic::Status {
    fn from(e: AppchainError) -> Self {
        match e {
            AppchainError::Database(_) => 
                Status::internal(e.to_string()),
            AppchainError::Serialization(_) => 
                Status::invalid_argument(e.to_string()),
            _ => Status::unknown(e.to_string()),
        }
    }
}