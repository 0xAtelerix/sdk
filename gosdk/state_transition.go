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
	cexProc      CEXStreamProcessor
	multichain   MultichainStateAccessor
	subscriber   *Subscriber
}

func NewDefaultBatchProcessor[appTx apptypes.AppTransaction[R], R apptypes.Receipt](
	extBlockProc ExternalBlockProcessor,
	cexProc CEXStreamProcessor,
	multichain MultichainStateAccessor,
	subscriber *Subscriber,
) *DefaultBatchProcessor[appTx, R] {
	return &DefaultBatchProcessor[appTx, R]{
		extBlockProc: extBlockProc,
		cexProc:      cexProc,
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

	for _, blk := range batch.ExternalBlocks {
		chainID := apptypes.ChainType(blk.ChainID)

		switch {
		case library.IsEvmChain(chainID):
			evmReceipts, err := b.multichain.EVMReceipts(ctx, *blk)
			if err != nil {
				logger.Warn().Err(err).Msg("failed to get EVM receipts")

				continue
			}

			hasSubscriptions := b.subscriber.HasEthSubscriptions(chainID)

			// No subscriptions = default "listen to all", no receipts = empty block is still an event.
			// In both cases skip the receipt loop and pass the block through to ProcessBlock.
			var subFound bool
			if !hasSubscriptions || len(evmReceipts) == 0 {
				subFound = true
			} else {
				hasHandlers := b.subscriber.HasEVMHandlers()

			receiptLoop:
				for _, rec := range evmReceipts {
					for _, lg := range rec.Logs {
						if lg == nil || len(lg.Topics) == 0 {
							continue
						}

						emitter := library.EthereumAddress(lg.Address)
						topic0 := library.EthereumTopic(lg.Topics[0])

						if !b.subscriber.IsEthSubscription(chainID, emitter, topic0) {
							continue
						}

						subFound = true

						// If no handlers but subscription matched, break to skip remaining receipts and call ProcessBlock
						if !hasHandlers {
							break receiptLoop
						}

						// Process through EVMEventRegistry for handler dispatch
						events, matched, err := b.subscriber.EVMEventRegistry.HandleLog(lg, rec.TxHash)
						if err == nil && matched {
							byName := map[string][]tokens.AppEvent{}
							for _, e := range events {
								byName[e.Name()] = append(byName[e.Name()], e)
							}

							for k, evs := range byName {
								b.subscriber.Handle(k, evs, dbtx)
							}
						}
					}
				}
			}

			if subFound {
				// Optional for complex cases, most of the cases should be handled by Subscription.Handle
				ext, err := b.extBlockProc.ProcessBlock(*blk, dbtx)
				if err != nil {
					return nil, nil, err
				}

				extTxs = append(extTxs, ext...)
			}

		case library.IsSolanaChain(chainID):
			block, err := b.multichain.SolanaBlock(ctx, *blk)
			if err != nil {
				logger.Warn().Err(err).Msg("failed to get Solana block")

				continue
			}

			hasSubscriptions := b.subscriber.HasSolanaSubscriptions(chainID)

			var solSubFound bool
			if !hasSubscriptions || len(block.Transactions) == 0 {
				solSubFound = true
			} else {
				for _, tx := range block.Transactions {
					for i := range tx.Transaction.Message.Header.NumRequireSignatures {
						pub := tx.Transaction.Message.Accounts[i]
						if b.subscriber.IsSolanaSubscription(chainID, library.SolanaAddress(pub)) {
							solSubFound = true

							break
						}
					}

					if solSubFound {
						break
					}
				}
			}

			if solSubFound {
				ext, err := b.extBlockProc.ProcessBlock(*blk, dbtx)
				if err != nil {
					return nil, nil, err
				}

				extTxs = append(extTxs, ext...)
			}

		default:
			logger.Error().Uint64("chainID", blk.ChainID).Msg("Unknown chain type")
		}
	}

	// Process CEX order book refs if processor is set
	if b.cexProc != nil && len(batch.CEXOrderBookRefs) > 0 {
		cexExt, cexErr := b.cexProc.ProcessCEXStream(ctx, batch.CEXOrderBookRefs, dbtx)
		if cexErr != nil {
			logger.Error().Err(cexErr).Msg("CEX stream processing failed")
		} else {
			extTxs = append(extTxs, cexExt...)
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

type CEXStreamProcessor interface {
	ProcessCEXStream(
		ctx context.Context,
		refs []apptypes.CEXOrderBookRef,
		tx kv.RwTx,
	) ([]apptypes.ExternalTransaction, error)
}
