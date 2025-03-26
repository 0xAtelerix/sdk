//! Module for reading event batches from a file.
//!
//! This implementation mimics the Go version using an async file reader.
//! It reads batches of events from the file starting at a given position.
//! Each batch is prefixed by a 4-byte big-endian size, followed by 32 bytes
//! of "atropos", then a series of events where each event is prefixed by a
//! 4-byte size.

use std::path::Path;
use thiserror::Error;
use tokio::fs::File;
use tokio::io::{AsyncReadExt, AsyncSeekExt, SeekFrom};
use tokio::time::{sleep, Duration};

/// Custom error type for reader operations.
#[derive(Debug, Error)]
pub enum ReaderError {
    #[error("I/O error: {0}")]
    Io(#[from] std::io::Error),
    #[error("Corrupted batch data")]
    CorruptedBatch,
}

/// A read batch containing an atropos, a list of events, and the file offset.
#[derive(Debug, Clone)]
pub struct ReadBatch {
    pub atropos: [u8; 32],
    pub events: Vec<Vec<u8>>,
    /// The file offset where this batch starts.
    pub offset: u64,
}

/// Trait for types that can read batches of transactions.
#[async_trait::async_trait]
pub trait BatchReader {
    /// Blocks until up to `limit` new batches are available.
    async fn get_new_batches_blocking(&mut self, limit: usize) -> Result<Vec<ReadBatch>, ReaderError>;
}

/// EventReader reads batches from a file with simple polling for new data.
pub struct EventReader {
    file: File,
    /// Current position in the file.
    position: u64,
    /// Indicates whether EOF has been reached.
    reached_eof: bool,
    /// The path of the data file (used for simulated watching).
    _data_file_path: String,
}

impl EventReader {
    /// Creates a new EventReader starting at the given position.
    /// If `start_position` is less than 8, it is set to 8.
    pub async fn new<P: AsRef<Path>>(data_file_path: P, start_position: u64) -> Result<Self, ReaderError> {
        let path_str = data_file_path.as_ref().to_string_lossy().to_string();
        let file = File::open(&data_file_path).await?;
        let pos = if start_position < 8 { 8 } else { start_position };
        Ok(Self {
            file,
            position: pos,
            reached_eof: false,
            _data_file_path: path_str,
        })
    }

    /// Reads up to `limit` new batches from the file.
    async fn read_new_batches(&mut self, limit: usize) -> Result<Vec<ReadBatch>, ReaderError> {
        let mut batches = Vec::new();

        // Seek to the current position.
        self.file.seek(SeekFrom::Start(self.position)).await?;

        for _ in 0..limit {
            let batch_start = self.position;

            // Read 4 bytes for batch size.
            let mut size_buf = [0u8; 4];
            match self.file.read_exact(&mut size_buf).await {
                Ok(n) if n == size_buf.len() => {},
                Ok(_) => return Err(ReaderError::CorruptedBatch),
                Err(e) if e.kind() == std::io::ErrorKind::UnexpectedEof => {
                    self.reached_eof = true;
                    break;
                },
                Err(e) => return Err(e.into()),
            }
            let batch_size = u32::from_be_bytes(size_buf) as u64;
            // Read atropos (32 bytes).
            let mut atropos = [0u8; 32];
            match self.file.read_exact(&mut atropos).await {
                Ok(n) if n == atropos.len() => {},
                Ok(_) => {
                    // TODO: recheck this logic.
                    self.file.seek(SeekFrom::Start(batch_start)).await?;
                    self.reached_eof = true;
                    break;
                },
                Err(e) if e.kind() == std::io::ErrorKind::UnexpectedEof => {
                    // Roll back position.
                    self.file.seek(SeekFrom::Start(batch_start)).await?;
                    self.reached_eof = true;
                    break;
                },
                Err(e) => return Err(e.into()),
            }
            // Calculate remaining bytes to read for the batch.
            let remaining = batch_size.checked_sub(32).ok_or(ReaderError::CorruptedBatch)?;
            let mut remaining_data = vec![0u8; remaining as usize];
            match self.file.read_exact(&mut remaining_data).await {
                Ok(n) if n == remaining_data.len() => {},
                Ok(_) => {
                    self.file.seek(SeekFrom::Start(batch_start)).await?;
                    self.reached_eof = true;
                    break;
                },
                Err(e) if e.kind() == std::io::ErrorKind::UnexpectedEof => {
                    self.file.seek(SeekFrom::Start(batch_start)).await?;
                    self.reached_eof = true;
                    break;
                },
                Err(e) => return Err(e.into()),
            }

            // Parse events from the remaining data.
            let mut events = Vec::new();
            let mut offset = 0;
            while offset < remaining_data.len() {
                if offset + 4 > remaining_data.len() {
                    return Err(ReaderError::CorruptedBatch);
                }
                let event_size = u32::from_be_bytes([
                    remaining_data[offset],
                    remaining_data[offset + 1],
                    remaining_data[offset + 2],
                    remaining_data[offset + 3],
                ]) as usize;
                offset += 4;
                if offset + event_size > remaining_data.len() {
                    return Err(ReaderError::CorruptedBatch);
                }
                let event = remaining_data[offset..offset + event_size].to_vec();
                events.push(event);
                offset += event_size;
            }

            // Append the batch.
            batches.push(ReadBatch {
                atropos,
                events,
                offset: batch_start,
            });
            // Update current position: 4 bytes for size + batch_size bytes.
            self.position += 4 + batch_size;
        }

        Ok(batches)
    }
}

#[async_trait::async_trait]
impl BatchReader for EventReader {
    async fn get_new_batches_blocking(&mut self, limit: usize) -> Result<Vec<ReadBatch>, ReaderError> {
        loop {
            // If we haven't reached EOF, try reading batches.
            if !self.reached_eof {
                let batches = self.read_new_batches(limit).await?;
                if !batches.is_empty() {
                    return Ok(batches);
                }
            }
            // Simulate waiting for file changes (e.g., via a file watcher).
            sleep(Duration::from_millis(100)).await;
            // Reset reached_eof flag to reattempt reading.
            self.reached_eof = false;
        }
    }
}
