package gosdk

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/rs/zerolog/log"
	"io"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/0xAtelerix/sdk/gosdk/types"
)

// Что должен дергать апчейн, чтобы получить транзакции
type BatchReader[appTx types.AppTransaction] interface {
	GetNewBatchesBlocking(limit int) ([]types.Batch[appTx], error)
}

// EventReader with fsnotify
type EventReader struct {
	dataFile   *os.File
	watcher    *fsnotify.Watcher
	position   int64 // Указывает на начало последнего прочитанного батча
	reachedEOF bool  // Достигли ли мы конца файла? Нужно, чтобы переключаться на fnotify блокировку
}

// NewEventReader инициализирует reader с возможностью задать начальную позицию.
func NewEventReader(dataFilePath string, startPosition int64) (*EventReader, error) {
	f, err := os.OpenFile(dataFilePath, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := watcher.Add(dataFilePath); err != nil {
		return nil, err
	}

	// Если передали некорректную позицию (меньше заголовка), ставим её в 8
	if startPosition < 8 {
		startPosition = 8
	}

	return &EventReader{
		dataFile:   f,
		watcher:    watcher,
		position:   startPosition,
		reachedEOF: false,
	}, nil
}

func (er *EventReader) Close() {
	er.dataFile.Close()
	er.watcher.Close()
}

// Batch содержит атропос и события
type ReadBatch struct {
	Atropos   [32]byte
	Events    [][]byte
	Offset    int64 // Сохранённая позиция этого батча в файле
	EndOffset int64 // позиция сразу после батча
}

// readNewBatches читает новые батчи, но не более `limit` за один вызов.
func (er *EventReader) readNewBatches(ctx context.Context, limit int) ([]ReadBatch, error) {
	var batches []ReadBatch
	logger := log.Ctx(ctx)
	// Перемещаем курсор в нужное место
	_, err := er.dataFile.Seek(er.position, 0)
	if err != nil {
		return nil, err
	}

	// Начинаем читать батчи
	for i := 0; i < limit; i++ {
		batchStartPos := er.position // Запоминаем начало батча

		// Читаем размер батча (4 байта)
		var batchSize uint32
		err := binary.Read(er.dataFile, binary.BigEndian, &batchSize)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			er.reachedEOF = true
			logger.Trace().Msg("reached eof in reading batch size")
			break
		} else if err != nil {
			return nil, err
		}

		// Читаем атропос (32 байта)
		var atropos [32]byte
		err = binary.Read(er.dataFile, binary.BigEndian, &atropos)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			_, _ = er.dataFile.Seek(batchStartPos, 0) // Откат на начало
			er.reachedEOF = true
			logger.Trace().Msg("reached eof in reading atropos")
			break
		} else if err != nil {
			return nil, err
		}

		// Читаем весь оставшийся батч в один буфер
		remainingData := make([]byte, batchSize-32)
		n, err := io.ReadFull(er.dataFile, remainingData)
		if err == io.EOF || err == io.ErrUnexpectedEOF || n == 0 {
			_, _ = er.dataFile.Seek(batchStartPos, 0) // Откат на начало
			er.reachedEOF = true
			logger.Trace().Msg("reached eof in reading remainingData")
			break
		} else if err != nil {
			return nil, err
		}

		// Разбираем события внутри батча
		events := [][]byte{}
		offset := 0
		for offset < len(remainingData) {
			// Читаем размер события (4 байта)
			var valueSize uint32
			err = binary.Read(bytes.NewReader(remainingData[offset:offset+4]), binary.BigEndian, &valueSize)
			if err != nil {
				return nil, fmt.Errorf("corrupted file")
			}
			offset += 4

			// Читаем само событие
			eventData := remainingData[offset : offset+int(valueSize)]
			events = append(events, eventData)
			offset += int(valueSize)
		}

		batchEndPos := batchStartPos + int64(4+batchSize)

		// Добавляем batch в список
		batches = append(batches, ReadBatch{
			Atropos:   atropos,
			Events:    events,
			Offset:    batchStartPos,
			EndOffset: batchEndPos,
		})

		// Если успешно прочитали весь батч, сдвигаем позицию
		er.position += int64(batchSize) + 4
	}
	logger.Trace().Int("batches", len(batches)).Msg("readNewBatches return")
	return batches, nil
}

func (er *EventReader) GetNewBatchesNonBlocking(ctx context.Context, limit int) ([]ReadBatch, error) {
	return er.readNewBatches(ctx, limit)
}

// GetNewBatchesBlocking ждёт, пока появятся новые батчи.
func (er *EventReader) GetNewBatchesBlocking(ctx context.Context, limit int) ([]ReadBatch, error) {
	logger := log.Ctx(ctx)

	var (
		timer   *time.Timer // nil, пока EOF не достигнут
		timerCh <-chan time.Time
	)

	for {
		// 1. Пытаемся читать
		batches, err := er.readNewBatches(ctx, limit)
		if err != nil {
			logger.Error().Err(err).Msg("readNewBatches err")
			return nil, err
		}
		if len(batches) > 0 {
			// Нашли новые батчи: выключаем таймер (если был) и отдаём caller'у
			if timer != nil {
				if !timer.Stop() && timerCh != nil {
					<-timerCh // вычищаем, чтобы не словить старый тик позже
				}
				timer, timerCh = nil, nil
			}
			return batches, nil
		}
		// 2. EOF: включаем/перезапускаем таймер ровно ОДИН раз
		if timer == nil {
			timer = time.NewTimer(100 * time.Millisecond)
			timerCh = timer.C
		} else {
			if !timer.Stop() && timerCh != nil {
				logger.Trace().Msg("drain channel")
				select {
				case <-timerCh:
				default:

				} // дренация, если уже успел тикнуть
				logger.Trace().Msg("drained channel")
			}
			timer.Reset(100 * time.Millisecond)
		}
		logger.Trace().Msg("Locking: readNewBatches return 0 batches")
		// 3. Ждём либо fsnotify-событие, либо истечение тайм-аутa
		select {
		case ev := <-er.watcher.Events:
			logger.Debug().Str("watcher", ev.String()).Msg("watcher event")
			if ev.Op&(fsnotify.Write|fsnotify.Rename|fsnotify.Create) != 0 {
				// При любом «интересном» событии снова идём в начало for
				continue
			}

		case <-timerCh:
			logger.Debug().Msg("timer expired")
			// Таймаут прошёл — снова в начало for (polling)
			continue

		case err := <-er.watcher.Errors:
			logger.Warn().Err(err).Msg("fsnotify error")

		case <-ctx.Done():
			logger.Debug().Msg("Read blocking context done")
			return nil, ctx.Err()
		}
	}
}
