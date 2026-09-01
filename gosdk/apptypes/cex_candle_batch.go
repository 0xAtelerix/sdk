package apptypes

import (
	"fmt"
	"strings"

	sdkerrors "github.com/0xAtelerix/sdk/gosdk/library/errors"
)

// Venue-candle wire constants mirror the appchain bar-source contract
// (price source 5 = venue candle API, policy 1 = confirmed). The numeric
// values are part of the committed identity and must not drift.
const (
	CEXCandlePriceSourceVenueAPI uint8 = 5
	CEXCandlePolicyConfirmed     uint8 = 1
)

const (
	ErrCEXCandleInvalid         sdkerrors.SDKError = "invalid cex candle"
	ErrCEXCandleBatchRefInvalid sdkerrors.SDKError = "invalid cex candle batch ref"
)

// CEXCandleBar is one finalized venue candle in a bootstrap/continuation
// payload. OHLCV are canonical decimal strings; the producer never scales —
// the adopting appchain converts once against the registered market scale.
type CEXCandleBar struct {
	BarStartMS uint64 `json:"barStartMs" cbor:"1,keyasint"`
	BarCloseMS uint64 `json:"barCloseMs" cbor:"2,keyasint"`
	Open       string `json:"open"       cbor:"3,keyasint"`
	High       string `json:"high"       cbor:"4,keyasint"`
	Low        string `json:"low"        cbor:"5,keyasint"`
	Close      string `json:"close"      cbor:"6,keyasint"`
	Volume     string `json:"volume"     cbor:"7,keyasint"`
}

// ValidateCEXCandleBars validates a fixed-order candle payload before storage
// or adoption. Bars must sit on the timeframe grid, close exactly one
// timeframe after they open, carry canonical positive OHLC (volume may be a
// canonical "0" — a venue candle can close without trades), satisfy
// high ≥ max(open, close) and low ≤ min(open, close), and ascend strictly by
// bar start. Gaps between bars are legal: venues legitimately omit intervals.
func ValidateCEXCandleBars(bars []CEXCandleBar, timeframeMS uint64) error {
	if len(bars) == 0 {
		return fmt.Errorf("%w: empty batch", ErrCEXCandleInvalid)
	}

	if timeframeMS == 0 {
		return fmt.Errorf("%w: timeframe is zero", ErrCEXCandleInvalid)
	}

	previousStart := uint64(0)

	for i, bar := range bars {
		if bar.BarStartMS == 0 || bar.BarStartMS%timeframeMS != 0 {
			return fmt.Errorf("%w: off-grid bar start at index=%d", ErrCEXCandleInvalid, i)
		}

		if bar.BarCloseMS != bar.BarStartMS+timeframeMS {
			return fmt.Errorf(
				"%w: close is not start plus timeframe at index=%d",
				ErrCEXCandleInvalid,
				i,
			)
		}

		for _, price := range []string{bar.Open, bar.High, bar.Low, bar.Close} {
			if !isCanonicalPositiveDecimal(price) {
				return fmt.Errorf(
					"%w: noncanonical price at index=%d",
					ErrCEXCandleInvalid,
					i,
				)
			}
		}

		if bar.Volume != "0" && !isCanonicalPositiveDecimal(bar.Volume) {
			return fmt.Errorf("%w: noncanonical volume at index=%d", ErrCEXCandleInvalid, i)
		}

		if compareCanonicalDecimals(bar.High, bar.Open) < 0 ||
			compareCanonicalDecimals(bar.High, bar.Close) < 0 {
			return fmt.Errorf("%w: high below open or close at index=%d", ErrCEXCandleInvalid, i)
		}

		if compareCanonicalDecimals(bar.Low, bar.Open) > 0 ||
			compareCanonicalDecimals(bar.Low, bar.Close) > 0 {
			return fmt.Errorf("%w: low above open or close at index=%d", ErrCEXCandleInvalid, i)
		}

		if i > 0 && bar.BarStartMS <= previousStart {
			return fmt.Errorf(
				"%w: unordered or duplicate bar start at index=%d",
				ErrCEXCandleInvalid,
				i,
			)
		}

		previousStart = bar.BarStartMS
	}

	return nil
}

// compareCanonicalDecimals orders two canonical decimal strings numerically
// without parsing into floats: integer parts compare by length then
// lexically; fractional parts compare lexically after right-padding, which
// is exact because canonical form carries no trailing zeros.
func compareCanonicalDecimals(a, b string) int {
	aInt, aFrac, _ := strings.Cut(a, ".")
	bInt, bFrac, _ := strings.Cut(b, ".")

	if len(aInt) != len(bInt) {
		if len(aInt) < len(bInt) {
			return -1
		}

		return 1
	}

	if c := strings.Compare(aInt, bInt); c != 0 {
		return c
	}

	if len(aFrac) < len(bFrac) {
		aFrac += strings.Repeat("0", len(bFrac)-len(aFrac))
	} else if len(bFrac) < len(aFrac) {
		bFrac += strings.Repeat("0", len(aFrac)-len(bFrac))
	}

	return strings.Compare(aFrac, bFrac)
}

