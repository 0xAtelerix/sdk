use crate::Batch;
use notify::{RecommendedWatcher, Watcher};
use std::{
    path::Path,
    sync::{Arc, Mutex},
    time::Duration,
};

pub struct EventReader<T> {
    file: std::fs::File,
    watcher: Arc<Mutex<RecommendedWatcher>>,
    position: u64,
    marker: std::marker::PhantomData<T>,
}

impl<T: serde::de::DeserializeOwned> EventReader<T> {
    pub fn new(path: &Path, start_pos: u64) -> Result<Self, String> {
        let file = std::fs::OpenOptions::new()
            .read(true)
            .open(path)
            .map_err(|e| format!("File error: {}", e))?;

        let watcher = notify::recommended_watcher(|res| {
            if let Err(e) = res {
                log::error!("Watcher error: {}", e);
            }
        })
        .map_err(|e| e.to_string())?;

        watcher
            .watch(path, notify::RecursiveMode::NonRecursive)
            .map_err(|e| e.to_string())?;

        Ok(Self {
            file,
            watcher: Arc::new(Mutex::new(watcher)),
            position: start_pos.max(8), // Minimum header size
            marker: std::marker::PhantomData,
        })
    }

    pub fn read_batches(&mut self, limit: usize) -> Result<Vec<Batch<T>>, String> {
        let mut batches = Vec::new();
        let mut buf = Vec::new();

        self.file
            .seek(std::io::SeekFrom::Start(self.position))
            .map_err(|e| e.to_string())?;

        for _ in 0..limit {
            match self.read_next_batch() {
                Ok(Some(batch)) => {
                    self.position = batch.offset + batch.size as u64;
                    batches.push(batch.data);
                }
                Ok(None) => break,
                Err(e) => return Err(e),
            }
        }

        Ok(batches)
    }

    async fn try_read_batch(&mut self) -> Result<Option<Batch<T>>, std::io::Error> {
        let mut size_buf = [0u8; 4];
        self.file.read_exact(&mut size_buf).await?;
        let size = u32::from_be_bytes(size_buf) as usize;

        if size > self.buffer.len() {
            self.buffer.resize(size, 0);
        }

        self.file.read_exact(&mut self.buffer[..size]).await?;
        let batch: Batch<T> = bincode::deserialize(&self.buffer[..size])
            .map_err(|e| std::io::Error::new(std::io::ErrorKind::InvalidData, e))?;

        Ok(Some(batch))
    }

    async fn wait_for_data(&self) -> Result<(), ReaderError> {
        let mut interval = tokio::time::interval(Duration::from_millis(100));
        loop {
            interval.tick().await;
            if self.file.metadata().await?.len() > self.position {
                return Ok(());
            }
        }
    }
}

#[derive(thiserror::Error, Debug)]
pub enum ReaderError {
    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),
    #[error("Notify error: {0}")]
    Notify(#[from] notify::Error),
    #[error("Serialization error: {0}")]
    Serialization(#[from] bincode::Error),
}
