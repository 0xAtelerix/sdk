//! Module implementing a generic transaction pool.
//!
//! The TxPool struct allows adding, retrieving, removing, and listing all transactions.
//! Transactions must implement the `AppTransaction` trait (bound by Serialize/Deserialize).

use libmdbx::{Database, TableFlags, WriteFlags, NoWriteMap};
use serde::{de::DeserializeOwned, Serialize};
use std::marker::PhantomData;
use std::path::Path;
use std::sync::{Arc, Mutex};
use thiserror::Error;
use bincode::error::{EncodeError, DecodeError};

use crate::buckets::TX_POOL;

#[derive(Debug, Error)]
pub enum TxPoolError {
    #[error("MDBX error: {0}")]
    MdbxError(#[from] libmdbx::Error),
    #[error("Serialization error: {0}")]
    SerializationError(#[from] EncodeError),
    #[error("Deserialization error: {0}")]
    DeserializationError(#[from] DecodeError),
}

/// Trait bound for application transactions.
pub trait AppTransaction: Serialize + DeserializeOwned {}

/// Generic transaction pool using MDBX.
/// The DB is opened on the provided path and a table named "txpool" is used.
pub struct TxPool<T: AppTransaction> {
    db: Arc<Mutex<Database<NoWriteMap>>>,
    phantom: PhantomData<T>,
}

impl<T: AppTransaction> TxPool<T> {
    /// Creates a new TxPool. Opens (or creates) the MDBX environment at the given path.
    pub fn new<P: AsRef<Path>>(db_path: P) -> Result<Self, TxPoolError> {
        let env = Database::open(db_path)?;
        Ok(Self {
            db: Arc::new(Mutex::new(env)),
            phantom: PhantomData,
        })
    }

    /// Adds a transaction to the pool using the provided hash as the key.
    pub fn add_transaction(&self, hash: &str, tx: &T) -> Result<(), TxPoolError> {
        // Serialize the transaction to binary.
        let data = bincode::encode_to_vec(tx, bincode::config::standard())?;
        let db = self.db.lock().unwrap();
        let mut txn = db.begin_rw_txn()?;
        // Open (or create) the "txpool" table.
        let table = txn
            .open_table(Some("txpool"))
            .or_else(|_| txn.create_table(Some("txpool"), TableFlags::empty()))?;
        txn.put(&table, hash.as_bytes(), &data, WriteFlags::empty())?;
        txn.commit()?;
        Ok(())
    }

    /// Retrieves a transaction by its hash.
    pub fn get_transaction(&self, hash: &str) -> Result<Option<T>, TxPoolError> {
        let db = self.db.lock().unwrap();
        let txn = db.begin_ro_txn()?;
        let table = txn.open_table(Some("txpool"))?;
        if let Some(data) = txn.get(&table, hash.as_bytes())? {
            let tx: T = bincode::decode_from_slice(&data, bincode::config::standard())?;
            Ok(Some(tx))
        } else {
            Ok(None)
        }
    }

    /// Removes a transaction from the pool.
    pub fn remove_transaction(&self, hash: &str) -> Result<(), TxPoolError> {
        let db = self.db.lock().unwrap();
        let txn = db.begin_rw_txn()?;
        let table = txn.open_table(Some("txpool"))?;
        txn.del(&table, hash.as_bytes(), None)?;
        txn.commit()?;
        Ok(())
    }

    /// Retrieves all transactions from the pool.
    pub fn get_all_transactions(&self) -> Result<Vec<T>, TxPoolError> {
        let db = self.db.lock().unwrap();
        let txn = db.begin_ro_txn()?;
        let table = txn.open_table(Some(TX_POOL))?;
        let mut transactions = Vec::new();
        let mut cursor = txn.cursor(&table)?;
        for result in cursor.iter() {
            let (_key, value) = result?;
            let tx: T = bincode::decode_from_slice(&value, bincode::config::standard())?;
            transactions.push(tx);
        }
        Ok(transactions)
    }

    pub fn get_all_transactions1(&self) -> Result<Vec<T>, TxPoolError> {
        let db = self.db.lock().unwrap();
        let txn = db.begin_ro_txn()?;
        let table = txn.open_table(Some(TX_POOL))?;
        let mut transactions = Vec::new();
        let mut cursor = txn.cursor(&table)?;
        for result in cursor.iter() {
            let (_key, value) = result?;
            // Use bincode::deserialize instead of decode_from_slice.
            let (tx, _): (T, usize) = bincode::decode_from_slice(&value, bincode::config::standard())?;
            serde::deserialize(&value).unwrap();
            transactions.push(tx);
        }
        Ok(transactions)
    }
    
    /// Closes the TxPool.
    /// (In libmdbx, resources are released on drop.)
    pub fn close(&self) {
        // Explicit close is not required; dropping self will clean up.
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde::{Deserialize, Serialize};
    use tempfile::tempdir;

    #[derive(Debug, Serialize, Deserialize, PartialEq, Clone)]
    struct CustomTransaction {
        pub hash: String,
        pub from: String,
        pub to: String,
        pub value: i32,
    }

    impl AppTransaction for CustomTransaction {}

    fn random_transaction(hash: &str) -> CustomTransaction {
        CustomTransaction {
            hash: hash.to_string(),
            from: "Alice".to_string(),
            to: "Bob".to_string(),
            value: 100,
        }
    }

    #[test]
    fn test_add_get_remove_transaction() -> Result<(), TxPoolError> {
        let temp_dir = tempdir().unwrap();
        let pool: TxPool<CustomTransaction> = TxPool::new(temp_dir.path())?;

        let tx = random_transaction("tx1");

        // Add transaction.
        pool.add_transaction(&tx.hash, &tx)?;

        // Retrieve and verify.
        let retrieved = pool.get_transaction(&tx.hash)?;
        assert_eq!(retrieved, Some(tx.clone()));

        // Remove and verify removal.
        pool.remove_transaction(&tx.hash)?;
        let retrieved = pool.get_transaction(&tx.hash)?;
        assert!(retrieved.is_none());

        Ok(())
    }

    #[test]
    fn test_get_all_transactions() -> Result<(), TxPoolError> {
        let temp_dir = tempdir().unwrap();
        let pool: TxPool<CustomTransaction> = TxPool::new(temp_dir.path())?;

        let tx1 = random_transaction("tx1");
        let tx2 = random_transaction("tx2");

        pool.add_transaction(&tx1.hash, &tx1)?;
        pool.add_transaction(&tx2.hash, &tx2)?;

        let mut all_txs = pool.get_all_transactions()?;
        // Order may be arbitrary, so sort by hash.
        all_txs.sort_by(|a: &CustomTransaction, b: &CustomTransaction| a.hash.cmp(&b.hash));

        let mut expected = vec![tx1, tx2];
        expected.sort_by(|a, b| a.hash.cmp(&b.hash));

        assert_eq!(all_txs, expected);

        Ok(())
    }
}