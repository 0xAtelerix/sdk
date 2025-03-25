// rustsdk/src/txpool/test.rs
use proptest::prelude::*;
use tempfile::tempdir;

proptest! {
    #[test]
    fn txpool_operations(transactions in prop::collection::vec(any::<TestTransaction>(), 1..100)) {
        let dir = tempdir().unwrap();
        let pool = TxPool::<TestTransaction>::new(dir.path()).unwrap();
        let mut unique_txs = std::collections::HashMap::new();

        // Test insertion
        for tx in &transactions {
            let hash = format!("{:x}", blake3::hash(&tx.encode()));
            if unique_txs.contains_key(&hash) { continue; }
            
            pool.add_transaction(&hash, tx).unwrap();
            unique_txs.insert(hash, tx.clone());
        }

        // Verify retrieval
        for (hash, tx) in &unique_txs {
            let retrieved = pool.get_transaction(hash).unwrap().unwrap();
            assert_eq!(*tx, retrieved);
        }

        // Test deletion
        let to_delete = unique_txs.keys().take(unique_txs.len() / 2).collect::<Vec<_>>();
        for hash in &to_delete {
            pool.remove_transaction(hash).unwrap();
            assert!(pool.get_transaction(hash).unwrap().is_none());
        }

        // Verify remaining
        for (hash, tx) in &unique_txs {
            if !to_delete.contains(&hash.as_str()) {
                let retrieved = pool.get_transaction(hash).unwrap().unwrap();
                assert_eq!(*tx, retrieved);
            }
        }
    }
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
struct TestTransaction {
    from: String,
    to: String,
    amount: u64,
}

impl Arbitrary for TestTransaction {
    type Parameters = ();
    type Strategy = BoxedStrategy<Self>;
    
    fn arbitrary_with(_: Self::Parameters) -> Self::Strategy {
        (any::<String>(), any::<String>(), 1..1000u64)
            .prop_map(|(from, to, amount)| Self { from, to, amount })
            .boxed()
    }
}