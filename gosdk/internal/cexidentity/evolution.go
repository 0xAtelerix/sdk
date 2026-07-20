// Package cexidentity validates source-controlled CEX identity registry evolution.
package cexidentity

import (
	"errors"
	"fmt"
	"math"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

var (
	errInitialVersion  = errors.New("initial cex identity registry version must be 1")
	errIdentityChanged = errors.New("existing cex identity assignment changed")
	errVersionDrift    = errors.New("cex identity registry version does not match evolution")
)

type symbolKey struct {
	exchangeID   apptypes.CEXExchangeID
	marketTypeID apptypes.CEXMarketTypeID
	symbolID     apptypes.CEXSymbolID
}

// ValidateInitial validates the first source-controlled registry document.
func ValidateInitial(current apptypes.OrderBookIdentityJSON) error {
	if _, err := apptypes.NewOrderBookIDRegistryFromJSON(current); err != nil {
		return fmt.Errorf("validate initial cex identity registry: %w", err)
	}

	if current.Version != 1 {
		return fmt.Errorf("%w: got %d", errInitialVersion, current.Version)
	}

	return nil
}

// ValidateEvolution validates one committed registry document against its base.
func ValidateEvolution(previous, current apptypes.OrderBookIdentityJSON) error {
	if _, err := apptypes.NewOrderBookIDRegistryFromJSON(previous); err != nil {
		return fmt.Errorf("validate base cex identity registry: %w", err)
	}

	if _, err := apptypes.NewOrderBookIDRegistryFromJSON(current); err != nil {
		return fmt.Errorf("validate current cex identity registry: %w", err)
	}

	if err := validateExistingAssignments(previous, current); err != nil {
		return err
	}

	hasAdditions := len(current.Exchanges) > len(previous.Exchanges) ||
		len(current.MarketTypes) > len(previous.MarketTypes) ||
		len(current.Symbols) > len(previous.Symbols)

	expectedVersion := previous.Version
	if hasAdditions {
		if previous.Version == math.MaxUint32 {
			return fmt.Errorf("%w: base version cannot be incremented", errVersionDrift)
		}

		expectedVersion++
	}

	if current.Version != expectedVersion {
		return fmt.Errorf(
			"%w: base=%d current=%d additions=%t expected=%d",
			errVersionDrift,
			previous.Version,
			current.Version,
			hasAdditions,
			expectedVersion,
		)
	}

	return nil
}

func validateExistingAssignments(previous, current apptypes.OrderBookIdentityJSON) error {
	currentExchanges := make(
		map[apptypes.CEXExchangeID]apptypes.OrderBookExchangeJSON,
		len(current.Exchanges),
	)
	for _, exchange := range current.Exchanges {
		currentExchanges[exchange.ID] = exchange
	}

	for _, exchange := range previous.Exchanges {
		currentExchange, ok := currentExchanges[exchange.ID]
		if !ok || currentExchange != exchange {
			return fmt.Errorf("%w: exchange_id=%d", errIdentityChanged, exchange.ID)
		}
	}

	currentMarkets := make(
		map[apptypes.CEXMarketTypeID]apptypes.OrderBookMarketJSON,
		len(current.MarketTypes),
	)
	for _, market := range current.MarketTypes {
		currentMarkets[market.ID] = market
	}

	for _, market := range previous.MarketTypes {
		if currentMarket, ok := currentMarkets[market.ID]; !ok || currentMarket != market {
			return fmt.Errorf("%w: market_type_id=%d", errIdentityChanged, market.ID)
		}
	}

	return validateExistingSymbols(previous.Symbols, current.Symbols)
}

func validateExistingSymbols(
	previous []apptypes.OrderBookSymbolJSON,
	current []apptypes.OrderBookSymbolJSON,
) error {
	currentSymbols := make(map[symbolKey]apptypes.OrderBookSymbolJSON, len(current))
	for _, symbol := range current {
		currentSymbols[symbolKey{
			exchangeID:   symbol.ExchangeID,
			marketTypeID: symbol.MarketTypeID,
			symbolID:     symbol.SymbolID,
		}] = symbol
	}

	for _, symbol := range previous {
		key := symbolKey{
			exchangeID:   symbol.ExchangeID,
			marketTypeID: symbol.MarketTypeID,
			symbolID:     symbol.SymbolID,
		}
		if currentSymbol, ok := currentSymbols[key]; !ok || currentSymbol != symbol {
			return fmt.Errorf(
				"%w: exchange_id=%d market_type_id=%d symbol_id=%d",
				errIdentityChanged,
				symbol.ExchangeID,
				symbol.MarketTypeID,
				symbol.SymbolID,
			)
		}
	}

	return nil
}
