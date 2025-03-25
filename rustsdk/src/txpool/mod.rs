use crate::types::AppTransaction;
use mdbx::{Environment, Transaction, RwTransaction};
use serde::{Serialize, de::DeserializeOwned};
use std::{marker::PhantomData, path::Path};

const TXPOOL_BUCKET: &str = "txpool";

pub struct TxPool<T: AppTransaction> {
    env: Environment,
    _phantom: PhantomData<T>,
}

impl<T: AppTransaction + Serialize + DeserializeOwned> TxPool<T> {
    pub fn new(db_path: &Path) -> Result<Self, String> {
        let env = Environment::new()
            .set_max_dbs(10)
            .open(db_path)
            .map_err(|e| format!("MDBX error: {}", e))?;

        let tx = env.begin_rw_txn().map_err(|e| e.to_string())?;
        tx.create_db(Some(TXPOOL_BUCKET), None)
            .map_err(|e| e.to_string())?;
        tx.commit().map_err(|e| e.to_string())?;

        Ok(Self {
            env,
            _phantom: PhantomData,
        })
    }

    pub fn add_transaction(&self, hash: &str, tx: &T) -> Result<(), String> {
        let mut tx_rw = self.env.begin_rw_txn().map_err(|e| e.to_string())?;
        let serialized = serde_json::to_vec(tx).map_err(|e| e.to_string())?;
        tx_rw.put(TXPOOL_BUCKET, hash.as_bytes(), &serialized)
            .map_err(|e| e.to_string())?;
        tx_rw.commit().map_err(|e| e.to_string())
    }

    pub fn get_transaction(&self, hash: &str) -> Result<Option<T>, String> {
        let tx = self.env.begin_ro_txn().map_err(|e| e.to_string())?;
        match tx.get(TXPOOL_BUCKET, hash.as_bytes()) {
            Ok(data) => serde_json::from_slice(data)
                .map(Some)
                .map_err(|e| e.to_string()),
            Err(mdbx::Error::NotFound) => Ok(None),
            Err(e) => Err(e.to_string()),
        }
    }

    pub fn remove_transaction(&self, hash: &str) -> Result<(), String> {
        let mut tx_rw = self.env.begin_rw_txn().map_err(|e| e.to_string())?;
        tx_rw.del(TXPOOL_BUCKET, hash.as_bytes())
            .map_err(|e| e.to_string())?;
        tx_rw.commit().map_err(|e| e.to_string())
    }

    pub fn get_all_transactions(&self) -> Result<Vec<T>, String> {
        let tx = self.env.begin_ro_txn().map_err(|e| e.to_string())?;
        let mut cursor = tx.cursor(TXPOOL_BUCKET).map_err(|e| e.to_string())?;
        let mut txs = Vec::new();

        for (_, value) in cursor.iter() {
            let tx: T = serde_json::from_slice(value).map_err(|e| e.to_string())?;
            txs.push(tx);
        }

        Ok(txs)
    }
}