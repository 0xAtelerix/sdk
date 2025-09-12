package gosdk

import (
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

type StateTransitionInterface[appTx apptypes.AppTransaction[R], R apptypes.Receipt] interface {
	ProcessBatch(
		batch apptypes.Batch[appTx, R],
		tx kv.RwTx,
	) ([]R, []apptypes.ExternalTransaction, error)
}

type BatchProcesser[appTx apptypes.AppTransaction[R], R apptypes.Receipt] struct {
	StateTransitionSimplified
}

func NewBatchProcesser[appTx apptypes.AppTransaction[R], R apptypes.Receipt](
	s StateTransitionSimplified,
) *BatchProcesser[appTx, R] {
	return &BatchProcesser[appTx, R]{
		StateTransitionSimplified: s,
	}
}

func (b BatchProcesser[appTx, R]) ProcessBatch(
	batch apptypes.Batch[appTx, R],
	dbtx kv.RwTx,
) ([]R, []apptypes.ExternalTransaction, error) {
	var extTxs []apptypes.ExternalTransaction

	for _, externalBlock := range batch.ExternalBlocks {
		// todo склоняестя ли наш вариант в сторону жесткого космос, где сильно ограничена модификация клиента?

		//TODO: filter blocks with subscription
		//TODO: get external blocks

		ext, err := b.ProcessBlock(*externalBlock, dbtx)
		if err != nil {
			return nil, nil, err
		}

		extTxs = append(extTxs, ext...)
	}

	receipts := make([]R, len(batch.Transactions))

	for i, tx := range batch.Transactions {
		res, ext, err := tx.Process(dbtx)
		if err != nil {
			return nil, nil, err
		}

		extTxs = append(extTxs, ext...)
		receipts[i] = res
	}

	return receipts, extTxs, nil
}

type StateTransitionSimplified interface {
	ProcessBlock(
		block apptypes.ExternalBlock,
		tx kv.RwTx,
	) ([]apptypes.ExternalTransaction, error) // external blocks
}
