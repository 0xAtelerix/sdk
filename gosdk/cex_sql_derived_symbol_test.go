package gosdk

import (
	"path/filepath"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// A market listed after the frozen v9 table has a derived id the registry
// cannot name. Its ref carries the label, and the reader accepts it only when
// the label hashes to that id.
func TestReadCEXOrderBooksAcceptsDerivedSymbolIDWithMatchingLabel(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	dbPath := filepath.Join(t.TempDir(), "cex.sqlite")
	db, err := openCEXTestDB(ctx, dbPath)
	require.NoError(t, err)

	bids, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "1", Quantity: "2"}})
	require.NoError(t, err)
	asks, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "3", Quantity: "4"}})
	require.NoError(t, err)

	const label = "NEWUSDT"

	derived := apptypes.SymbolID(1, label)
	require.GreaterOrEqual(t, derived, apptypes.CEXDerivedSymbolIDBase)
	require.NoError(t, insertCEXOrderBookV6(
		ctx, db, 1, 1, uint32(derived), "mexc", "spot", label, 11, bids, asks, 100,
	))
	require.NoError(t, db.Close())

	accessor, err := NewCEXDataAccessSQL(ctx, dbPath)
	require.NoError(t, err)

	defer accessor.Close()

	snapshots, readErrs := accessor.ReadCEXOrderBooks(ctx, []apptypes.CEXOrderBookRef{
		{Symbol: label, ExchangeID: 1, MarketTypeID: 1, SymbolID: derived, FetchedAt: 100},
		{ExchangeID: 1, MarketTypeID: 1, SymbolID: derived, FetchedAt: 100},
		{Symbol: "OTHERUSDT", ExchangeID: 1, MarketTypeID: 1, SymbolID: derived, FetchedAt: 100},
	})
	require.Len(t, readErrs, 3)
	require.NoError(t, readErrs[0])
	require.Equal(t, label, snapshots[0].Symbol)
	require.Equal(t, int64(11), snapshots[0].LastUpdateID)
	require.ErrorIs(t, readErrs[1], errCEXOrderBookUnknownIdentity)
	require.ErrorIs(t, readErrs[2], errCEXOrderBookUnknownIdentity)
}
