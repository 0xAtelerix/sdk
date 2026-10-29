package cexvenue

import (
	"strings"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// Builder perp order asset id: 100000 + perpDexs index * 10000 + meta index.
// A core perp's asset id is its meta index.
const (
	hlBuilderAssetBase = 100000
	hlBuilderDexStride = 10000
)

type hlPerpAsset struct {
	Name         string `json:"name"`
	SzDecimals   uint8  `json:"szDecimals"`
	MaxLeverage  uint32 `json:"maxLeverage"`
	OnlyIsolated bool   `json:"onlyIsolated"`
	IsDelisted   bool   `json:"isDelisted"`
	MarginMode   string `json:"marginMode"`
}

type hlPerpMeta struct {
	Universe        []hlPerpAsset `json:"universe"`
	CollateralToken uint32        `json:"collateralToken"`
}

// hlPerpFacts returns a perp's facts and its label base: the coin for the core
// dex, or the upper-case dex name plus the coin for a builder dex. The base
// asset is the label base in upper case (kPEPE has base asset KPEPE).
func hlPerpFacts(
	dex string,
	dexIndex, index int,
	asset *hlPerpAsset,
) (apptypes.CEXMarketFacts, string) {
	f := apptypes.CEXMarketFacts{
		MarketTypeID: MarketTypePerp,
		VenueCoin:    asset.Name,
		Kind:         apptypes.CEXMarketKindHyperliquidCorePerp,
		Status:       statusTrading,
		VenueAssetID: uint32(index),
		SizeDecimals: &asset.SzDecimals,
		MaxLeverage:  asset.MaxLeverage,
		OnlyIsolated: asset.OnlyIsolated,
		MarginMode:   asset.MarginMode,
	}
	if asset.IsDelisted {
		f.Status, f.VenueStatus = statusGone, hyperliquidDelisted
	}

	base := asset.Name

	if dex != "" {
		f.Kind, f.BuilderDEX = apptypes.CEXMarketKindHyperliquidBuilder, dex
		f.VenueAssetID = uint32(hlBuilderAssetBase + dexIndex*hlBuilderDexStride + index)
		// A builder coin is "<dex>:<coin>"; any other name keeps its colon
		// and fails the label rule.
		base = strings.ToUpper(dex) + strings.TrimPrefix(asset.Name, dex+":")
	}

	f.BaseAsset = strings.ToUpper(base)

	return f, base
}
