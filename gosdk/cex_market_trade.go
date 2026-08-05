package gosdk

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/fxamacker/cbor/v2"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

var (
	errCEXMarketTradesTrailingBytes = errors.New("decode cex market trades: trailing bytes")
	errCEXMarketTradesNonCanonical  = errors.New("decode cex market trades: noncanonical cbor")
)

// cexMarketTradeWire is deliberately separate from the public app type.  The
// protocol is a compact fixed-order CBOR array, while the public type keeps
// field names useful to callers and other encoders.
type cexMarketTradeWire struct {
	//nolint:revive // fxamacker/cbor requires this blank sentinel for a fixed-order array.
	_            struct{} `cbor:",toarray"`
	Price        string
	Size         string
	Side         apptypes.CEXMarketTradeSide
	SourceTimeMS uint64
	TradeID      [16]byte
}

func toCEXMarketTradeWire(trades []apptypes.CEXMarketTrade) []cexMarketTradeWire {
	wire := make([]cexMarketTradeWire, len(trades))
	for i, trade := range trades {
		wire[i] = cexMarketTradeWire{
			Price: trade.Price, Size: trade.Size, Side: trade.Side,
			SourceTimeMS: trade.SourceTimeMS, TradeID: trade.TradeID,
		}
	}

	return wire
}

func fromCEXMarketTradeWire(wire []cexMarketTradeWire) []apptypes.CEXMarketTrade {
	trades := make([]apptypes.CEXMarketTrade, len(wire))
	for i, trade := range wire {
		trades[i] = apptypes.CEXMarketTrade{
			Price: trade.Price, Size: trade.Size, Side: trade.Side,
			SourceTimeMS: trade.SourceTimeMS, TradeID: trade.TradeID,
		}
	}

	return trades
}

// EncodeCEXMarketTrades validates and canonically encodes one compact market
// batch. The batch identity is the SHA-256 of these exact bytes.
func EncodeCEXMarketTrades(trades []apptypes.CEXMarketTrade) ([]byte, [32]byte, error) {
	if err := apptypes.ValidateCEXMarketTrades(trades); err != nil {
		return nil, [32]byte{}, err
	}

	mode, err := cbor.EncOptions{Sort: cbor.SortCanonical}.EncMode()
	if err != nil {
		return nil, [32]byte{}, fmt.Errorf("create canonical cex trade encoder: %w", err)
	}

	payload, err := mode.Marshal(toCEXMarketTradeWire(trades))
	if err != nil {
		return nil, [32]byte{}, fmt.Errorf("encode cex market trades: %w", err)
	}

	return payload, sha256.Sum256(payload), nil
}

// DecodeCEXMarketTrades rejects noncanonical, malformed, unordered, duplicate,
// or trailing payloads. It never returns a partial trade slice.
func DecodeCEXMarketTrades(payload []byte) ([]apptypes.CEXMarketTrade, error) {
	var wire []cexMarketTradeWire

	dec := cbor.NewDecoder(bytes.NewReader(payload))
	if err := dec.Decode(&wire); err != nil {
		return nil, fmt.Errorf("decode cex market trades: %w", err)
	}

	if dec.NumBytesRead() != len(payload) {
		return nil, errCEXMarketTradesTrailingBytes
	}

	trades := fromCEXMarketTradeWire(wire)
	if err := apptypes.ValidateCEXMarketTrades(trades); err != nil {
		return nil, err
	}

	canonical, _, err := EncodeCEXMarketTrades(trades)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(canonical, payload) {
		return nil, errCEXMarketTradesNonCanonical
	}

	return trades, nil
}
