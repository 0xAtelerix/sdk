package gosdk

import (
	"context"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog/log"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

type StateTransitionInterface[appTx apptypes.AppTransaction[R], R apptypes.Receipt] interface {
	ProcessBatch(
		ctx context.Context,
		batch apptypes.Batch[appTx, R],
		tx kv.RwTx,
	) ([]R, []apptypes.ExternalTransaction, error)
}

type BatchProcesser[appTx apptypes.AppTransaction[R], R apptypes.Receipt] struct {
	StateTransitionSimplified

	MultiChain *MultichainStateAccess
	Subscriber *Subscriber
}

func NewBatchProcesser[appTx apptypes.AppTransaction[R], R apptypes.Receipt](
	s StateTransitionSimplified,
	m *MultichainStateAccess,
	sb *Subscriber,
) *BatchProcesser[appTx, R] {
	return &BatchProcesser[appTx, R]{
		StateTransitionSimplified: s,
		MultiChain:                m,
		Subscriber:                sb,
	}
}

func (b BatchProcesser[appTx, R]) ProcessBatch(
	ctx context.Context,
	batch apptypes.Batch[appTx, R],
	dbtx kv.RwTx,
) ([]R, []apptypes.ExternalTransaction, error) {
	var extTxs []apptypes.ExternalTransaction

	logger := log.Ctx(ctx)

	receipts := make([]R, len(batch.Transactions))

	for i, tx := range batch.Transactions {
		res, ext, err := tx.Process(dbtx)
		if err != nil {
			return nil, nil, err
		}

		extTxs = append(extTxs, ext...)
		receipts[i] = res
	}

blockLoop:
	for _, blk := range batch.ExternalBlocks {
		switch {
		case IsEvmChain(apptypes.ChainType(blk.ChainID)):
			// todo склоняестя ли наш вариант в сторону жесткого космос, где сильно ограничена модификация клиента?
			ethReceipts, err := b.MultiChain.EthReceipts(ctx, *blk)
			if err != nil {
				logger.Debug().Err(err).Msg("failed to process batch external receipts")

				continue
			}

			var ok bool

			for _, rec := range ethReceipts {
				ok = b.Subscriber.IsEthSubscription(apptypes.ChainType(blk.ChainID), EthereumAddress(rec.ContractAddress))

				if ok {
					ext, err := b.ProcessBlock(*blk, dbtx)
					if err != nil {
						return nil, nil, err
					}

					extTxs = append(extTxs, ext...)

					continue blockLoop
				}
			}

		case IsSolanaChain(apptypes.ChainType(blk.ChainID)):
			block, err := b.MultiChain.SolanaBlock(ctx, *blk)
			if err != nil {
				logger.Debug().Err(err).Msg("failed to get Solana block")

				continue
			}

			for _, tx := range block.Transactions {
				for i := range tx.Transaction.Message.Header.NumRequireSignatures {
					pub := tx.Transaction.Message.Accounts[i]
					ok := b.Subscriber.IsSolanaSubscription(apptypes.ChainType(blk.ChainID), SolanaAddress(pub))

					if ok {
						ext, err := b.ProcessBlock(*blk, dbtx)
						if err != nil {
							return nil, nil, err
						}

						extTxs = append(extTxs, ext...)

						continue blockLoop
					}
				}
			}

		default:
			logger.Error().Uint64("chainID", blk.ChainID).Msg("Unknown chain type")
		}
	}

	/*
		for _, checkpoint := range batch.Checkpoints {
			checkpoint.ExternalTransactionsRoot
		}
	*/

	return receipts, extTxs, nil
}

type StateTransitionSimplified interface {
	ProcessBlock(
		block apptypes.ExternalBlock,
		tx kv.RwTx,
	) ([]apptypes.ExternalTransaction, error) // external blocks
}
