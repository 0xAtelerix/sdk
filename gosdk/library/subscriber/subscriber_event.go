package subscriber

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/library"
	"github.com/0xAtelerix/sdk/gosdk/library/tokens"
)

// AddEVMEvent registers (ABI,eventName)->kind into the subscriber's registry,
// and attaches a type-safe handler for that kind.
func AddEVMEvent[T any](
	s *Subscriber,
	chainID apptypes.ChainType,
	contract library.EthereumAddress,
	a abi.ABI,
	eventName string,
	kind string,
	fn func(tokens.Event[T], kv.RwTx),
) (sig common.Hash, err error) {
	// 1) register event → (sig, filter)
	sig, filter, err := tokens.RegisterEvent[T](s.EVMEventRegistry, a, eventName, kind)
	if err != nil {
		return common.Hash{}, err
	}

	// 2) attach handler (type-erased wrapper)
	s.SubscribeEthContract(chainID, contract, NewEVMHandler(kind, filter, fn))

	return sig, nil
}
