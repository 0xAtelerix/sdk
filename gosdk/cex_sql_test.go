package gosdk

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

func TestCEXDataAccessSQL_ReadCEXOrderBook_ClassifiesPrecisionMiss(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	dbPath := filepath.Join(t.TempDir(), "cex.sqlite")
	db, err := openCEXTestDB(ctx, dbPath)
	require.NoError(t, err)

	bids, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "1", Quantity: "2"}})
	require.NoError(t, err)
	asks, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "3", Quantity: "4"}})
	require.NoError(t, err)

	requestedFetchedAt := int64(1_777_000_000_000_000_000)
	err = insertCEXOrderBookV5(
		ctx,
		db,
		"mexc",
		"SPXUSDT",
		11,
		bids,
		asks,
		requestedFetchedAt+1,
	)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	accessor, err := NewCEXDataAccessSQL(ctx, dbPath)
	require.NoError(t, err)
	defer accessor.Close()

	_, err = accessor.ReadCEXOrderBook(ctx, "mexc", "SPXUSDT", requestedFetchedAt)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrCEXOrderBookNotFound)

	var readErr *CEXOrderBookReadError
	require.ErrorAs(t, err, &readErr)
	require.Equal(t, "no_row", readErr.Diagnostic.Result)
	require.Equal(t, "precision_mismatch", readErr.Diagnostic.MissHint)
	require.Equal(t, requestedFetchedAt+1, readErr.Diagnostic.NearestNewerFetchedAt)
}

func TestCEXDataAccessSQL_ReadCEXOrderBook_ClassifiesTrueAbsence(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	dbPath := filepath.Join(t.TempDir(), "cex.sqlite")
	db, err := openCEXTestDB(ctx, dbPath)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	accessor, err := NewCEXDataAccessSQL(ctx, dbPath)
	require.NoError(t, err)
	defer accessor.Close()

	_, err = accessor.ReadCEXOrderBook(ctx, "mexc", "SPXUSDT", time.Now().UnixNano())
	require.Error(t, err)
	require.ErrorIs(t, err, ErrCEXOrderBookNotFound)

	var readErr *CEXOrderBookReadError
	require.ErrorAs(t, err, &readErr)
	require.Equal(t, "no_row", readErr.Diagnostic.Result)
	require.Equal(t, "true_absence", readErr.Diagnostic.MissHint)
}

func TestCEXDataAccessSQL_ReadCEXOrderBooks_ReadsPreparedPoints(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	dbPath := filepath.Join(t.TempDir(), "cex.sqlite")
	db, err := openCEXTestDB(ctx, dbPath)
	require.NoError(t, err)

	bids, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "1", Quantity: "2"}})
	require.NoError(t, err)
	asks, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "3", Quantity: "4"}})
	require.NoError(t, err)

	err = insertCEXOrderBookV5(
		ctx,
		db,
		"mexc",
		"SPXUSDT",
		11,
		bids,
		asks,
		int64(100),
	)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	accessor, err := NewCEXDataAccessSQL(ctx, dbPath)
	require.NoError(t, err)
	defer accessor.Close()

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
	require.ErrorIs(t, errs[1], ErrCEXOrderBookNotFound)
	require.Nil(t, snapshots[1])
	require.Equal(t, []apptypes.CEXPriceLevel{{Price: "1", Quantity: "2"}}, snapshots[0].Bids)
	require.Equal(t, []apptypes.CEXPriceLevel{{Price: "3", Quantity: "4"}}, snapshots[0].Asks)

	var readErr *CEXOrderBookReadError
	require.ErrorAs(t, errs[1], &readErr)
	require.Equal(t, "no_row", readErr.Diagnostic.Result)
	require.Equal(t, "true_absence", readErr.Diagnostic.MissHint)
}

