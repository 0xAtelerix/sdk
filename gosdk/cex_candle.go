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
	errCEXCandleBarsTrailingBytes = errors.New("decode cex candle bars: trailing bytes")
	errCEXCandleBarsNonCanonical  = errors.New("decode cex candle bars: noncanonical cbor")
)

// cexCandleBarWire is deliberately separate from the public app type. The
// protocol is a compact fixed-order CBOR array, while the public type keeps
// field names useful to callers and other encoders.
type cexCandleBarWire struct {
	//nolint:revive // fxamacker/cbor requires this blank sentinel for a fixed-order array.
	_          struct{} `cbor:",toarray"`
	BarStartMS uint64
	BarCloseMS uint64
	Open       string
	High       string
	Low        string
	Close      string
	Volume     string
}

func toCEXCandleBarWire(bars []apptypes.CEXCandleBar) []cexCandleBarWire {
	wire := make([]cexCandleBarWire, len(bars))
	for i, bar := range bars {
		wire[i] = cexCandleBarWire{
			BarStartMS: bar.BarStartMS, BarCloseMS: bar.BarCloseMS,
			Open: bar.Open, High: bar.High, Low: bar.Low, Close: bar.Close,
			Volume: bar.Volume,
		}
	}

	return wire
}

func fromCEXCandleBarWire(wire []cexCandleBarWire) []apptypes.CEXCandleBar {
	bars := make([]apptypes.CEXCandleBar, len(wire))
	for i, bar := range wire {
		bars[i] = apptypes.CEXCandleBar{
			BarStartMS: bar.BarStartMS, BarCloseMS: bar.BarCloseMS,
			Open: bar.Open, High: bar.High, Low: bar.Low, Close: bar.Close,
			Volume: bar.Volume,
		}
	}

	return bars
}

// EncodeCEXCandleBars validates and canonically encodes one venue-candle
// batch for the given timeframe. The batch identity is the SHA-256 of these
// exact bytes.
func EncodeCEXCandleBars(
	bars []apptypes.CEXCandleBar,
	timeframeMS uint64,
) ([]byte, [32]byte, error) {
	if err := apptypes.ValidateCEXCandleBars(bars, timeframeMS); err != nil {
		return nil, [32]byte{}, err
	}

	mode, err := cbor.EncOptions{Sort: cbor.SortCanonical}.EncMode()
	if err != nil {
		return nil, [32]byte{}, fmt.Errorf("create canonical cex candle encoder: %w", err)
	}

	payload, err := mode.Marshal(toCEXCandleBarWire(bars))
	if err != nil {
		return nil, [32]byte{}, fmt.Errorf("encode cex candle bars: %w", err)
	}

	return payload, sha256.Sum256(payload), nil
}

// DecodeCEXCandleBars rejects noncanonical, malformed, off-grid, unordered,
// or trailing payloads. It never returns a partial bar slice.
func DecodeCEXCandleBars(payload []byte, timeframeMS uint64) ([]apptypes.CEXCandleBar, error) {
	var wire []cexCandleBarWire

	dec := cbor.NewDecoder(bytes.NewReader(payload))
	if err := dec.Decode(&wire); err != nil {
		return nil, fmt.Errorf("decode cex candle bars: %w", err)
	}

	if dec.NumBytesRead() != len(payload) {
		return nil, errCEXCandleBarsTrailingBytes
	}

	bars := fromCEXCandleBarWire(wire)
	if err := apptypes.ValidateCEXCandleBars(bars, timeframeMS); err != nil {
		return nil, err
	}

	canonical, _, err := EncodeCEXCandleBars(bars, timeframeMS)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(canonical, payload) {
		return nil, errCEXCandleBarsNonCanonical
	}

	return bars, nil
}
