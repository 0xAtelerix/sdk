package gosdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/0xAtelerix/sdk/gosdk/types"
)

type EventStreamWrapper[appTx types.AppTransaction] struct {
	eventReader *EventReader
	txReader    *EventReader
	chainID     uint32
}

func NewEventStreamWrapper[appTx types.AppTransaction](eventsPath, txPath string, chainID uint32, eventStartPos, txStartPos int64) (*EventStreamWrapper[appTx], error) {
	eventReader, err := NewEventReader(eventsPath, eventStartPos)
	if err != nil {
		return nil, fmt.Errorf("failed to create event reader: %w", err)
	}

	txReader, err := NewEventReader(txPath, txStartPos)
	if err != nil {
		eventReader.Close()
		return nil, fmt.Errorf("failed to create tx reader: %w", err)
	}

	return &EventStreamWrapper[appTx]{
		eventReader: eventReader,
		txReader:    txReader,
		chainID:     chainID,
	}, nil
}

func (ews *EventStreamWrapper[appTx]) GetNewBatchesBlocking(limit int) ([]types.Batch[appTx], error) {
	eventBatches, err := ews.eventReader.GetNewBatchesBlocking(limit)
	if err != nil {
		return nil, err
	}

	var result []types.Batch[appTx]

	for _, eventBatch := range eventBatches {
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
				expectedTxBatches = append(expectedTxBatches, txRef{
					eventID:   evt.Base.ID,
					batchHash: batch.Hash,
				})
			}
		}

		// Считываем нужные транзакционные батчи
		type collectedTx struct {
			RawTxs    [][]byte
			EndOffset int64
		}
		collected := make(map[[32]byte]collectedTx, len(expectedTxBatches))
		for len(collected) < len(expectedTxBatches) {
			txBatches, err := ews.txReader.GetNewBatchesBlocking(len(expectedTxBatches)) // ограничим разумно
			if err != nil {
				return nil, fmt.Errorf("reading txs: %w", err)
			}

			for _, txb := range txBatches {
				for _, ref := range expectedTxBatches {
					if bytes.Equal(txb.Atropos[:], ref.eventID[:]) {
						// если совпал eventID — ок, сохраняем
						collected[ref.batchHash] = collectedTx{
							RawTxs:    txb.Events,
							EndOffset: txb.EndOffset,
						}
					}
				}
			}
		}

		// Теперь собираем результат
		for _, ref := range expectedTxBatches {
			txsRaw, ok := collected[ref.batchHash]
			if !ok {
				return nil, fmt.Errorf("missing tx batch for %x", ref.batchHash[:4])
			}

			var parsedTxs []appTx
			for _, rawTx := range txsRaw.RawTxs {
				var tx appTx
				if err := json.Unmarshal(rawTx, &tx); err != nil {
					return nil, fmt.Errorf("failed to unmarshal tx: %w", err)
				}
				parsedTxs = append(parsedTxs, tx)
			}

			result = append(result, types.Batch[appTx]{
				Transactions: parsedTxs,
				// берем EndOffset из ивент батча — txBatch тоже можно пробрасывать
				EndOffset:   eventBatch.EndOffset,
				TxEndOffset: txsRaw.EndOffset,
			})
		}
	}

	return result, nil
}

func (ews *EventStreamWrapper[appTx]) Close() {
	ews.eventReader.Close()
	ews.txReader.Close()
}
