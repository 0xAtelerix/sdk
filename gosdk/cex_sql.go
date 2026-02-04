package gosdk

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/goccy/go-json"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// CEXDataAccessSQL implements CEXDataAccessor using SQLite
type CEXDataAccessSQL struct {
	db *sql.DB
}

// NewCEXDataAccessSQL opens the CEX SQLite database in read-only mode.
func NewCEXDataAccessSQL(ctx context.Context, dbPath string) (*CEXDataAccessSQL, error) {
	db, err := openSQLite(ctx, dbPath, "ro")
	if err != nil {
		return nil, fmt.Errorf("open cex sqlite %s: %w", dbPath, err)
	}

	return &CEXDataAccessSQL{db: db}, nil
}

// ReadCEXOrderBook reads a specific order book snapshot by exchange, symbol, and fetchedAt timestamp.
func (c *CEXDataAccessSQL) ReadCEXOrderBook(
	ctx context.Context,
	exchange string,
	symbol string,
	fetchedAt int64,
) (*apptypes.CEXOrderBookSnapshot, error) {
	var (
		lastUpdateID     int64
		bidsRaw, asksRaw []byte
	)

	err := c.db.QueryRowContext(ctx, `
		SELECT last_update_id, bids, asks
		FROM cex_orderbooks
		WHERE exchange = ? AND symbol = ? AND fetched_at = ?
		LIMIT 1
	`, exchange, symbol, fetchedAt).Scan(&lastUpdateID, &bidsRaw, &asksRaw)
	if err != nil {
		return nil, fmt.Errorf("read order book %s/%s@%d: %w", exchange, symbol, fetchedAt, err)
	}

	snapshot := &apptypes.CEXOrderBookSnapshot{
		Exchange:     exchange,
		Symbol:       symbol,
		LastUpdateID: lastUpdateID,
		FetchedAt:    fetchedAt,
	}

	if err := json.Unmarshal(bidsRaw, &snapshot.Bids); err != nil {
		return nil, fmt.Errorf("unmarshal bids %s/%s: %w", exchange, symbol, err)
	}

	if err := json.Unmarshal(asksRaw, &snapshot.Asks); err != nil {
		return nil, fmt.Errorf("unmarshal asks %s/%s: %w", exchange, symbol, err)
	}

	return snapshot, nil
}

// Close closes the underlying SQLite database.
func (c *CEXDataAccessSQL) Close() {
	if c.db != nil {
		c.db.Close()
	}
}
