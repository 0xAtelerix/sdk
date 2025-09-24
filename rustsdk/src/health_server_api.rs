//! Module implementing the gRPC Health Check service.
//!
//! This dummy implementation always returns a healthy status.

use async_trait::async_trait;
use std::pin::Pin;
use tokio_stream::Stream;
use tonic::{Request, Response, Status};

use crate::proto::{health_server::Health, HealthCheckResponse};
use crate::proto::health_check_response::ServingStatus;

/// HealthService is a dummy implementation of the gRPC Health service.
#[derive(Debug, Default)]
pub struct HealthService {}

#[async_trait]
impl Health for HealthService {
    async fn check(
        &self,
        _request: Request<()>,
    ) -> Result<Response<HealthCheckResponse>, Status> {
        // Always return a healthy status.
        Ok(Response::new(HealthCheckResponse {
            status: ServingStatus::Serving as i32,
        }))
    }

    type WatchStream = Pin<Box<dyn Stream<Item = Result<HealthCheckResponse, Status>> + Send>>;

    async fn watch(
        &self,
        _request: Request<()>,
    ) -> Result<Response<Self::WatchStream>, Status> {
        // Dummy implementation: stream a single health check response.
        let output = Box::pin(tokio_stream::iter(vec![Ok(HealthCheckResponse {
            status: ServingStatus::Serving as i32,
        })]));
        Ok(Response::new(output))
    }
}
