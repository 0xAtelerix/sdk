package gosdk

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/library"
)

// The epoch rollover tests replay the restart window around an end-of-epoch
// marker. The stream position a restarted appchain resumes from must never
// run ahead of what processBatch has committed, otherwise the tail batches of
// the finished epoch are silently skipped and the node's block sequence
// diverges from the rest of the cluster.

func TestEpochRolloverRestartDoesNotSkipUncommittedTail(t *testing.T) {
	dir, db, tailAtropos, headAtropos := newEpochRolloverFixture(t)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	first := newEpochRolloverWrapper(ctx, t, dir, db)

	// The first read crosses the end-of-epoch marker in the same call: it
	// returns the tail batch of epoch 1 and rolls the reader over to epoch 2.
	batches := readEpochRolloverBatches(ctx, t, first, 10)
	require.Equal(t, tailAtropos, batches[0].Atropos)
	require.NoError(t, first.Close())

	// The appchain stopped before committing anything: no snapshot position
	// was persisted for the tail batch. A restarted wrapper must replay the
	// tail batch of epoch 1, not resume at epoch 2.
	second := newEpochRolloverWrapper(ctx, t, dir, db)

	batches = readEpochRolloverBatches(ctx, t, second, 1)
	require.Equalf(
		t,
		tailAtropos,
		batches[0].Atropos,
		"restart across an epoch rollover skipped the uncommitted tail batch of the finished epoch",
	)

	batches = readEpochRolloverBatches(ctx, t, second, 1)
	require.Equal(t, headAtropos, batches[0].Atropos)
	require.NoError(t, second.Close())
}

func TestEpochRolloverCommittedTailResumesAtNewEpoch(t *testing.T) {
	dir, db, tailAtropos, headAtropos := newEpochRolloverFixture(t)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	first := newEpochRolloverWrapper(ctx, t, dir, db)

	batches := readEpochRolloverBatches(ctx, t, first, 10)
	require.Equal(t, tailAtropos, batches[0].Atropos)

	// Batches carry the epoch of the file they were read from, even though the
	// reader has already rolled over to epoch 2 by the time they are returned.
	require.Equal(t, uint32(1), batches[0].Epoch)

	// Commit the tail batch the way processBatch does: position keyed by the
	// batch's own epoch.
	require.NoError(t, db.Update(ctx, func(tx kv.RwTx) error {
		return WriteSnapshotPosition(tx, batches[0].Epoch, batches[0].EndOffset)
	}))
	require.NoError(t, first.Close())

	// The restarted wrapper resumes after the committed tail, re-reads the
	// end-of-epoch marker, rolls over, and hands out the epoch 2 head batch.
	second := newEpochRolloverWrapper(ctx, t, dir, db)

	batches = readEpochRolloverBatches(ctx, t, second, 10)
	require.Equal(t, headAtropos, batches[0].Atropos)
	require.Equal(t, uint32(2), batches[0].Epoch)
	require.NoError(t, second.Close())
}

// newEpochRolloverFixture lays out a two-epoch stream: epoch 1 holds one tail
// batch followed by the end-of-epoch marker, epoch 2 holds one head batch.
func newEpochRolloverFixture(
	t *testing.T,
) (string, kv.RwDB, [32]byte, [32]byte) {
	t.Helper()

	dir := t.TempDir()
	db := openCEXRefTestDB(t)

	valset := NewValidatorSet(map[ValidatorID]Stake{1: 10, 2: 20, 3: 30})

	valsetData, err := cbor.Marshal(valset)
	require.NoError(t, err)

	var epochKey [4]byte

	binary.BigEndian.PutUint32(epochKey[:], 1)

	require.NoError(t, db.Update(t.Context(), func(tx kv.RwTx) error {
		return tx.Put(ValsetBucket, epochKey[:], valsetData)
	}))

	epoch1 := filepath.Join(dir, "epoch_1.data")
	require.NoError(t, os.WriteFile(epoch1, eventReaderHeader(), 0o644))

	tailAtropos := [32]byte{1: 0xAA}
	require.NoError(t, appendEventReaderBatch(
		epoch1,
		tailAtropos,
		[][]byte{marshalEpochRolloverEvent(t, 1, [32]byte{1: 0xA1})},
	))

	var markerAtropos [32]byte

	binary.BigEndian.PutUint32(markerAtropos[:4], 2)
	copy(markerAtropos[4:], library.EndOfEpochSuffix)
	require.NoError(t, appendEventReaderBatch(epoch1, markerAtropos, [][]byte{valsetData}))

	epoch2 := filepath.Join(dir, "epoch_2.data")
	require.NoError(t, os.WriteFile(epoch2, eventReaderHeader(), 0o644))

	headAtropos := [32]byte{1: 0xBB}
	require.NoError(t, appendEventReaderBatch(
		epoch2,
		headAtropos,
		[][]byte{marshalEpochRolloverEvent(t, 2, [32]byte{1: 0xB1})},
	))

	return dir, db, tailAtropos, headAtropos
}

func newEpochRolloverWrapper(
	ctx context.Context,
	t *testing.T,
	dir string,
	db kv.RwDB,
) *MdbxEventStreamWrapper[*CustomTransaction[Receipt], Receipt] {
	t.Helper()

	logger := zerolog.Nop()
	valset := NewValidatorSet(map[ValidatorID]Stake{1: 10, 2: 20, 3: 30})

	wrapper, err := NewMdbxEventStreamWrapper[*CustomTransaction[Receipt], Receipt](
		ctx,
		dir,
		1,
		openCEXRefTestDB(t),
		&logger,
		db,
		NewVotingFromValidatorSet[apptypes.ExternalBlock](valset),
		NewVotingFromValidatorSet[apptypes.Checkpoint](valset),
	)
	require.NoError(t, err)

	return wrapper
}

// readEpochRolloverBatches keeps polling until the wrapper hands out batches:
// a call that only consumes the end-of-epoch marker legitimately returns an
// empty slice.
func readEpochRolloverBatches(
	ctx context.Context,
	t *testing.T,
	wrapper *MdbxEventStreamWrapper[*CustomTransaction[Receipt], Receipt],
	want int,
) []apptypes.Batch[*CustomTransaction[Receipt], Receipt] {
	t.Helper()

	for {
		batches, err := wrapper.GetNewBatchesBlocking(ctx, want)
		require.NoError(t, err)

		if len(batches) > 0 {
			return batches
		}
	}
}

func marshalEpochRolloverEvent(t *testing.T, epoch uint32, id [32]byte) []byte {
	t.Helper()

	raw, err := cbor.Marshal(apptypes.Event{
		Base: apptypes.BaseEvent{ID: id, Epoch: epoch, Creator: 1},
	})
	require.NoError(t, err)

	return raw
}
