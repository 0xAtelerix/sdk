package gosdk

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/goccy/go-json"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

func TestCEXDataAccessSQL_ReadCEXOrderBook_ClassifiesPrecisionMiss(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "cex.sqlite")
	db, err := openSQLite(ctx, dbPath, "rwc")
	require.NoError(t, err)

	defer db.Close()

	_, err = db.ExecContext(ctx, `
CREATE TABLE cex_orderbooks_v3 (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	exchange TEXT NOT NULL,
	symbol TEXT NOT NULL,
	last_update_id INTEGER NOT NULL,
	bids BLOB NOT NULL,
	asks BLOB NOT NULL,
	fetched_at INTEGER NOT NULL,
	consumed INTEGER NOT NULL DEFAULT 0
);
`)
	require.NoError(t, err)

	bids, err := json.Marshal([]apptypes.CEXPriceLevel{{Price: "1", Quantity: "2"}})
	require.NoError(t, err)
	asks, err := json.Marshal([]apptypes.CEXPriceLevel{{Price: "3", Quantity: "4"}})
	require.NoError(t, err)

	requestedFetchedAt := int64(1_777_000_000_000_000_000)
	_, err = db.ExecContext(
		ctx,
		`INSERT INTO cex_orderbooks_v3(exchange, symbol, last_update_id, bids, asks, fetched_at) VALUES(?, ?, ?, ?, ?, ?)`,
		"mexc",
		"SPXUSDT",
		11,
		bids,
		asks,
		requestedFetchedAt+1,
	)
	require.NoError(t, err)

	accessor := &CEXDataAccessSQL{db: db}
	_, err = accessor.ReadCEXOrderBook(ctx, "mexc", "SPXUSDT", requestedFetchedAt)
	require.Error(t, err)

	var readErr *CEXOrderBookReadError
	require.ErrorAs(t, err, &readErr)
	require.Equal(t, "no_row", readErr.Diagnostic.Result)
	require.Equal(t, "precision_mismatch", readErr.Diagnostic.MissHint)
	require.Equal(t, requestedFetchedAt+1, readErr.Diagnostic.NearestNewerFetchedAt)
}

func TestCEXDataAccessSQL_ReadCEXOrderBook_ClassifiesTrueAbsence(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "cex.sqlite")
	db, err := openSQLite(ctx, dbPath, "rwc")
	require.NoError(t, err)

	defer db.Close()

	_, err = db.ExecContext(ctx, `
CREATE TABLE cex_orderbooks_v3 (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	exchange TEXT NOT NULL,
	symbol TEXT NOT NULL,
	last_update_id INTEGER NOT NULL,
	bids BLOB NOT NULL,
	asks BLOB NOT NULL,
	fetched_at INTEGER NOT NULL,
	consumed INTEGER NOT NULL DEFAULT 0
);
`)
	require.NoError(t, err)

	accessor := &CEXDataAccessSQL{db: db}
	_, err = accessor.ReadCEXOrderBook(ctx, "mexc", "SPXUSDT", time.Now().UnixNano())
	require.Error(t, err)

	var readErr *CEXOrderBookReadError
	require.ErrorAs(t, err, &readErr)
	require.Equal(t, "no_row", readErr.Diagnostic.Result)
	require.Equal(t, "true_absence", readErr.Diagnostic.MissHint)
}

func TestCEXDataAccessSQL_ReadCEXOrderBooks_BatchesInOneTx(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "cex.sqlite")
	db, err := openSQLite(ctx, dbPath, "rwc")
	require.NoError(t, err)

	defer db.Close()

	_, err = db.ExecContext(ctx, `
CREATE TABLE cex_orderbooks_v3 (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	exchange TEXT NOT NULL,
	symbol TEXT NOT NULL,
	last_update_id INTEGER NOT NULL,
	bids BLOB NOT NULL,
	asks BLOB NOT NULL,
	fetched_at INTEGER NOT NULL,
	consumed INTEGER NOT NULL DEFAULT 0
);
`)
	require.NoError(t, err)

	bids, err := json.Marshal([]apptypes.CEXPriceLevel{{Price: "1", Quantity: "2"}})
	require.NoError(t, err)
	asks, err := json.Marshal([]apptypes.CEXPriceLevel{{Price: "3", Quantity: "4"}})
	require.NoError(t, err)

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO cex_orderbooks_v3(exchange, symbol, last_update_id, bids, asks, fetched_at) VALUES(?, ?, ?, ?, ?, ?)`,
		"mexc",
		"SPXUSDT",
		11,
		bids,
		asks,
		int64(100),
	)
	require.NoError(t, err)

	accessor := &CEXDataAccessSQL{db: db}
	snapshots, errs := accessor.ReadCEXOrderBooks(ctx, []apptypes.CEXOrderBookRef{
		{Exchange: "mexc", Symbol: "SPXUSDT", FetchedAt: 100},
		{Exchange: "mexc", Symbol: "ETHUSDT", FetchedAt: 200},
	})
	require.Len(t, snapshots, 2)
	require.Len(t, errs, 2)
	require.NoError(t, errs[0])
	require.NotNil(t, snapshots[0])
	require.Equal(t, int64(11), snapshots[0].LastUpdateID)
	require.Error(t, errs[1])
	require.Nil(t, snapshots[1])
}
