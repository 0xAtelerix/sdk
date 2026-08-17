package gosdk

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// NO_TEST_DOUBLE: the production SQLite accessor reads a real database file;
// rows are written through the production codec.

const cexCandleBatchesV1TestSchema = `
CREATE TABLE cex_candle_batches_v1 (
    id INTEGER PRIMARY KEY,
    exchange_id INTEGER NOT NULL,
    market_type_id INTEGER NOT NULL,
    symbol_id INTEGER NOT NULL,
    timeframe_ms INTEGER NOT NULL,
    price_source INTEGER NOT NULL,
    policy INTEGER NOT NULL,
    generation_id INTEGER NOT NULL,
    batch_index INTEGER NOT NULL,
    batch_count INTEGER NOT NULL,
    bar_count INTEGER NOT NULL,
    first_bar_start_ms INTEGER NOT NULL,
    last_bar_close_ms INTEGER NOT NULL,
    encoded_bytes INTEGER NOT NULL,
    payload_sha256 BLOB NOT NULL,
    payload BLOB NOT NULL
)`

type cexCandleBatchReaderFixture struct {
	dbPath  string
	bars    []apptypes.CEXCandleBar
	payload []byte
	ref     apptypes.CEXCandleBatchRef
}

func newCEXCandleBatchReaderFixture(ctx context.Context, t *testing.T) cexCandleBatchReaderFixture {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "cex.sqlite")

	db, err := openSQLite(ctx, dbPath, "rwc")
	require.NoError(t, err)

	_, err = db.ExecContext(
		ctx,
		`CREATE TABLE cex_orderbook_pairs_v3 (
			id INTEGER PRIMARY KEY, exchange_id INTEGER NOT NULL,
			market_type_id INTEGER NOT NULL, symbol_id INTEGER NOT NULL
		)`,
	)
	require.NoError(t, err)
	_, err = db.ExecContext(
		ctx,
		`CREATE TABLE cex_orderbooks_v6 (
			id INTEGER PRIMARY KEY, pair_id INTEGER NOT NULL,
			last_update_id INTEGER NOT NULL, bids BLOB NOT NULL,
			asks BLOB NOT NULL, fetched_at INTEGER NOT NULL
		)`,
	)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, cexCandleBatchesV1TestSchema)
	require.NoError(t, err)

	bars := testCandleBars()

	payload, digest, err := EncodeCEXCandleBars(bars, testCandleTimeframeMS)
	require.NoError(t, err)

	symbolID := cexSymbolIDForTest(t, 3, 2, "BTCUSDT")

	ref := apptypes.CEXCandleBatchRef{
		ExchangeID:      3,
		MarketTypeID:    2,
		SymbolID:        apptypes.CEXSymbolID(symbolID),
		TimeframeMS:     testCandleTimeframeMS,
		PriceSource:     apptypes.CEXCandlePriceSourceVenueAPI,
		Policy:          apptypes.CEXCandlePolicyConfirmed,
		GenerationID:    7,
		BatchIndex:      0,
		BatchCount:      2,
		BarCount:        uint32(len(bars)),
		FirstBarStartMS: bars[0].BarStartMS,
		LastBarCloseMS:  bars[len(bars)-1].BarCloseMS,
		EncodedBytes:    uint32(len(payload)),
		PayloadSHA256:   digest,
		BatchID:         1,
	}
	require.NoError(t, ref.Validate())

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO cex_candle_batches_v1 VALUES(1,3,2,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		int64(symbolID),
		int64(ref.TimeframeMS),
		int64(ref.PriceSource),
		int64(ref.Policy),
		int64(ref.GenerationID),
		int64(ref.BatchIndex),
		int64(ref.BatchCount),
		int64(ref.BarCount),
		int64(ref.FirstBarStartMS),
		int64(ref.LastBarCloseMS),
		len(payload),
		digest[:],
		payload,
	)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	return cexCandleBatchReaderFixture{dbPath: dbPath, bars: bars, payload: payload, ref: ref}
}

func TestReadCEXCandleBatchReturnsValidatedPayload(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	fixture := newCEXCandleBatchReaderFixture(ctx, t)

	accessor, err := NewCEXDataAccessSQL(ctx, fixture.dbPath)
	require.NoError(t, err)

	defer accessor.Close()

	bars, err := accessor.ReadCEXCandleBatch(ctx, fixture.ref)
	require.NoError(t, err)
	require.Equal(t, fixture.bars, bars)
}

func TestReadCEXCandleBatchRejectsMismatches(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("missing row", func(t *testing.T) {
		t.Parallel()

		fixture := newCEXCandleBatchReaderFixture(ctx, t)
		fixture.ref.BatchID = 99

		accessor, err := NewCEXDataAccessSQL(ctx, fixture.dbPath)
		require.NoError(t, err)

		defer accessor.Close()

		_, err = accessor.ReadCEXCandleBatch(ctx, fixture.ref)
		require.ErrorIs(t, err, ErrCEXCandleBatchNotFound)
	})

	t.Run("ref metadata drift", func(t *testing.T) {
		t.Parallel()

		fixture := newCEXCandleBatchReaderFixture(ctx, t)
		fixture.ref.GenerationID++

		accessor, err := NewCEXDataAccessSQL(ctx, fixture.dbPath)
		require.NoError(t, err)

		defer accessor.Close()

		_, err = accessor.ReadCEXCandleBatch(ctx, fixture.ref)
		require.ErrorIs(t, err, ErrCEXCandleBatchInvalid)
	})

	t.Run("tampered payload", func(t *testing.T) {
		t.Parallel()

		fixture := newCEXCandleBatchReaderFixture(ctx, t)

		db, err := openSQLite(ctx, fixture.dbPath, "rw")
		require.NoError(t, err)
		tamperCEXCandleBatchPayload(ctx, t, db)
		require.NoError(t, db.Close())

		accessor, err := NewCEXDataAccessSQL(ctx, fixture.dbPath)
		require.NoError(t, err)

		defer accessor.Close()

		_, err = accessor.ReadCEXCandleBatch(ctx, fixture.ref)
		require.ErrorIs(t, err, ErrCEXCandleBatchInvalid)
	})
}

func tamperCEXCandleBatchPayload(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		`UPDATE cex_candle_batches_v1 SET payload = X'00' , encoded_bytes = 1 WHERE id = 1`,
	)
	require.NoError(t, err)
}
