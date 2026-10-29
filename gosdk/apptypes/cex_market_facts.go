package apptypes

// Market facts are the latest venue description of one market. Pelacli embeds
// them in events; an appchain stores the latest record per market key and
// derives its own catalog from them. Every record is the whole latest value,
// so a lost, repeated or reordered record is repaired by the next one.

// CEXDerivedSymbolIDBase is the first id of the derived range. Frozen v9 ids
// are below it; ids for every other label are computed by SymbolID.
const CEXDerivedSymbolIDBase CEXSymbolID = 0x80000000

// CEXMarketFactsMaxRecordsPerBlock (R) caps the records a producer sends per
// block, sweep and change records together.
const CEXMarketFactsMaxRecordsPerBlock = 64

// CEXVenueFaultMaxRemovalPercent (X): a successful venue response that would
// take more than this share of a venue's trading markets out of trading is a
// venue fault and is dropped whole.
const CEXVenueFaultMaxRemovalPercent = 20

// CEXMarketStatus is a market's status as the latest venue report gives it.
type CEXMarketStatus uint8

const (
	CEXMarketStatusUnknown           CEXMarketStatus = 0
	CEXMarketStatusTrading           CEXMarketStatus = 1
	CEXMarketStatusHalted            CEXMarketStatus = 2
	CEXMarketStatusSuspectedDelisted CEXMarketStatus = 3
	CEXMarketStatusDelisted          CEXMarketStatus = 4
)

// CEXMarketKind is the venue product a market belongs to.
type CEXMarketKind uint8

const (
	CEXMarketKindUnknown              CEXMarketKind = 0
	CEXMarketKindHyperliquidCorePerp  CEXMarketKind = 1
	CEXMarketKindHyperliquidBuilder   CEXMarketKind = 2
	CEXMarketKindHyperliquidSpot      CEXMarketKind = 3
	CEXMarketKindBinanceSpot          CEXMarketKind = 4
	CEXMarketKindBinanceUSDMPerpetual CEXMarketKind = 5
	CEXMarketKindBinanceTradFiPerp    CEXMarketKind = 6
	CEXMarketKindBinanceOtherFutures  CEXMarketKind = 7
	CEXMarketKindMEXCSpot             CEXMarketKind = 8
)

// CEXMarketFacts is one market's latest facts. The record key is
// (ExchangeID, MarketTypeID, Network, Label).
type CEXMarketFacts struct {
	ExchangeID   CEXExchangeID   `cbor:"1,keyasint"  json:"exchange_id"`
	MarketTypeID CEXMarketTypeID `cbor:"2,keyasint"  json:"market_type_id"`
	Label        string          `cbor:"3,keyasint"  json:"label"`
	// SymbolID is SymbolID(ExchangeID, Label); receivers recompute it and
	// drop a record whose id disagrees.
	SymbolID CEXSymbolID `cbor:"4,keyasint" json:"symbol_id"`
	// Network is "mainnet" or "testnet" for Hyperliquid and empty elsewhere.
	Network string `cbor:"5,keyasint,omitempty" json:"network,omitempty"`
	// VenueCoin is the venue's own name: the HL coin ("BTC", "xyz:TSLA",
	// "@107") or the venue symbol elsewhere.
	VenueCoin   string          `cbor:"6,keyasint"            json:"venue_coin"`
	Kind        CEXMarketKind   `cbor:"7,keyasint"            json:"kind"`
	Status      CEXMarketStatus `cbor:"8,keyasint"            json:"status"`
	VenueStatus string          `cbor:"9,keyasint,omitempty"  json:"venue_status,omitempty"`
	BaseAsset   string          `cbor:"10,keyasint"           json:"base_asset"`
	QuoteAsset  string          `cbor:"11,keyasint"           json:"quote_asset"`
	BuilderDEX  string          `cbor:"12,keyasint,omitempty" json:"builder_dex,omitempty"`
	// Category is the HL perp category or the Binance underlyingType.
	Category            string  `cbor:"13,keyasint,omitempty" json:"category,omitempty"`
	VenueAssetID        uint32  `cbor:"14,keyasint,omitempty" json:"venue_asset_id,omitempty"`
	SizeDecimals        *uint8  `cbor:"15,keyasint,omitempty" json:"size_decimals,omitempty"`
	BaseAssetPrecision  *uint8  `cbor:"16,keyasint,omitempty" json:"base_asset_precision,omitempty"`
	QuoteAssetPrecision *uint8  `cbor:"17,keyasint,omitempty" json:"quote_asset_precision,omitempty"`
	QuantityStep        string  `cbor:"18,keyasint,omitempty" json:"quantity_step,omitempty"`
	PriceTick           string  `cbor:"19,keyasint,omitempty" json:"price_tick,omitempty"`
	MinNotional         string  `cbor:"20,keyasint,omitempty" json:"min_notional,omitempty"`
	MaxLeverage         uint32  `cbor:"21,keyasint,omitempty" json:"max_leverage,omitempty"`
	OnlyIsolated        bool    `cbor:"22,keyasint,omitempty" json:"only_isolated,omitempty"`
	MarginMode          string  `cbor:"23,keyasint,omitempty" json:"margin_mode,omitempty"`
	BaseTokenIndex      *uint32 `cbor:"24,keyasint,omitempty" json:"base_token_index,omitempty"`
	QuoteTokenIndex     *uint32 `cbor:"25,keyasint,omitempty" json:"quote_token_index,omitempty"`
	BaseWeiDecimals     *uint8  `cbor:"26,keyasint,omitempty" json:"base_wei_decimals,omitempty"`
	QuoteWeiDecimals    *uint8  `cbor:"27,keyasint,omitempty" json:"quote_wei_decimals,omitempty"`
	ObservedAtUnixMs    int64   `cbor:"28,keyasint"           json:"observed_at_unix_ms"`
}
