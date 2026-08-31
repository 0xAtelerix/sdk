package txpool

import (
	"context"
	"testing"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/stretchr/testify/require"
)

func TestAddTransactionAdmissionHookIsAdditive(t *testing.T) {
	t.Run("existing-behaviour-unchanged", func(t *testing.T) {
		pool := newArtifactTestPool(t)
		ctx := context.Background()
		tx := CustomTransaction[Receipt]{From: "alice", To: "bob", Value: 1}

		require.NoError(t, pool.AddTransaction(ctx, tx))
		txHash := tx.Hash()
		stored, err := pool.GetTransaction(ctx, txHash[:])
		require.NoError(t, err)
		require.Equal(t, tx, stored)

		_, err = pool.GetArtifact(ctx, []byte("not-admitted"))
		require.ErrorIs(t, err, ErrArtifactNotFound)
	})

	t.Run("artifact-and-transaction-admitted-together", func(t *testing.T) {
		pool := newArtifactTestPool(t)
		ctx := context.Background()
		tx := CustomTransaction[Receipt]{From: "alice", To: "bob", Value: 2}
		key := []byte("artifact-key")
		artifact := []byte("compiled-wasm")

		require.NoError(t, pool.AddTransactionWithArtifact(ctx, tx, key, artifact))
		txHash := tx.Hash()
		storedTx, err := pool.GetTransaction(ctx, txHash[:])
		require.NoError(t, err)
		require.Equal(t, tx, storedTx)
		storedArtifact, err := pool.GetArtifact(ctx, key)
		require.NoError(t, err)
		require.Equal(t, artifact, storedArtifact)
	})
}

func TestArtifactSurvivesTransactionBatching(t *testing.T) {
	t.Run("admission-batch-apply", func(t *testing.T) {
		pool := newArtifactTestPool(t)
		ctx := context.Background()
		tx := CustomTransaction[Receipt]{From: "alice", To: "bob", Value: 3}
		key := []byte("retained-artifact")
		artifact := []byte("compiled-wasm")

		require.NoError(t, pool.AddTransactionWithArtifact(ctx, tx, key, artifact))
		_, transactions, err := pool.CreateTransactionBatch(ctx)
		require.NoError(t, err)
		require.Len(t, transactions, 1)

		txHash := tx.Hash()
		_, err = pool.GetTransaction(ctx, txHash[:])
		require.Error(t, err)
		storedArtifact, err := pool.GetArtifact(ctx, key)
		require.NoError(t, err)
		require.Equal(t, artifact, storedArtifact)
	})
}

func newArtifactTestPool(t *testing.T) *TxPool[CustomTransaction[Receipt], Receipt] {
	t.Helper()
	db, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(t.TempDir()).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg { return Tables() }).
		Open()
	require.NoError(t, err)
	pool := NewTxPool[CustomTransaction[Receipt]](db)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	return pool
}
