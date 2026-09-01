package gosdk

import (
	"context"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// CEXDataAccessor is an interface for accessing CEX order book data
// Both core (writer) and SDK (reader) use this interface
type CEXDataAccessor interface {
	ReadCEXOrderBook(
		ctx context.Context,
		exchange string,
		symbol string,
		fetchedAt int64,
	) (*apptypes.CEXOrderBookSnapshot, error)
	ReadCEXOrderBooks(
		ctx context.Context,
		refs []apptypes.CEXOrderBookRef,
	) ([]*apptypes.CEXOrderBookSnapshot, []error)
	ReadCEXMarketTradeBatch(
		ctx context.Context,
		ref apptypes.CEXMarketTradeBatchRef,
	) ([]apptypes.CEXMarketTrade, error)
	Close()
}

// CEXCandleDataAccessor is the additive venue-candle payload reader. It is a
// separate interface so existing CEXDataAccessor implementations and fakes
// stay valid; consumers that adopt candle batches require both.
type CEXCandleDataAccessor interface {
	ReadCEXCandleBatch(
		ctx context.Context,
		ref apptypes.CEXCandleBatchRef,
	) ([]apptypes.CEXCandleBar, error)
}

// Ensure SQL implementation satisfies the interfaces.
var (
	_ CEXDataAccessor       = (*CEXDataAccessSQL)(nil)
	_ CEXCandleDataAccessor = (*CEXDataAccessSQL)(nil)
)
