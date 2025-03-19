package gosdk

import (
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/types"
)

type StateTransitionInterface[appTx types.AppTransaction] interface {
	ProcessBatch(batch types.Batch[appTx], tx kv.RwTx) ([]types.ExternalTransaction, error)
}

type BatchProcesser[appTx types.AppTransaction] struct {
	StateTransitionSimplified[appTx]
}

func (b BatchProcesser[appTx]) ProcessBatch(batch types.Batch[appTx], dbtx kv.RwTx) ([]types.ExternalTransaction, error) {
	var extTxs []types.ExternalTransaction

	for _, externalBlock := range batch.ExternalBlocks {
		// todo склоняестя ли наш вариант в сторону жесткого космос, где сильно ограничена модификация клиента?
		ext, err := b.StateTransitionSimplified.ProcessBlock(externalBlock, dbtx)
		if err != nil {
			return nil, err
		}

		extTxs = append(extTxs, ext...)
	}

	for _, tx := range batch.Transactions {
		ext, err := b.StateTransitionSimplified.ProcessTX(tx, dbtx)
		if err != nil {
			return nil, err
		}

		extTxs = append(extTxs, ext...)
	}
	return extTxs, nil
}

type StateTransitionSimplified[appTx types.AppTransaction] interface {
	ProcessTX(tx appTx, dbtx kv.RwTx) ([]types.ExternalTransaction, error)
	ProcessBlock(block types.ExternalBlock, tx kv.RwTx) ([]types.ExternalTransaction, error) // тут внешние блоки
}
