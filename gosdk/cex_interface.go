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
	Close()
}

// Ensure SQL implementation satisfies the interface.
var _ CEXDataAccessor = (*CEXDataAccessSQL)(nil)
