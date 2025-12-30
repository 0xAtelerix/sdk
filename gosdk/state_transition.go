package gosdk

import (
	"context"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog/log"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/library"
	"github.com/0xAtelerix/sdk/gosdk/library/tokens"
)

type BatchProcessor[appTx apptypes.AppTransaction[R], R apptypes.Receipt] interface {
	ProcessBatch(
		ctx context.Context,
		batch apptypes.Batch[appTx, R],
		dbtx kv.RwTx,
	) ([]R, []apptypes.ExternalTransaction, error)
}

type DefaultBatchProcessor[appTx apptypes.AppTransaction[R], R apptypes.Receipt] struct {
	extBlockProc ExternalBlockProcessor
	multichain   MultichainStateAccessor
	subscriber   *Subscriber
}

func NewDefaultBatchProcessor[appTx apptypes.AppTransaction[R], R apptypes.Receipt](
	extBlockProc ExternalBlockProcessor,
	multichain MultichainStateAccessor,
	subscriber *Subscriber,
) *DefaultBatchProcessor[appTx, R] {
	return &DefaultBatchProcessor[appTx, R]{
		extBlockProc: extBlockProc,
		multichain:   multichain,
		subscriber:   subscriber,
	}
}

// ProcessBatch processes a batch of transactions and external blocks.
// It handles:
// 1. Processing app transactions (calls tx.Process() for each)
// 2. Filtering external blocks by subscriptions (EVM/Solana address matching)
// 3. Delegating matched blocks to ExternalBlockProcessor.ProcessBlock()
func (b *DefaultBatchProcessor[appTx, R]) ProcessBatch(
	ctx context.Context,
	batch apptypes.Batch[appTx, R],
	dbtx kv.RwTx,
) ([]R, []apptypes.ExternalTransaction, error) {
	var extTxs []apptypes.ExternalTransaction

	logger := log.Ctx(ctx)

	// Process app transactions
	receipts := make([]R, len(batch.Transactions))
	for i, tx := range batch.Transactions {
		res, ext, err := tx.Process(dbtx)
		if err != nil {
			return nil, nil, err
		}

		extTxs = append(extTxs, ext...)
		receipts[i] = res
	}

	// Skip if multichain or external block processor or subscriber is not set
	if b.multichain == nil || b.extBlockProc == nil || b.subscriber == nil {
		return receipts, extTxs, nil
	}

blockLoop:
	for _, blk := range batch.ExternalBlocks {
		switch {
		case library.IsEvmChain(apptypes.ChainType(blk.ChainID)):
			evmReceipts, err := b.multichain.EVMReceipts(ctx, *blk)
			if err != nil {
				logger.Debug().Err(err).Msg("failed to get EVM receipts")

				continue
			}

			var ok bool

			for _, rec := range evmReceipts {
				for _, lg := range rec.Logs {
					emitter := library.EthereumAddress(lg.Address)
					if !b.subscriber.IsEthSubscription(apptypes.ChainType(blk.ChainID), emitter) {
						continue
					}

					events, matched, err := b.subscriber.EVMEventRegistry.HandleLog(lg, rec.TxHash)
					if err != nil {
						logger.Info().Err(err).Msg("failed to decode external events")

						continue
					}

					if !matched {
						continue
					}

					ok = true

					// dispatch by kind to handlers you stored on Subscriber
					byName := map[string][]tokens.AppEvent{}
					for _, e := range events {
						byName[e.Name()] = append(byName[e.Name()], e)
					}

					for k, evs := range byName {
						b.subscriber.Handle(k, evs, dbtx)
					}
				}

				if ok {
					// Optional for complex cases, most of the cases should be handled by Subscription.Handle
					ext, err := b.extBlockProc.ProcessBlock(*blk, dbtx)
					if err != nil {
						return nil, nil, err
					}

					extTxs = append(extTxs, ext...)

					continue blockLoop
				}
			}

		case library.IsSolanaChain(apptypes.ChainType(blk.ChainID)):
			block, err := b.multichain.SolanaBlock(ctx, *blk)
			if err != nil {
				logger.Info().Err(err).Msg("failed to get Solana block")

				continue
			}

			for _, tx := range block.Transactions {
				for i := range tx.Transaction.Message.Header.NumRequireSignatures {
					pub := tx.Transaction.Message.Accounts[i]
					ok := b.subscriber.IsSolanaSubscription(apptypes.ChainType(blk.ChainID), library.SolanaAddress(pub))

					if ok {
						ext, err := b.extBlockProc.ProcessBlock(*blk, dbtx)
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

	return receipts, extTxs, nil
}

type ExternalBlockProcessor interface {
	ProcessBlock(
		block apptypes.ExternalBlock,
		tx kv.RwTx,
	) ([]apptypes.ExternalTransaction, error)
}
