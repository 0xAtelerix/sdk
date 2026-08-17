package apptypes

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

// NO_TEST_DOUBLE: pure wire-contract data, no substituted owner.

func validCEXCandleBatchRef() CEXCandleBatchRef {
	return CEXCandleBatchRef{
		ExchangeID:      3, // binance in the default registry
		MarketTypeID:    2, // perp
		SymbolID:        1, // BTCUSDT
		TimeframeMS:     900_000,
		PriceSource:     CEXCandlePriceSourceVenueAPI,
		Policy:          CEXCandlePolicyConfirmed,
		GenerationID:    3,
		BatchID:         42,
		BatchIndex:      0,
		BatchCount:      17,
		BarCount:        256,
		FirstBarStartMS: 1_786_800_600_000,
		LastBarCloseMS:  1_787_030_400_000,
		EncodedBytes:    12_345,
		PayloadSHA256:   [32]byte{31: 1},
	}
}

func TestCEXCandleBatchRefValidateAcceptsRegisteredIdentity(t *testing.T) {
	t.Parallel()

	ref := validCEXCandleBatchRef()
	require.NoError(t, ref.Validate())

	encoded, err := cbor.Marshal(ref)
	require.NoError(t, err)

	var decoded CEXCandleBatchRef

	require.NoError(t, cbor.Unmarshal(encoded, &decoded))
	require.Equal(t, ref, decoded)
}

func TestCEXCandleBatchRefValidateRejects(t *testing.T) {
	t.Parallel()

	mutations := map[string]func(*CEXCandleBatchRef){
		"zero exchange":        func(r *CEXCandleBatchRef) { r.ExchangeID = 0 },
		"unknown exchange":     func(r *CEXCandleBatchRef) { r.ExchangeID = 250 },
		"zero market type":     func(r *CEXCandleBatchRef) { r.MarketTypeID = 0 },
		"zero symbol":          func(r *CEXCandleBatchRef) { r.SymbolID = 0 },
		"unregistered symbol":  func(r *CEXCandleBatchRef) { r.SymbolID = 4_000_000_000 },
		"zero timeframe":       func(r *CEXCandleBatchRef) { r.TimeframeMS = 0 },
		"foreign price source": func(r *CEXCandleBatchRef) { r.PriceSource = 1 },
		"foreign policy":       func(r *CEXCandleBatchRef) { r.Policy = 2 },
		"zero generation":      func(r *CEXCandleBatchRef) { r.GenerationID = 0 },
		"zero batch id":        func(r *CEXCandleBatchRef) { r.BatchID = 0 },
		"zero batch count":     func(r *CEXCandleBatchRef) { r.BatchCount = 0 },
		"index out of range":   func(r *CEXCandleBatchRef) { r.BatchIndex = r.BatchCount },
		"zero bar count":       func(r *CEXCandleBatchRef) { r.BarCount = 0 },
		"zero first bar":       func(r *CEXCandleBatchRef) { r.FirstBarStartMS = 0 },
		"reversed bar window":  func(r *CEXCandleBatchRef) { r.LastBarCloseMS = r.FirstBarStartMS },
		"zero encoded bytes":   func(r *CEXCandleBatchRef) { r.EncodedBytes = 0 },
		"zero payload sha":     func(r *CEXCandleBatchRef) { r.PayloadSHA256 = [32]byte{} },
	}

	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ref := validCEXCandleBatchRef()
			mutate(&ref)
			require.ErrorIs(t, ref.Validate(), ErrCEXCandleBatchRefInvalid)
		})
	}
}

func validCEXCandleBar(startMS uint64) CEXCandleBar {
	return CEXCandleBar{
		BarStartMS: startMS,
		BarCloseMS: startMS + 900_000,
		Open:       "100.5",
		High:       "101.25",
		Low:        "99.9",
		Close:      "101",
		Volume:     "12.75",
	}
}

func TestValidateCEXCandleBarsAcceptsSparseAscendingAndZeroVolume(t *testing.T) {
	t.Parallel()

	first := validCEXCandleBar(1_786_800_600_000)
	first.Volume = "0" // venue candle with no trades in the interval

	// Deliberate gap: the venue omitted the intermediate interval.
	second := validCEXCandleBar(1_786_800_600_000 + 3*900_000)

	require.NoError(t, ValidateCEXCandleBars([]CEXCandleBar{first, second}, 900_000))
}

