package apptypes

import (
	"encoding/binary"
	"hash/fnv"
)

const (
	// cexDerivedSymbolIDMask keeps the lower 31 bits of the label hash.
	cexDerivedSymbolIDMask = 0x7fffffff
	// cexSymbolIDHashSeparator separates the exchange id from the label bytes.
	cexSymbolIDHashSeparator = 0x00
	cexExchangeIDHashBytes   = 2
)

// SymbolID returns the permanent id of a market label on an exchange. A label
// in the frozen v9 table on that exchange keeps its id, whatever its market
// type. Any other label gets CEXDerivedSymbolIDBase | (FNV-1a 64 & 0x7fffffff)
// over the exchange id as 2-byte big-endian, a 0x00 byte and the label bytes.
// The market type is not an input, so spot and perp with one label share an id.
//
// The label is used exactly as given; it is not trimmed or case-folded, because
// receivers recompute the id from the label in a facts record. SymbolID returns
// 0, which no ref check accepts, for an empty label or if the embedded table
// cannot load.
func SymbolID(exchangeID CEXExchangeID, label string) CEXSymbolID {
	return DefaultOrderBookIDRegistry.SymbolID(exchangeID, label)
}

// SymbolID returns the permanent id of a label through the embedded table.
func (h DefaultOrderBookIDRegistryHandle) SymbolID(
	exchangeID CEXExchangeID,
	label string,
) CEXSymbolID {
	registry, err := h.registry()
	if err != nil {
		return 0
	}

	return registry.SymbolID(exchangeID, label)
}

// SymbolID returns the frozen id of a label in this table, or its derived id.
func (r *OrderBookIDRegistry) SymbolID(exchangeID CEXExchangeID, label string) CEXSymbolID {
	if r == nil || label == "" {
		return 0
	}

	// addOrderBookSymbol enforces one id per (exchange, label) across market
	// types, so the first candidate is the label's id.
	if frozen := r.legacyCandidates[cexLegacySymbolKey{
		exchangeID: exchangeID,
		label:      label,
	}]; len(frozen) > 0 {
		return frozen[0].SymbolID
	}

	return derivedCEXSymbolID(exchangeID, label)
}

func derivedCEXSymbolID(exchangeID CEXExchangeID, label string) CEXSymbolID {
	var prefix [cexExchangeIDHashBytes + 1]byte

	binary.BigEndian.PutUint16(prefix[:cexExchangeIDHashBytes], uint16(exchangeID))
	prefix[cexExchangeIDHashBytes] = cexSymbolIDHashSeparator

	hasher := fnv.New64a()
	_, _ = hasher.Write(prefix[:])
	_, _ = hasher.Write([]byte(label))

	return CEXDerivedSymbolIDBase | CEXSymbolID(hasher.Sum64()&cexDerivedSymbolIDMask)
}
