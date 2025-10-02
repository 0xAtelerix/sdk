package tests

import (
	"testing"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/memdb"
	"github.com/stretchr/testify/require"
)

func TestDB(t *testing.T, buckets ...string) (kv.RwDB, func()) {
	t.Helper()

	db := memdb.NewTestDB(t)

	// Create tables
	err := db.Update(t.Context(), func(tx kv.RwTx) error {
		for _, bucket := range buckets {
			txErr := tx.CreateBucket(bucket)
			require.NoError(t, txErr)
		}

		return nil
	})
	if err != nil {
		db.Close()
	}

	require.NoError(t, err)

	return db, db.Close
}
