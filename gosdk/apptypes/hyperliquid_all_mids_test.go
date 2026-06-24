package apptypes

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/stretchr/testify/require"
)

type testAllMidsTx struct{}

func (testAllMidsTx) Hash() [32]byte { return [32]byte{1} }

func (testAllMidsTx) Process(kv.RwTx) (testAllMidsReceipt, []ExternalTransaction, error) {
	return testAllMidsReceipt{}, nil, nil
}

type testAllMidsReceipt struct{}

func (testAllMidsReceipt) TxHash() [32]byte { return [32]byte{1} }

func (testAllMidsReceipt) Status() TxReceiptStatus { return ReceiptConfirmed }

func (testAllMidsReceipt) Error() string { return "" }

func TestHyperliquidAllMidsRefsRoundTripSeparatelyFromOrderBookRefs(t *testing.T) {
	t.Parallel()

	ref := HyperliquidAllMidsRef{
		Kind:              HyperliquidPublicDataKindAllMids,
		Network:           HyperliquidNetworkTestnet,
		MarketType:        HyperliquidMarketTypePerp,
		AssetID:           1,
		Symbol:            "ETHUSDC",
		MidPriceQuoteUnit: "2350.125",
		FetchedAtUnixMS:   1_706_000_000_000,
	}

	raw, err := cbor.Marshal(Batch[testAllMidsTx, testAllMidsReceipt]{
		HyperliquidAllMidsRefs: []HyperliquidAllMidsRef{ref},
		CEXOrderBookRefs: []CEXOrderBookRef{{
			Exchange:  "mexc",
			Symbol:    "SPXUSDT",
			FetchedAt: 99,
		}},
	})
	require.NoError(t, err)

	var batch Batch[testAllMidsTx, testAllMidsReceipt]
	require.NoError(t, cbor.Unmarshal(raw, &batch))
	require.Equal(t, []HyperliquidAllMidsRef{ref}, batch.HyperliquidAllMidsRefs)
	require.Equal(t, []CEXOrderBookRef{{
		Exchange:  "mexc",
		Symbol:    "SPXUSDT",
		FetchedAt: 99,
	}}, batch.CEXOrderBookRefs)

	eventRaw, err := Event{HyperliquidAllMidsRefs: []HyperliquidAllMidsRef{ref}}.Bytes()
	require.NoError(t, err)

	var event Event
	require.NoError(t, cbor.Unmarshal(eventRaw, &event))
	require.Equal(t, []HyperliquidAllMidsRef{ref}, event.HyperliquidAllMidsRefs)
	require.Empty(t, event.CEXOrderBookRefs)
}