// CEXCandleBatchRef identifies one immutable venue-candle batch. The payload
// remains in the later storage owner; the ref carries exact registered market
// identity, generation coordinates, and bounded provenance.
type CEXCandleBatchRef struct {
	ExchangeID      CEXExchangeID   `json:"exchangeId"      cbor:"1,keyasint"`
	MarketTypeID    CEXMarketTypeID `json:"marketTypeId"    cbor:"2,keyasint"`
	SymbolID        CEXSymbolID     `json:"symbolId"        cbor:"3,keyasint"`
	TimeframeMS     uint64          `json:"timeframeMs"     cbor:"4,keyasint"`
	PriceSource     uint8           `json:"priceSource"     cbor:"5,keyasint"`
	Policy          uint8           `json:"policy"          cbor:"6,keyasint"`
	GenerationID    uint64          `json:"generationId"    cbor:"7,keyasint"`
	BatchIndex      uint32          `json:"batchIndex"      cbor:"8,keyasint"`
	BatchCount      uint32          `json:"batchCount"      cbor:"9,keyasint"`
	BarCount        uint32          `json:"barCount"        cbor:"10,keyasint"`
	FirstBarStartMS uint64          `json:"firstBarStartMs" cbor:"11,keyasint"`
	LastBarCloseMS  uint64          `json:"lastBarCloseMs"  cbor:"12,keyasint"`
	EncodedBytes    uint32          `json:"encodedBytes"    cbor:"13,keyasint"`
	PayloadSHA256   [32]byte        `json:"payloadSha256"   cbor:"14,keyasint"`
	// BatchID is the immutable storage row identity in the producer store;
	// consumers use it for the exact payload lookup and outbox acknowledgement,
	// exactly like CEXMarketTradeBatchRef.BatchID.
	BatchID uint64 `json:"batchId" cbor:"15,keyasint"`
}

// Validate rejects incomplete, unregistered, or out-of-contract candle batch
// refs before a producer or consumer can treat the reference as durable
// provenance. Identity resolution mirrors CEXMarketTradeBatchRef.
func (r CEXCandleBatchRef) Validate() error {
	if r.ExchangeID == 0 {
		return fmt.Errorf("%w: exchange_id is zero", ErrCEXCandleBatchRefInvalid)
	}

	if _, ok := DefaultOrderBookIDRegistry.ExchangeLabel(r.ExchangeID); !ok {
		return fmt.Errorf(
			"%w: unknown exchange_id=%d",
			ErrCEXCandleBatchRefInvalid,
			r.ExchangeID,
		)
	}

	if r.MarketTypeID == 0 {
		return fmt.Errorf("%w: market_type_id is zero", ErrCEXCandleBatchRefInvalid)
	}

	if _, ok := DefaultOrderBookIDRegistry.MarketTypeLabel(r.MarketTypeID); !ok {
		return fmt.Errorf(
			"%w: unknown market_type_id=%d",
			ErrCEXCandleBatchRefInvalid,
			r.MarketTypeID,
		)
	}

	if r.SymbolID == 0 {
		return fmt.Errorf("%w: symbol_id is zero", ErrCEXCandleBatchRefInvalid)
	}

	if _, ok := DefaultOrderBookIDRegistry.SymbolLabel(
		r.ExchangeID,
		r.MarketTypeID,
		r.SymbolID,
	); !ok {
		return fmt.Errorf(
			"%w: unknown symbol identity=%d/%d/%d",
			ErrCEXCandleBatchRefInvalid,
			r.ExchangeID,
			r.MarketTypeID,
			r.SymbolID,
		)
	}

	if r.TimeframeMS == 0 {
		return fmt.Errorf("%w: timeframe is zero", ErrCEXCandleBatchRefInvalid)
	}

	if r.PriceSource != CEXCandlePriceSourceVenueAPI {
		return fmt.Errorf(
			"%w: price_source=%d is not venue candle api",
			ErrCEXCandleBatchRefInvalid,
			r.PriceSource,
		)
	}

	if r.Policy != CEXCandlePolicyConfirmed {
		return fmt.Errorf(
			"%w: policy=%d is not confirmed",
			ErrCEXCandleBatchRefInvalid,
			r.Policy,
		)
	}

	if r.GenerationID == 0 {
		return fmt.Errorf("%w: generation_id is zero", ErrCEXCandleBatchRefInvalid)
	}

	if r.BatchID == 0 {
		return fmt.Errorf("%w: batch_id is zero", ErrCEXCandleBatchRefInvalid)
	}

	if r.BatchCount == 0 {
		return fmt.Errorf("%w: batch_count is zero", ErrCEXCandleBatchRefInvalid)
	}

	if r.BatchIndex >= r.BatchCount {
		return fmt.Errorf(
			"%w: batch_index=%d out of range count=%d",
			ErrCEXCandleBatchRefInvalid,
			r.BatchIndex,
			r.BatchCount,
		)
	}

	if r.BarCount == 0 {
		return fmt.Errorf("%w: bar_count is zero", ErrCEXCandleBatchRefInvalid)
	}

	if r.FirstBarStartMS == 0 || r.LastBarCloseMS == 0 {
		return fmt.Errorf("%w: bar window is zero", ErrCEXCandleBatchRefInvalid)
	}

	if r.LastBarCloseMS <= r.FirstBarStartMS {
		return fmt.Errorf("%w: bar window is reversed", ErrCEXCandleBatchRefInvalid)
	}

	if r.EncodedBytes == 0 {
		return fmt.Errorf("%w: encoded_bytes is zero", ErrCEXCandleBatchRefInvalid)
	}

	if r.PayloadSHA256 == [32]byte{} {
		return fmt.Errorf("%w: payload_sha256 is zero", ErrCEXCandleBatchRefInvalid)
	}

	return nil
}
