pub mod appchain {
    tonic::include_proto!("atelerix");
}

pub use appchain::{
    CheckpointResponse, CreateInternalTransactionsBatchResponse,
    GetChainIdResponse, GetCheckpointsRequest, GetExternalTransactionsRequest,
    GetExternalTransactionsResponse, HealthCheckResponse, emitter_client, emitter_server,
    health_client, health_server,
};

#[cfg(test)]
mod tests {
    use super::*;
    use tokio::net::TcpListener;
    use tokio_stream::wrappers::TcpListenerStream;
    use tonic::{Request, Response, Status, transport::Server};

    #[derive(Debug, Default)]
    struct MockEmitter;

    #[tonic::async_trait]
    impl emitter_server::Emitter for MockEmitter {
        async fn get_checkpoints(
            &self,
            _: Request<GetCheckpointsRequest>,
        ) -> Result<Response<CheckpointResponse>, Status> {
            Ok(Response::new(CheckpointResponse {
                checkpoints: vec![],
            }))
        }

        async fn create_internal_transactions_batch(
            &self,
            _: Request<()>,
        ) -> Result<Response<CreateInternalTransactionsBatchResponse>, Status> {
            Ok(Response::new(CreateInternalTransactionsBatchResponse {
                batch_hash: vec![],
                internal_transactions: vec![],
            }))
        }

        async fn get_external_transactions(
            &self,
            _: Request<GetExternalTransactionsRequest>,
        ) -> Result<Response<GetExternalTransactionsResponse>, Status> {
            Ok(Response::new(GetExternalTransactionsResponse {
                blocks: vec![],
            }))
        }

        async fn get_chain_id(
            &self,
            _: Request<()>,
        ) -> Result<Response<GetChainIdResponse>, Status> {
            Ok(Response::new(GetChainIdResponse { chain_id: 42 }))
        }
    }

    #[tokio::test]
    async fn test_grpc_services() -> Result<(), Box<dyn std::error::Error>> {
        let listener = TcpListener::bind("127.0.0.1:0").await?;
        let addr = listener.local_addr()?;

        tokio::spawn(async move {
            Server::builder()
                .add_service(emitter_server::EmitterServer::new(MockEmitter))
                .serve_with_incoming(TcpListenerStream::new(listener))
                .await
        });

        // Test Emitter client
        let mut emitter_client =
            emitter_client::EmitterClient::connect(format!("http://{addr}")).await?;

        let chain_id = emitter_client.get_chain_id(()).await?.into_inner().chain_id;
        assert_eq!(chain_id, 42);

        Ok(())
    }
}
