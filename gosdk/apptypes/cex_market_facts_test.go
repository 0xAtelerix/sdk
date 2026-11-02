package apptypes

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

func u8(v uint8) *uint8 { return &v }

func u32(v uint32) *uint32 { return &v }

func fullCEXMarketFacts() CEXMarketFacts {
	return CEXMarketFacts{
		ExchangeID: CEXExchangeIDHyperliquid, MarketTypeID: CEXMarketTypeIDSpot,
		Label: "HYPEUSDC", SymbolID: 0x80000001, Network: "mainnet", VenueCoin: "@107",
		Kind: CEXMarketKindHyperliquidSpot, Status: CEXMarketStatusTrading,
		VenueStatus: "trading", BaseAsset: "HYPE", QuoteAsset: "USDC", BuilderDEX: "xyz",
		Category: "crypto", VenueAssetID: 10107, SizeDecimals: u8(2),
		BaseAssetPrecision: u8(8), QuoteAssetPrecision: u8(6), QuantityStep: "0.01",
		PriceTick: "0.001", MinNotional: "10", MaxLeverage: 5, OnlyIsolated: true,
		MarginMode: "strictIsolated", BaseTokenIndex: u32(150), QuoteTokenIndex: u32(0),
		BaseWeiDecimals: u8(8), QuoteWeiDecimals: u8(8), ObservedAtUnixMs: 1_761_782_400_000,
	}
}

func minimalCEXMarketFacts() CEXMarketFacts {
	return CEXMarketFacts{
		ExchangeID: CEXExchangeIDBinance, MarketTypeID: CEXMarketTypeIDSpot,
		Label: "BTCUSDT", SymbolID: 42, VenueCoin: "BTCUSDT",
		Kind: CEXMarketKindBinanceSpot, Status: CEXMarketStatusTrading,
		BaseAsset: "BTC", QuoteAsset: "USDT", ObservedAtUnixMs: 1,
	}
}

func TestCEXMarketFactsCBORRoundTrip(t *testing.T) {
	t.Parallel()

	for name, facts := range map[string]CEXMarketFacts{
		"full":    fullCEXMarketFacts(),
		"minimal": minimalCEXMarketFacts(),
	} {
		raw, err := cbor.Marshal(facts)
		require.NoError(t, err, name)

		var decoded CEXMarketFacts
		require.NoError(t, cbor.Unmarshal(raw, &decoded), name)
		require.Equal(t, facts, decoded, name)
	}
}

// The record is a frozen wire contract; these bytes must never change.
func TestCEXMarketFactsCBORGolden(t *testing.T) {
	t.Parallel()

	raw, err := cbor.Marshal(minimalCEXMarketFacts())
	require.NoError(t, err)
	require.Equal(t, cexMarketFactsMinimalGolden, hex.EncodeToString(raw))
}

// {1:3, 2:1, 3:"BTCUSDT", 4:42, 6:"BTCUSDT", 7:4, 8:1, 10:"BTC", 11:"USDT", 28:1}
const cexMarketFactsMinimalGolden = "aa0103020103674254435553445404182a066742544355534454070408010a634254430b6455534454181c01"

func TestCEXMarketFactsContainerTags(t *testing.T) {
	t.Parallel()

	eventField := mustStructField(t, reflect.TypeFor[Event](), "CEXMarketFacts")
	require.Equal(t, "12,keyasint,omitempty", eventField.Tag.Get("cbor"))
	require.Equal(t, "cexMarketFacts,omitempty", eventField.Tag.Get("json"))
	require.Equal(t, reflect.TypeFor[[]CEXMarketFacts](), eventField.Type)

	batchType := reflect.TypeFor[Batch[testAllMidsTx, testAllMidsReceipt]]()
	batchField := mustStructField(t, batchType, "CEXMarketFacts")
	require.Equal(t, "11,keyasint,omitempty", batchField.Tag.Get("cbor"))
	require.Equal(t, reflect.TypeFor[[]CEXMarketFacts](), batchField.Type)
}

// preFactsEvent and preFactsBatch are the Event and Batch shapes before
// market facts were added.
type preFactsEvent struct {
	Base                    BaseEvent                `cbor:"1,keyasint"`
	CreationTime            uint64                   `cbor:"2,keyasint"`
	PrevEpochHash           *[32]byte                `cbor:"3,keyasint"`
	TxPool                  []AppchainTxPoolBatch    `cbor:"4,keyasint"`
	Appchains               []Checkpoint             `cbor:"5,keyasint"`
	BlockVotes              []ExternalBlock          `cbor:"6,keyasint"`
	Signature               [64]byte                 `cbor:"7,keyasint"`
	CEXOrderBookRefs        []CEXOrderBookRef        `cbor:"8,keyasint"`
	HyperliquidAllMidsRefs  []HyperliquidAllMidsRef  `cbor:"9,keyasint"`
	CEXMarketTradeBatchRefs []CEXMarketTradeBatchRef `cbor:"10,keyasint"`
	CEXCandleBatchRefs      []CEXCandleBatchRef      `cbor:"11,keyasint"`
}

