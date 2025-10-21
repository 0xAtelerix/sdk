package gosdk

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/utility"
)

type MdbxEventStreamWrapper[appTx apptypes.AppTransaction[R], R apptypes.Receipt] struct {
	streamPath        string
	eventReader       *EventReader
	txReader          kv.RoDB
	chainID           uint32
	logger            *zerolog.Logger
	subscriber        *Subscriber
	appchainDB        kv.RwDB
	votingBlocks      *Voting[apptypes.ExternalBlock]
	votingCheckpoints *Voting[apptypes.Checkpoint]
	currentEpoch      uint32
}

type EventStreamWrapperConstructor[appTx apptypes.AppTransaction[R], R apptypes.Receipt] func(
	eventsPath string,
	chainID uint32,
	eventStartPos int64,
	txBatchDB kv.RoDB,
	logger *zerolog.Logger,
	appchainTx kv.RoDB,
	subscriber *Subscriber,
	votingBlocks *Voting[apptypes.ExternalBlock],
	votingCheckpoints *Voting[apptypes.Checkpoint],
) (Streamer[appTx, R], error)

func NewMdbxEventStreamWrapper[appTx apptypes.AppTransaction[R], R apptypes.Receipt](
	eventsPath string,
	chainID uint32,
	txBatchDB kv.RoDB,
	logger *zerolog.Logger,
	appchainDB kv.RwDB,
	subscriber *Subscriber,
	votingBlocks *Voting[apptypes.ExternalBlock],
	votingCheckpoints *Voting[apptypes.Checkpoint],
) (*MdbxEventStreamWrapper[appTx, R], error) {
	wrapper := &MdbxEventStreamWrapper[appTx, R]{
		streamPath:        eventsPath,
		txReader:          txBatchDB,
		chainID:           chainID,
		logger:            logger,
		subscriber:        subscriber,
		appchainDB:        appchainDB,
		votingBlocks:      votingBlocks,
		votingCheckpoints: votingCheckpoints,
	}
	err := wrapper.InitReader(context.TODO())
	if err != nil {
		return nil, err
	}
	return wrapper, nil
}

func (ews *MdbxEventStreamWrapper[appTx, R]) InitReader(ctx context.Context) error {
	pos, epoch, err := GetLastStreamPositions(ctx, ews.appchainDB)
	if err != nil {
		return err
	}
	ews.currentEpoch = epoch
	newPath := filepath.Join(ews.streamPath, fmt.Sprintf("epoch_%d.data", epoch))
	err = WaitFile(ctx, newPath, log.Ctx(ctx))
	if err != nil {
		return err
	}

	eventReader, err := NewEventReader(newPath, pos)
	if err != nil {
		return fmt.Errorf("failed to create event reader: %w", err)
	}
	ews.eventReader = eventReader

	return nil
}

type Streamer[appTx apptypes.AppTransaction[R], R apptypes.Receipt] interface {
	GetNewBatchesBlocking(ctx context.Context, limit int) ([]apptypes.Batch[appTx, R], error)
	Close() error
}

var EndOfEpoch = bytes.Repeat([]byte{0xFF}, 28)