func TestValidateCEXCandleBarsRejects(t *testing.T) {
	t.Parallel()

	base := uint64(1_786_800_600_000)

	cases := map[string][]CEXCandleBar{
		"empty batch": {},
		"off-grid start": {func() CEXCandleBar {
			b := validCEXCandleBar(base + 1)
			b.BarCloseMS = base + 1 + 900_000

			return b
		}()},
		"close not start plus timeframe": {func() CEXCandleBar {
			b := validCEXCandleBar(base)
			b.BarCloseMS = base + 60_000

			return b
		}()},
		"noncanonical open": {func() CEXCandleBar {
			b := validCEXCandleBar(base)
			b.Open = "01.5"

			return b
		}()},
		"trailing zero close": {func() CEXCandleBar {
			b := validCEXCandleBar(base)
			b.Close = "1.50"

			return b
		}()},
		"signed volume": {func() CEXCandleBar {
			b := validCEXCandleBar(base)
			b.Volume = "-1"

			return b
		}()},
		"zero price": {func() CEXCandleBar {
			b := validCEXCandleBar(base)
			b.Low = "0"

			return b
		}()},
		"high below open": {func() CEXCandleBar {
			b := validCEXCandleBar(base)
			b.High = "100.4"

			return b
		}()},
		"high below close on integer length": {func() CEXCandleBar {
			b := validCEXCandleBar(base)
			b.High = "99.95"
			b.Open = "9.9"
			b.Close = "100"

			return b
		}()},
		"low above close": {func() CEXCandleBar {
			b := validCEXCandleBar(base)
			b.Low = "101.5"
			b.High = "102"

			return b
		}()},
		"unordered starts": {
			validCEXCandleBar(base + 900_000),
			validCEXCandleBar(base),
		},
		"duplicate starts": {
			validCEXCandleBar(base),
			validCEXCandleBar(base),
		},
	}

	for name, bars := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.ErrorIs(t, ValidateCEXCandleBars(bars, 900_000), ErrCEXCandleInvalid)
		})
	}
}

func TestBatchAndEventCarryCandleBatchRefs(t *testing.T) {
	t.Parallel()

	ref := validCEXCandleBatchRef()

	event := Event{CEXCandleBatchRefs: []CEXCandleBatchRef{ref}}
	encodedEvent, err := cbor.Marshal(event)
	require.NoError(t, err)

	var decodedEvent Event

	require.NoError(t, cbor.Unmarshal(encodedEvent, &decodedEvent))
	require.Equal(t, []CEXCandleBatchRef{ref}, decodedEvent.CEXCandleBatchRefs)

	batch := Batch[testAllMidsTx, testAllMidsReceipt]{
		CEXCandleBatchRefs: []CEXCandleBatchRef{ref},
	}
	encodedBatch, err := cbor.Marshal(batch)
	require.NoError(t, err)

	var decodedBatch Batch[testAllMidsTx, testAllMidsReceipt]

	require.NoError(t, cbor.Unmarshal(encodedBatch, &decodedBatch))
	require.Equal(t, []CEXCandleBatchRef{ref}, decodedBatch.CEXCandleBatchRefs)
}

func TestCandleRefReportingHelpers(t *testing.T) {
	t.Parallel()

	first := validCEXCandleBatchRef()

	second := validCEXCandleBatchRef()
	second.TimeframeMS = 3_600_000

	rendered := FormatCEXCandleRefMarkets([]CEXCandleBatchRef{first, first, second})
	require.Equal(t, "3/2/1@900000=2,3/2/1@3600000=1", rendered)
	require.Empty(t, FormatCEXCandleRefMarkets(nil))

	exchange, marketType, symbol, timeframe := CEXCandleRefLabels(first)
	require.Equal(t, "binance", exchange)
	require.Equal(t, "perp", marketType)
	require.Equal(t, "BTCUSDT", symbol)
	require.Equal(t, "900000", timeframe)

	unregistered := first
	unregistered.SymbolID = 4_000_000_000
	_, _, symbol, _ = CEXCandleRefLabels(unregistered)
	require.Equal(t, "4000000000", symbol)
}
