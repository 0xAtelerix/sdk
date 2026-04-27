package gosdk

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/library"
	"github.com/0xAtelerix/sdk/gosdk/utility"
)

// Что должен дергать апчейн, чтобы получить транзакции
type BatchReader[appTx apptypes.AppTransaction[R], R apptypes.Receipt] interface {
	GetNewBatchesBlocking(limit int) ([]apptypes.Batch[appTx, R], error)
}

// EventReader reads append-only event batches from one epoch data file.
type EventReader struct {
	// dataFile is the open epoch data file.
	dataFile *os.File
	// watcher wakes the blocking reader when the file is appended or replaced.
	watcher *fsnotify.Watcher
	// watcherEvents is watcher.Events captured for tests and alternate constructors.
	watcherEvents <-chan fsnotify.Event
	// watcherErrors is watcher.Errors captured for tests and alternate constructors.
	watcherErrors <-chan error
	// pollInterval is the fallback poll cadence when fsnotify is quiet.
	pollInterval time.Duration
	// position points to the start of the next unread batch.
	position int64
	// reachedEOF records whether the last read reached EOF and should block.
	reachedEOF bool
}

// EventReaderConfig contains tunables for EventReader.
type EventReaderConfig struct {
	// PollInterval is the fallback poll cadence when fsnotify misses an append.
	PollInterval time.Duration
}

// DefaultEventReaderConfig returns the production EventReader defaults.
func DefaultEventReaderConfig() EventReaderConfig {
	return EventReaderConfig{
		PollInterval: 10 * time.Millisecond,
	}
}

// NewEventReader инициализирует reader с возможностью задать начальную позицию.
func NewEventReader(dataFilePath string, startPosition int64) (*EventReader, error) {
	return NewEventReaderWithConfig(dataFilePath, startPosition, DefaultEventReaderConfig())
}

func NewEventReaderWithPollInterval(
	dataFilePath string,
	startPosition int64,
	pollInterval time.Duration,
) (*EventReader, error) {
	return NewEventReaderWithConfig(dataFilePath, startPosition, EventReaderConfig{
		PollInterval: pollInterval,
	})
}

func NewEventReaderWithConfig(
	dataFilePath string,
	startPosition int64,
	config EventReaderConfig,
) (*EventReader, error) {
	f, err := os.OpenFile(dataFilePath, os.O_RDONLY, 0o644)
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

	if config.PollInterval <= 0 {
		config.PollInterval = DefaultEventReaderConfig().PollInterval
	}

	return &EventReader{
		dataFile:      f,
		watcher:       watcher,
		watcherEvents: watcher.Events,
		watcherErrors: watcher.Errors,
		pollInterval:  config.PollInterval,
		position:      startPosition,
		reachedEOF:    false,
	}, nil
}

func (er *EventReader) Close() error {
	err := er.dataFile.Close()
	if err != nil {
		return err
	}

	if er.watcher == nil {
		return nil
	}

	return er.watcher.Close()
}

// Batch содержит атропос и события
type ReadBatch struct {
	Atropos   [32]byte
	Events    [][]byte
	Offset    int64 // Сохранённая позиция этого батча в файле
	EndOffset int64 // позиция сразу после батча
}

func (er *EventReader) GetNewBatchesNonBlocking(
	ctx context.Context,
	limit int,
) ([]ReadBatch, error) {
	return er.readNewBatches(ctx, limit)
}

// GetNewBatchesBlocking ждёт, пока появятся новые батчи.
func (er *EventReader) GetNewBatchesBlocking(ctx context.Context, limit int) ([]ReadBatch, error) {
	logger := log.Ctx(ctx)

	var (
		// nil, пока EOF не достигнут
		timer   *time.Timer
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
					select {
					case <-timerCh:
					default:
					} // вычищаем, чтобы не словить старый тик позже
				}

				timer, timerCh = nil, nil //nolint:wastedassign,ineffassign // used, false-positive
			}

			return batches, nil
		}

		// 2. EOF: включаем/перезапускаем таймер ровно ОДИН раз
		if timer == nil {
			timer = time.NewTimer(er.pollInterval)
			timerCh = timer.C
		} else {
			if !timer.Stop() && timerCh != nil {
				logger.Debug().Msg("drain channel")

				select {
				case <-timerCh:
				default:
				} // дренация, если уже успел тикнуть

				logger.Debug().Msg("drained channel")
			}

			timer.Reset(er.pollInterval)
		}

		logger.Debug().Msg("Locking: readNewBatches return 0 batches")

		// 3. Ждём либо fsnotify-событие, либо истечение тайм-аутa
		select {
		case ev := <-er.watcherEvents:
			logger.Debug().Str("watcher", ev.String()).Msg("watcher event")

			if ev.Op&(fsnotify.Write|fsnotify.Rename|fsnotify.Create) != 0 {
				// При любом «интересном» событии снова идём в начало for
				continue
			}

		case <-timerCh:
			logger.Debug().Msg("timer expired")
			// Таймаут прошёл — снова в начало for (polling)
			continue

		case err := <-er.watcherErrors:
			logger.Warn().Err(err).Msg("fsnotify error")

		case <-ctx.Done():
			logger.Debug().Msg("Read blocking context done")

			return nil, ctx.Err()
		}
	}
}

// readNewBatches читает новые батчи, но не более `limit` за один вызов.
func (er *EventReader) readNewBatches(ctx context.Context, limit int) ([]ReadBatch, error) {
	var batches []ReadBatch

	logger := log.Ctx(ctx)
	vid := utility.ValidatorIDFromCtx(ctx)
	cid := utility.ChainIDFromCtx(ctx)

	// Перемещаем курсор в нужное место
	_, err := er.dataFile.Seek(er.position, 0)
	if err != nil {
		return nil, err
	}

	tRead := time.Now()

	// Начинаем читать батчи
	for range limit {
		batchStartPos := er.position // Запоминаем начало батча

		// Читаем размер батча (4 байта)
		var batchSize uint32

		err := binary.Read(er.dataFile, binary.BigEndian, &batchSize)
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			er.reachedEOF = true

			logger.Trace().Msg("reached eof in reading batch size")

			break
		} else if err != nil {
			return nil, err
		}

		// Читаем атропос (32 байта)
		var atropos [32]byte

		err = binary.Read(er.dataFile, binary.BigEndian, &atropos)
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
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
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || n == 0 {
			_, _ = er.dataFile.Seek(batchStartPos, 0) // Откат на начало
			er.reachedEOF = true

			logger.Trace().Msg("reached eof in reading remainingData")

			break
		} else if err != nil {
			return nil, err
		}

		// Разбираем события внутри батча
		var events [][]byte

		offset := 0
		for offset < len(remainingData) {
			// Читаем размер события (4 байта)
			var valueSize uint32

			err = binary.Read(
				bytes.NewReader(remainingData[offset:offset+4]),
				binary.BigEndian,
				&valueSize,
			)
			if err != nil {
				return nil, library.ErrCorruptedFile
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
		EventBatchEvents.WithLabelValues(vid, cid).Observe(float64(len(events)))
		// Если успешно прочитали весь батч, сдвигаем позицию
		er.position += int64(batchSize) + 4
	}

	StreamReadDuration.WithLabelValues(vid, cid).Observe(time.Since(tRead).Seconds())

	logger.Trace().Int("batches", len(batches)).Msg("readNewBatches return")

	return batches, nil
}