type preFactsBatch struct {
	Atropos                 [32]byte                 `cbor:"1,keyasint"`
	Transactions            []testAllMidsTx          `cbor:"2,keyasint"`
	ExternalBlocks          []*ExternalBlock         `cbor:"3,keyasint"`
	Checkpoints             []*Checkpoint            `cbor:"4,keyasint"`
	EndOffset               int64                    `cbor:"5,keyasint"`
	TxEndOffset             int64                    `cbor:"6,keyasint"`
	CEXOrderBookRefs        []CEXOrderBookRef        `cbor:"7,keyasint"`
	HyperliquidAllMidsRefs  []HyperliquidAllMidsRef  `cbor:"8,keyasint"`
	CEXMarketTradeBatchRefs []CEXMarketTradeBatchRef `cbor:"9,keyasint"`
	CEXCandleBatchRefs      []CEXCandleBatchRef      `cbor:"10,keyasint"`
}

func TestCEXMarketFactsAbsentKeepsPreFactsEncoding(t *testing.T) {
	t.Parallel()

	obRefs := []CEXOrderBookRef{{ExchangeID: 1, MarketTypeID: 2, SymbolID: 7, FetchedAt: 11}}
	candles := []CEXCandleBatchRef{{ExchangeID: 1, MarketTypeID: 2, SymbolID: 7, BatchID: 3}}
	block := ExternalBlock{ChainID: 1, BlockNumber: 2, BlockHash: [32]byte{3}}

	newEvent, err := cbor.Marshal(Event{
		Base: BaseEvent{Epoch: 4, Creator: 5}, CreationTime: 6,
		BlockVotes: []ExternalBlock{block}, CEXOrderBookRefs: obRefs, CEXCandleBatchRefs: candles,
	})
	require.NoError(t, err)
	oldEvent, err := cbor.Marshal(preFactsEvent{
		Base: BaseEvent{Epoch: 4, Creator: 5}, CreationTime: 6,
		BlockVotes: []ExternalBlock{block}, CEXOrderBookRefs: obRefs, CEXCandleBatchRefs: candles,
	})
	require.NoError(t, err)
	require.Equal(t, oldEvent, newEvent)

	newBatch, err := cbor.Marshal(Batch[testAllMidsTx, testAllMidsReceipt]{
		Atropos: [32]byte{9}, ExternalBlocks: []*ExternalBlock{&block}, EndOffset: 8,
		CEXOrderBookRefs: obRefs, CEXCandleBatchRefs: candles,
	})
	require.NoError(t, err)
	oldBatch, err := cbor.Marshal(preFactsBatch{
		Atropos: [32]byte{9}, ExternalBlocks: []*ExternalBlock{&block}, EndOffset: 8,
		CEXOrderBookRefs: obRefs, CEXCandleBatchRefs: candles,
	})
	require.NoError(t, err)
	require.Equal(t, oldBatch, newBatch)

	var decodedEvent Event
	require.NoError(t, cbor.Unmarshal(oldEvent, &decodedEvent))
	require.Nil(t, decodedEvent.CEXMarketFacts)
	require.Equal(t, obRefs, decodedEvent.CEXOrderBookRefs)

	var decodedBatch Batch[testAllMidsTx, testAllMidsReceipt]
	require.NoError(t, cbor.Unmarshal(oldBatch, &decodedBatch))
	require.Nil(t, decodedBatch.CEXMarketFacts)
}

func TestCEXMarketFactsTravelOnEventAndBatch(t *testing.T) {
	t.Parallel()

	facts := []CEXMarketFacts{fullCEXMarketFacts(), minimalCEXMarketFacts()}

	raw, err := Event{CEXMarketFacts: facts}.Bytes()
	require.NoError(t, err)

	var event Event
	require.NoError(t, cbor.Unmarshal(raw, &event))
	require.Equal(t, facts, event.CEXMarketFacts)

	// An old reader ignores the new key.
	var old preFactsEvent
	require.NoError(t, cbor.Unmarshal(raw, &old))

	raw, err = cbor.Marshal(Batch[testAllMidsTx, testAllMidsReceipt]{CEXMarketFacts: facts})
	require.NoError(t, err)

	var batch Batch[testAllMidsTx, testAllMidsReceipt]
	require.NoError(t, cbor.Unmarshal(raw, &batch))
	require.Equal(t, facts, batch.CEXMarketFacts)
}
