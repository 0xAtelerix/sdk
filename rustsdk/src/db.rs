use mdbx::{Environment, Transaction};
use tokio::task;

pub async fn async_txn<F, R>(env: &Environment, f: F) -> Result<R, AppchainError>
where
    F: FnOnce(&Transaction) -> Result<R, AppchainError> + Send + 'static,
    R: Send + 'static,
{
    let env = env.clone();
    task::spawn_blocking(move || {
        let tx = env.begin_ro_txn()?;
        let result = f(&tx)?;
        Ok(result)
    })
    .await?
}
