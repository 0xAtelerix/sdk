package gosdk

import (
	"bytes"
	"context"
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/rs/zerolog/log"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

type EventStreamWrapper[appTx apptypes.AppTransaction[R], R apptypes.Receipt] struct {
	eventReader *EventReader
	txReader    *EventReader
	chainID     uint32
}

func NewEventStreamWrapper[appTx apptypes.AppTransaction[R], R apptypes.Receipt](
	eventsPath, txPath string,
	chainID uint32,
	eventStartPos, txStartPos int64,
) (*EventStreamWrapper[appTx, R], error) {
	eventReader, err := NewEventReader(eventsPath, eventStartPos)
	if err != nil {
		return nil, fmt.Errorf("failed to create event reader: %w", err)
	}

	txReader, err := NewEventReader(txPath, txStartPos)
	if err != nil {
		closeErr := eventReader.Close()
		if closeErr != nil {
			log.Warn().Err(closeErr).Msgf("Failed to close event reader: %q", closeErr)
		}

		return nil, fmt.Errorf("failed to create tx reader: %w", err)
	}

	return &EventStreamWrapper[appTx, R]{
		eventReader: eventReader,
		txReader:    txReader,
		chainID:     chainID,
	}, nil
}

func (ews *EventStreamWrapper[appTx, R]) GetNewBatchesBlocking(
	ctx context.Context,
	limit int,
) ([]apptypes.Batch[appTx, R], error) {
	eventBatches, err := ews.eventReader.GetNewBatchesBlocking(ctx, limit)
	if err != nil {
		return nil, err
	}

	var result []apptypes.Batch[appTx, R]

	for _, eventBatch := range eventBatches {
		// Список нужных транзакционных батчей
		type txRef struct {
			eventID   [32]byte
			batchHash [32]byte
		}

		var expectedTxBatches []txRef

		for _, rawEvent := range eventBatch.Events {
			var evt apptypes.Event

			if err := cbor.Unmarshal(rawEvent, &evt); err != nil {
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
			txBatches, err := ews.txReader.GetNewBatchesBlocking(
				ctx,
				len(expectedTxBatches),
			) // ограничим разумно
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
				return nil, fmt.Errorf("%w: %x", ErrMissingTxBatch, ref.batchHash[:4])
			}

			var parsedTxs []appTx
			for _, rawTx := range txsRaw.RawTxs {
				var tx appTx
				if err := cbor.Unmarshal(rawTx, &tx); err != nil {
					return nil, fmt.Errorf("failed to unmarshal tx: %w", err)
				}

				parsedTxs = append(parsedTxs, tx)
			}

			result = append(result, apptypes.Batch[appTx, R]{
				Transactions: parsedTxs,
				// берем EndOffset из ивент батча — txBatch тоже можно пробрасывать
				EndOffset:   eventBatch.EndOffset,
				TxEndOffset: txsRaw.EndOffset,
			})
		}
	}

	return result, nil
}

func (ews *EventStreamWrapper[appTx, R]) Close() error {
	err := ews.eventReader.Close()
	if err != nil {
		return fmt.Errorf("failed to close event reader: %w", err)
	}

	err = ews.txReader.Close()
	if err != nil {
		return fmt.Errorf("failed to close tx reader: %w", err)
	}

	return nil
}
