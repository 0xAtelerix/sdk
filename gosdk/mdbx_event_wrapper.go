package gosdk

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog"

	"github.com/0xAtelerix/sdk/gosdk/types"
	"github.com/0xAtelerix/sdk/gosdk/utility"
)

type MdbxEventStreamWrapper[appTx types.AppTransaction] struct {
	eventReader *EventReader
	txReader    kv.RoDB
	chainID     uint32
	logger      *zerolog.Logger
}

type EventStreamWrapperConstructor[appTx types.AppTransaction] func(eventsPath string, chainID uint32, eventStartPos int64, txBatchDB kv.RoDB, logger *zerolog.Logger) (*MdbxEventStreamWrapper[appTx], error)

func NewMdbxEventStreamWrapper[appTx types.AppTransaction](eventsPath string, chainID uint32, eventStartPos int64, txBatchDB kv.RoDB, logger *zerolog.Logger) (*MdbxEventStreamWrapper[appTx], error) {
	eventReader, err := NewEventReader(eventsPath, eventStartPos)
	if err != nil {
		return nil, fmt.Errorf("failed to create event reader: %w", err)
	}

	return &MdbxEventStreamWrapper[appTx]{
		eventReader: eventReader,
		txReader:    txBatchDB,
		chainID:     chainID,
		logger:      logger,
	}, nil
}

type Streamer[appTx types.AppTransaction] interface {
	GetNewBatchesBlocking(ctx context.Context, limit int) ([]types.Batch[appTx], error)
	Close()
}

func (ews *MdbxEventStreamWrapper[appTx]) GetNewBatchesBlocking(ctx context.Context, limit int) ([]types.Batch[appTx], error) {
	eventBatches, err := ews.eventReader.GetNewBatchesBlocking(ctx, limit)
	if err != nil {
		return nil, err
	}

	ews.logger.Debug().Int("batches", len(eventBatches)).Msg("got new batches")
	var result []types.Batch[appTx]

	for _, eventBatch := range eventBatches {
		ews.logger.Debug().Hex("atropos", eventBatch.Atropos[:]).Int("events", len(eventBatch.Events)).Msg("Processing event batch")

		txBatches := map[[32]byte][][]byte{}
		// Список нужных транзакционных батчей
		type txRef struct {
			eventID   [32]byte
			batchHash [32]byte
		}
		var expectedTxBatches []txRef

		for _, rawEvent := range eventBatch.Events {
			var evt types.Event
			if err := json.Unmarshal(rawEvent, &evt); err != nil {
				return nil, fmt.Errorf("failed to decode event: %w", err)
			}

			for _, batch := range evt.TxPool {
				if batch.ChainID != uint64(ews.chainID) {
					continue
				}
				txBatches[batch.Hash] = [][]byte{}
				expectedTxBatches = append(expectedTxBatches, txRef{
					eventID:   evt.Base.ID,
					batchHash: batch.Hash,
				})
			}
		}
		ews.logger.Debug().Hex("atropos", eventBatch.Atropos[:]).Int("expected batches", len(expectedTxBatches)).Int("txBatches", len(txBatches)).Msg("expectedTxBatches")

		for numOfFound := 0; numOfFound < len(txBatches); {
			if numOfFound != 0 {
				time.Sleep(time.Millisecond * 50)

				s := []string{}
				for i := range txBatches {
					s = append(s, hex.EncodeToString(i[:]))
				}
				ews.logger.Debug().Int("numOfFound", numOfFound).Int("len(txBatches)", len(txBatches)).Strs("batches", s).Msg("timed out waiting for batches")
			}
			err := func() error {
				tx, err := ews.txReader.BeginRo(context.TODO())
				if err != nil {
					return err
				}
				defer tx.Rollback()

				for hsh, txBatch := range txBatches {
					if len(txBatch) > 0 {
						continue
					}

					val, err := tx.GetOne(TxBuckets, hsh[:])
					if err != nil {
						return err
					}
					if len(val) == 0 {
						continue
					}
					txs, err := utility.Unflatten(val)
					if err != nil {
						return err
					}
					txBatches[hsh] = txs
					ews.logger.Debug().Str("hash", hex.EncodeToString(hsh[:])).Msg("found tx batch")
					numOfFound++
				}
				return nil
			}()
			if err != nil {
				ews.logger.Error().Err(err).Msg("got tx batches from mdbx")
				return nil, err
			}
		}
		ews.logger.Debug().Int("expectedTxBatches", len(expectedTxBatches)).Msg("got tx batches from mdbx")

		var allParsedTxs []appTx
		for _, ref := range expectedTxBatches {
			txsRaw, ok := txBatches[ref.batchHash]
			if !ok {
				return nil, fmt.Errorf("missing tx batch for %x", ref.batchHash[:4])
			}

			for _, rawTx := range txsRaw {
				var tx appTx
				if err := json.Unmarshal(rawTx, &tx); err != nil {
					ews.logger.Error().Err(err).Str("json", string(rawTx)).Msg("failed to unmarshal tx")
					return nil, fmt.Errorf("failed to unmarshal tx: %w", err)
				}
				allParsedTxs = append(allParsedTxs, tx)
			}
		}

		result = append(result, types.Batch[appTx]{
			Atropos:      eventBatch.Atropos,
			Transactions: allParsedTxs,
			EndOffset:    eventBatch.EndOffset,
		})
	}

	return result, nil
}

func (ews *MdbxEventStreamWrapper[appTx]) Close() {
	ews.eventReader.Close()
	ews.txReader.Close()
}
