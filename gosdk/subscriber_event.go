package gosdk

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/library"
	"github.com/0xAtelerix/sdk/gosdk/library/tokens"
)

// AddEVMEvent registers (ABI,eventName) into the subscriber's registry,
func AddEVMEvent[T any](
	ctx context.Context,
	s *Subscriber,
	event *library.EVMEvent,
	fn func(tokens.Event[T], kv.RwTx),
	tx kv.RwDB,
) (sig common.Hash, err error) {
	err = tx.Update(ctx, func(tx kv.RwTx) error {
		var txErr error

		sig, txErr = tokens.RegisterEvent[T](s.EVMEventRegistry, event.ABI, event.EventName)
		if txErr != nil {
			return txErr
		}

		s.SubscribeEthContract(event.ChainID, event.Contract, NewEVMHandler(event.EventName, fn))

		eventData, txErr := cbor.Marshal(event)
		if txErr != nil {
			return txErr
		}

		eventKey := EVMEventKey(event.ChainID, event.Contract, event.EventName)

		return tx.Put(SubscriptionEventLibraryBucket, eventKey, eventData)
	})
	if err != nil {
		return common.Hash{}, err
	}

	return sig, nil
}

func addEVMEvent[T any](
	s *Subscriber,
	event *library.EVMEvent,
	h func(tokens.Event[T], kv.RwTx),
) (sig common.Hash, err error) {
	sig, err = tokens.RegisterEvent[T](s.EVMEventRegistry, event.ABI, event.EventName)
	if err != nil {
		return common.Hash{}, err
	}

	if s.evmHandlers == nil {
		s.evmHandlers = make(map[string]AppEventHandler)
	}

	if h == nil {
		return
	}

	s.evmHandlers[event.EventName] = NewEVMHandler(event.EventName, h)

	return sig, nil
}

// should be called by a user on application init stage for all events. Events and handlers are in 1-1 relations.
func LoadEVMEvent[T any](
	ctx context.Context,
	s *Subscriber,
	chainID apptypes.ChainType,
	contract library.EthereumAddress,
	eventName string,
	tx kv.RoDB,
	handler func(tokens.Event[T], kv.RwTx),
) error {
	roTx, err := tx.BeginRo(ctx)
	if err != nil {
		return err
	}

	defer roTx.Rollback()

	evKey := EVMEventKey(chainID, contract, eventName)

	evData, err := roTx.GetOne(SubscriptionEventLibraryBucket, evKey)
	if err != nil {
		return err
	}

	var ev library.EVMEvent

	err = cbor.Unmarshal(evData, &ev)
	if err != nil {
		return err
	}

	_, err = addEVMEvent(s, &ev, handler)
	if err != nil {
		return err
	}

	return nil
}
