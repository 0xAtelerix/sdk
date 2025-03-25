use crate::appchain::HealthCheckResponse;
use tonic::{Request, Response, Status};

#[derive(Debug, Default)]
pub struct HealthService;

#[tonic::async_trait]
impl super::appchain::Health for HealthService {
    async fn check(
        &self,
        _: Request<()>,
    ) -> Result<Response<HealthCheckResponse>, Status> {
        Ok(Response::new(HealthCheckResponse {
            status: HealthCheckResponse_ServingStatus::Serving.into(),
        }))
    }

    async fn watch(
        &self,
        _: Request<()>,
    ) -> Result<Response<Self::WatchStream>, Status> {
        Err(Status::unimplemented("Watch not implemented"))
    }
}