func (ews *MdbxEventStreamWrapper[appTx, R]) GetNewBatchesBlocking(
	ctx context.Context,
	limit int,
) ([]apptypes.Batch[appTx, R], error) {
	ews.logger.Debug().Int("len", limit).Msg("get new batches")

	vid := utility.ValidatorIDFromCtx(ctx)
	cid := utility.ChainIDFromCtx(ctx)

	eventBatches, err := ews.eventReader.GetNewBatchesBlocking(ctx, limit)
	if err != nil {
		return nil, err
	}

	ews.logger.Debug().Int("batches", len(eventBatches)).Msg("got new batches")

	var result []apptypes.Batch[appTx, R] //nolint:prealloc // hard to predict also many cases will be with empty batches

	// getting the valset for the epoch
	var valset *ValidatorSet

	newEpoch := uint32(0)
	var newValset []byte
	for _, eventBatch := range eventBatches {
		ews.logger.Info().Str("atropos", hex.EncodeToString(eventBatch.Atropos[4:])).Msg("Compare atropos hash")
		if bytes.Equal(eventBatch.Atropos[4:], EndOfEpoch) {
			newEpoch = binary.BigEndian.Uint32(eventBatch.Atropos[:4])
			newValset = eventBatch.Events[0]
			continue
		}

		ews.logger.Debug().
			Hex("atropos", eventBatch.Atropos[:]).
			Int("events", len(eventBatch.Events)).
			Msg("Processing event batch")

		txBatches := map[[32]byte][][]byte{}
		// Список нужных транзакционных батчей
		type txRef struct {
			eventID   [32]byte
			batchHash [32]byte
		}

		var expectedTxBatches []txRef

		tParseEvt := time.Now()

		for _, rawEvent := range eventBatch.Events {
			var evt apptypes.Event

			if err = cbor.Unmarshal(rawEvent, &evt); err != nil {
				return nil, fmt.Errorf("failed to decode event: %w", err)
			}

			var notFoundCycleValset int

			// FIXME: epoch change, update valset accordingly
			for valset == nil {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
				}

				if notFoundCycleValset != 0 {
					time.Sleep(time.Millisecond * 50)
				}

				notFoundCycleValset++

				ews.logger.Debug().
					Int("notFoundCycleValset", notFoundCycleValset).
					Interface("validator set", valset).
					Msg("timed out waiting for valset")

				if notFoundCycleValset%(1000/50) == 0 {
					ews.logger.Warn().
						Int("notFoundCycleValset", notFoundCycleValset).
						Interface("validator set", valset).
						Msg("timed out waiting for valset")
				}

				key := [4]byte{}
				binary.BigEndian.PutUint32(key[:], evt.Base.Epoch)

				err = ews.appchainDB.View(ctx, func(tx kv.Tx) error {
					var valsetData []byte

					valsetData, err = tx.GetOne(ValsetBucket, key[:])
					if err != nil {
						ews.logger.Err(err).
							Uint32("Epoch", evt.Base.Epoch)

						return err
					}

					if len(valsetData) == 0 {
						return fmt.Errorf("%w epoch %d", ErrNoValidatorSet, evt.Base.Epoch)
					}

					valset = &ValidatorSet{}

					err = cbor.Unmarshal(valsetData, valset)
					if err != nil {
						ews.logger.Err(err).
							Uint32("Epoch", evt.Base.Epoch)

						return err
					}

					ews.votingBlocks.SetValset(valset)
					ews.votingCheckpoints.SetValset(valset)

					return nil
				})
				if err != nil {
					ews.logger.Warn().Err(err).Msg("failed to view validator set")

					continue
				}
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

			// votingBlocks
			for _, extBlock := range evt.BlockVotes {
				ews.votingBlocks.AddVote(
					extBlock,
					uint256.NewInt(uint64(valset.GetStake(ValidatorID(evt.Base.Creator)))),
					evt.Base.Epoch,
					evt.Base.Creator,
				)
			}

			for _, checkpoint := range evt.Appchains {
				ews.votingCheckpoints.AddVote(
					checkpoint,
					uint256.NewInt(uint64(valset.GetStake(ValidatorID(evt.Base.Creator)))),
					evt.Base.Epoch,
					evt.Base.Creator,
				)
			}
		}

		MdbxEventParseDuration.WithLabelValues(vid, cid).Observe(time.Since(tParseEvt).Seconds())
		MdbxTxBatchesExpectedTotal.WithLabelValues(vid, cid).Add(float64(len(expectedTxBatches)))

		ews.logger.Debug().
			Hex("atropos", eventBatch.Atropos[:]).
			Int("expected batches", len(expectedTxBatches)).
			Int("txBatches", len(txBatches)).
			Msg("expectedTxBatches")

		waitStart := time.Now()

		var notFoundCycle uint64

		for numOfFound := 0; numOfFound < len(txBatches); {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			if numOfFound != 0 {
				time.Sleep(time.Millisecond * 50)

				notFoundCycle++

				var s []string
				for i := range txBatches {
					s = append(s, hex.EncodeToString(i[:]))
				}

				ews.logger.Debug().
					Int("numOfFound", numOfFound).
					Int("len(txBatches)", len(txBatches)).
					Strs("batches", s).
					Msg("timed out waiting for batches")

				if notFoundCycle%(1000/50) == 0 {
					ews.logger.Warn().
						Int("numOfFound", numOfFound).
						Int("len(txBatches)", len(txBatches)).
						Strs("batches", s).
						Msg("timed out waiting for batches")
				}
			}

			err := func(ctx context.Context) error {
				tLookup := time.Now()

				tx, err := ews.txReader.BeginRo(ctx)
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

				MdbxTxLookupDuration.WithLabelValues(vid, cid).
					Observe(time.Since(tLookup).Seconds())

				return nil
			}(ctx)
			if err != nil {
				ews.logger.Error().Err(err).Msg("got tx batches from mdbx")

				return nil, err
			}
		}

		ews.logger.Debug().
			Int("expectedTxBatches", len(expectedTxBatches)).
			Msg("got tx batches from mdbx")

		if notFoundCycle > 0 {
			MdbxWaitCyclesTotal.WithLabelValues(vid, cid).Add(float64(notFoundCycle))
			MdbxWaitTimeSeconds.WithLabelValues(vid, cid).Observe(time.Since(waitStart).Seconds())
		}

		MdbxTxBatchesFoundTotal.WithLabelValues(vid, cid).Add(float64(len(txBatches)))

		var allParsedTxs []appTx

		for _, ref := range expectedTxBatches {
			txsRaw, ok := txBatches[ref.batchHash]
			if !ok {
				return nil, fmt.Errorf("%w: %x", ErrMissingTxBatch, ref.batchHash[:4])
			}

			for _, rawTx := range txsRaw {
				var tx appTx
				if err := cbor.Unmarshal(rawTx, &tx); err != nil {
					ews.logger.Error().
						Err(err).
						Str("json", string(rawTx)).
						Msg("failed to unmarshal tx")

					return nil, fmt.Errorf("failed to unmarshal tx: %w", err)
				}

				allParsedTxs = append(allParsedTxs, tx)
			}
		}

		result = append(result, apptypes.Batch[appTx, R]{
			Atropos:        eventBatch.Atropos,
			Transactions:   allParsedTxs,
			ExternalBlocks: ews.votingBlocks.PopFinalized(),      // return only finalized
			Checkpoints:    ews.votingCheckpoints.PopFinalized(), // return only finalized
			EndOffset:      eventBatch.EndOffset,
		})
	}
	if newEpoch > 0 {
		err = ews.appchainDB.Update(ctx, func(tx kv.RwTx) error {
			epochkey := make([]byte, 4)
			binary.BigEndian.PutUint32(epochkey, newEpoch)
			err = tx.Put(ValsetBucket, epochkey, newValset)
			ews.logger.Warn().
				Uint32("epoch", newEpoch).
				Int("valset len", len(newValset)).
				Msg("new epoch set")

			return WriteSnapshotPosition(tx, newEpoch, 8)
		})
		if err != nil {
			return nil, err
		}
		err = ews.InitReader(ctx)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (ews *MdbxEventStreamWrapper[appTx, R]) Close() error {
	err := ews.eventReader.Close()
	if err != nil {
		return err
	}

	return nil
}