func BenchmarkCEXDataAccessSQL_ReadCEXOrderBooks_PreparedPoints(b *testing.B) {
	ctx := b.Context()
	dbPath := filepath.Join(b.TempDir(), "cex.sqlite")
	db, err := openCEXTestDB(ctx, dbPath)
	require.NoError(b, err)

	bids, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "1", Quantity: "2"}})
	require.NoError(b, err)
	asks, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "3", Quantity: "4"}})
	require.NoError(b, err)

	for i := 0; i < 100; i++ {
		err = insertCEXOrderBookV5(
			ctx,
			db,
			"mexc",
			"SPXUSDT",
			int64(i),
			bids,
			asks,
			int64(1_000+i),
		)
		require.NoError(b, err)
	}
	require.NoError(b, db.Close())

	accessor, err := NewCEXDataAccessSQL(ctx, dbPath)
	require.NoError(b, err)
	defer accessor.Close()

	for _, refCount := range []int{1, 10, 100} {
		refs := make([]apptypes.CEXOrderBookRef, refCount)
		for i := range refs {
			refs[i] = apptypes.CEXOrderBookRef{
				Exchange:  "mexc",
				Symbol:    "SPXUSDT",
				FetchedAt: int64(1_000 + i),
			}
		}

		b.Run("refs_"+strconv.Itoa(refCount), func(b *testing.B) {
			for range b.N {
				snapshots, errs := accessor.ReadCEXOrderBooks(ctx, refs)
				require.Len(b, snapshots, len(refs))
				for _, err := range errs {
					require.NoError(b, err)
				}
			}
		})
	}
}

func openCEXTestDB(ctx context.Context, dbPath string) (*sql.DB, error) {
	db, err := openSQLite(ctx, dbPath, "rwc")
	if err != nil {
		return nil, err
	}

	if _, err = db.ExecContext(ctx, createCEXOrderBooksV5SQL); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}

		return nil, err
	}

	return db, nil
}

func insertCEXOrderBookV5(
	ctx context.Context,
	db *sql.DB,
	exchange string,
	symbol string,
	lastUpdateID int64,
	bids []byte,
	asks []byte,
	fetchedAt int64,
) error {
	if _, err := db.ExecContext(ctx, `
		INSERT INTO cex_pairs_v2(exchange, symbol)
		VALUES(?, ?)
		ON CONFLICT(exchange, symbol) DO NOTHING
	`, exchange, symbol); err != nil {
		return fmt.Errorf("insert pair: %w", err)
	}

	var pairID int64
	if err := db.QueryRowContext(ctx, `
		SELECT id
		FROM cex_pairs_v2
		WHERE exchange = ? AND symbol = ?
	`, exchange, symbol).Scan(&pairID); err != nil {
		return fmt.Errorf("select pair: %w", err)
	}

	if _, err := db.ExecContext(ctx, `
		INSERT INTO cex_orderbooks_v5(pair_id, last_update_id, bids, asks, fetched_at)
		VALUES(?, ?, ?, ?, ?)
	`, pairID, lastUpdateID, bids, asks, fetchedAt); err != nil {
		return fmt.Errorf("insert orderbook: %w", err)
	}

	return nil
}

func openSQLite(ctx context.Context, dbPath, mode string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?mode=%s&cache=shared&uri=true", dbPath, mode)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.PingContext(ctx); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}

		return nil, err
	}

	return db, nil
}

const createCEXOrderBooksV5SQL = `
CREATE TABLE cex_pairs_v2 (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	exchange TEXT NOT NULL,
	symbol TEXT NOT NULL,
	UNIQUE(exchange, symbol)
);

CREATE TABLE cex_orderbooks_v5 (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	pair_id INTEGER NOT NULL,
	last_update_id INTEGER NOT NULL,
	bids BLOB NOT NULL,
	asks BLOB NOT NULL,
	fetched_at INTEGER NOT NULL,
	consumed INTEGER NOT NULL DEFAULT 0,
	FOREIGN KEY(pair_id) REFERENCES cex_pairs_v2(id)
);

CREATE INDEX idx_cex_ob_v5_pair_fetched ON cex_orderbooks_v5(pair_id, fetched_at);
`
