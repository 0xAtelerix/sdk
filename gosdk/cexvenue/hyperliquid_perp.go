package cexvenue

import (
	"fmt"
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

type hlPerpDex struct {
	Name string `json:"name"`
}

// addHyperliquidPerps reads every dex listed by perpDexs (null is the core
// dex). A perp is quoted in its dex's collateral token.
func (c *collector) addHyperliquidPerps(in *HyperliquidInput, tokens map[uint32]hlToken) error {
	var (
		dexs     []*hlPerpDex
		category [][2]string
	)

	if err := decode("perpDexs", in.PerpDexs, &dexs); err != nil {
		return err
	}

	if in.PerpCategories != nil {
		if err := decode("perpCategories", in.PerpCategories, &category); err != nil {
			return err
		}
	}

	categories := make(map[string]string, len(category))
	for _, pair := range category {
		categories[pair[0]] = pair[1]
	}

	for dexIndex, dex := range dexs {
		name := ""
		if dex != nil {
			name = dex.Name
		}

		raw, ok := in.PerpMetas[name]
		if !ok {
			return fmt.Errorf(
				"%w: hyperliquid meta missing for dex %q",
				ErrInvalidVenueResponse,
				name,
			)
		}

		var meta hlPerpMeta
		if err := decode("meta", raw, &meta); err != nil {
			return err
		}

		quote, ok := tokens[meta.CollateralToken]
		if !ok {
			return fmt.Errorf(
				"%w: hyperliquid dex %q collateral token unknown",
				ErrInvalidVenueResponse,
				name,
			)
		}

		for i := range meta.Universe {
			f, base := hlPerpFacts(name, dexIndex, i, &meta.Universe[i])
			f.QuoteAsset, f.Category = quote.Name, categories[f.VenueCoin]
			c.add(&f, base+quote.Name)
		}
	}

	return nil
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
