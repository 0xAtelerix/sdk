#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::tempdir;
    use tonic::transport::Channel;

    async fn setup_test() -> (EmitterClient<Channel>, tempfile::TempDir) {
        let dir = tempdir().unwrap();
        let env = Environment::new().open(dir.path()).unwrap();
        
        let server = AppchainEmitterServer::new(env, 1);
        let addr = "http://[::1]:0".parse().unwrap();
        
        let (tx, rx) = tokio::sync::oneshot::channel();
        let serve = Server::builder()
            .add_service(EmitterServer::new(server))
            .serve_with_shutdown(addr, async { rx.await.unwrap() });
        
        let client = EmitterClient::connect(addr).await.unwrap();
        (client, dir)
    }

    #[tokio::test]
    async fn test_checkpoint_flow() {
        let (mut client, _dir) = setup_test().await;
        
        // Insert test data
        let checkpoint = Checkpoint { /* ... */ };
        let mut env = client.get_ref().env.clone();
        let tx = env.begin_rw_txn().unwrap();
        store_checkpoint(&tx, &checkpoint).unwrap();
        tx.commit().unwrap();

        // Test retrieval
        let response = client.get_checkpoints(GetCheckpointsRequest {
            chain_id: 1,
            latest_previous_checkpoint_block_number: 0,
            limit: Some(1),
        }).await.unwrap();

        assert_eq!(response.into_inner().checkpoints.len(), 1);
    }
}