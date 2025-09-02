package gosdk

import (
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

type StateTransitionInterface[appTx apptypes.AppTransaction] interface {
	ProcessBatch(batch apptypes.Batch[appTx], tx kv.RwTx) ([]apptypes.ExternalTransaction, error)
}

type BatchProcesser[appTx apptypes.AppTransaction] struct {
	StateTransitionSimplified
}

func (b BatchProcesser[appTx]) ProcessBatch(
	batch apptypes.Batch[appTx],
	dbtx kv.RwTx,
) ([]apptypes.ExternalTransaction, error) {
	var extTxs []apptypes.ExternalTransaction

	for _, externalBlock := range batch.ExternalBlocks {
		// todo склоняестя ли наш вариант в сторону жесткого космос, где сильно ограничена модификация клиента?
		ext, err := b.ProcessBlock(externalBlock, dbtx)
		if err != nil {
			return nil, err
		}

		extTxs = append(extTxs, ext...)
	}

	for _, tx := range batch.Transactions {
		ext, err := tx.Process(dbtx)
		if err != nil {
			return nil, err
		}

		extTxs = append(extTxs, ext...)
	}

	return extTxs, nil
}

type StateTransitionSimplified interface {
	ProcessBlock(
		block apptypes.ExternalBlock,
		tx kv.RwTx,
	) ([]apptypes.ExternalTransaction, error) // external blocks
}
