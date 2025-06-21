package gosdk

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/0xAtelerix/sdk/gosdk/types"
	"github.com/0xAtelerix/sdk/gosdk/utility"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog"
	"time"
)

type MdbxEventStreamWrapper[appTx types.AppTransaction] struct {
	eventReader *EventReader
	txReader    kv.RoDB
	chainID     uint32
	logger      *zerolog.Logger
}

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

func (ews *MdbxEventStreamWrapper[appTx]) GetNewBatchesBlocking(limit int) ([]types.Batch[appTx], error) {
	eventBatches, err := ews.eventReader.GetNewBatchesBlocking(limit)
	if err != nil {
		return nil, err
	}

	ews.logger.Debug().Int("batches", len(eventBatches)).Msg("got new batches")
	var result []types.Batch[appTx]

	for _, eventBatch := range eventBatches {
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
		ews.logger.Debug().Int("expected batches", len(expectedTxBatches)).Int("txBatches", len(txBatches)).Msg("expectedTxBatches")

		for numOfFound := 0; numOfFound < len(txBatches); {
			if numOfFound != 0 {
				time.Sleep(time.Millisecond * 50)
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
					numOfFound++
				}
				return nil
			}()
			if err != nil {
				return nil, err
			}
		}
		ews.logger.Debug().Int("expectedTxBatches", len(expectedTxBatches)).Msg("got tx batches from mdbx")

		// Теперь собираем результат
		for _, ref := range expectedTxBatches {
			txsRaw, ok := txBatches[ref.batchHash]
			if !ok {
				return nil, fmt.Errorf("missing tx batch for %x", ref.batchHash[:4])
			}

			var parsedTxs []appTx
			for _, rawTx := range txsRaw {
				var tx appTx
				if err := json.Unmarshal(rawTx, &tx); err != nil {
					return nil, fmt.Errorf("failed to unmarshal tx: %w", err)
				}
				parsedTxs = append(parsedTxs, tx)
			}

			result = append(result, types.Batch[appTx]{
				Transactions: parsedTxs,
				// берем EndOffset из ивент батча — txBatch тоже можно пробрасывать
				EndOffset: eventBatch.EndOffset,
			})
		}
	}

	return result, nil
}

func (ews *MdbxEventStreamWrapper[appTx]) Close() {
	ews.eventReader.Close()
	ews.txReader.Close()
}
