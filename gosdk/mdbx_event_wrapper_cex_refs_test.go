package gosdk

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erigontech/mdbx-go/mdbx"
	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"
	kvmdbx "github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// The wrapper is the only place where a decoded event becomes a batch the
// appchain state transition can see. Both kinds of public-data reference travel
// the same way — the producer puts them on an event, the consumer reads them off
// a batch — so a kind that is accumulated on one side and not the other is lost
// with no error anywhere: the batch stays valid and merely arrives empty.
func TestGetNewBatchesCarriesEveryPublicDataRefKind(t *testing.T) {
	t.Parallel()

	wrapper, path := newCEXRefTestWrapper(t)

	event := apptypes.Event{
		Base: apptypes.BaseEvent{Epoch: cexRefTestEpoch, Creator: 1},
		CEXOrderBookRefs: []apptypes.CEXOrderBookRef{
			{ExchangeID: 1, MarketTypeID: 2, SymbolID: 7, FetchedAt: 11},
		},
		CEXMarketTradeBatchRefs: []apptypes.CEXMarketTradeBatchRef{
			{ExchangeID: 1, MarketTypeID: 2, SymbolID: 7, BatchID: 1, TradeCount: 3},
		},
	}

	appendCEXRefTestEvents(t, path, [32]byte{31: 1}, event)

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	batches, err := wrapper.GetNewBatchesBlocking(ctx, 10)
	require.NoError(t, err)
	require.Len(t, batches, 1)

	require.Len(t, batches[0].CEXOrderBookRefs, 1)
	require.Len(
		t,
		batches[0].CEXMarketTradeBatchRefs,
		1,
		"the event carried a trade batch reference and the batch arrived without it",
	)
	require.Equal(t, uint64(1), batches[0].CEXMarketTradeBatchRefs[0].BatchID)
}

// Accumulators were reset with [:0], which keeps the backing array a batch
// already in the result slice is pointing at. A second event batch read in the
// same call then overwrites the first batch's references in place, so a
// consumer sees the newest market's data attributed to the older block.
func TestGetNewBatchesDoesNotAliasRefsAcrossBatches(t *testing.T) {
	t.Parallel()

	wrapper, path := newCEXRefTestWrapper(t)

	first := apptypes.Event{
		Base: apptypes.BaseEvent{Epoch: cexRefTestEpoch, Creator: 1},
		CEXOrderBookRefs: []apptypes.CEXOrderBookRef{
			{ExchangeID: 1, MarketTypeID: 2, SymbolID: 7, FetchedAt: 11},
		},
		CEXMarketTradeBatchRefs: []apptypes.CEXMarketTradeBatchRef{
			{ExchangeID: 1, MarketTypeID: 2, SymbolID: 7, BatchID: 1},
		},
	}
	second := apptypes.Event{
		Base: apptypes.BaseEvent{Epoch: cexRefTestEpoch, Creator: 1},
		CEXOrderBookRefs: []apptypes.CEXOrderBookRef{
			{ExchangeID: 1, MarketTypeID: 2, SymbolID: 9, FetchedAt: 22},
		},
		CEXMarketTradeBatchRefs: []apptypes.CEXMarketTradeBatchRef{
			{ExchangeID: 1, MarketTypeID: 2, SymbolID: 9, BatchID: 2},
		},
	}

	appendCEXRefTestEvents(t, path, [32]byte{31: 1}, first)
	appendCEXRefTestEvents(t, path, [32]byte{31: 2}, second)

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	batches, err := wrapper.GetNewBatchesBlocking(ctx, 10)
	require.NoError(t, err)
	require.Len(t, batches, 2)

	require.Equal(
		t,
		apptypes.CEXSymbolID(7),
		batches[0].CEXOrderBookRefs[0].SymbolID,
		"the second batch overwrote the first batch's order book reference",
	)
	require.Equal(
		t,
		uint64(1),
		batches[0].CEXMarketTradeBatchRefs[0].BatchID,
		"the second batch overwrote the first batch's trade batch reference",
	)
	require.Equal(t, apptypes.CEXSymbolID(9), batches[1].CEXOrderBookRefs[0].SymbolID)
	require.Equal(t, uint64(2), batches[1].CEXMarketTradeBatchRefs[0].BatchID)
}

const cexRefTestEpoch = uint32(1)

// newCEXRefTestWrapper builds the wrapper over a real event file and a real
// MDBX validator set, so the test exercises the production assembly path rather
// than a re-implementation of it.
func newCEXRefTestWrapper(
	t *testing.T,
) (*MdbxEventStreamWrapper[*CustomTransaction[Receipt], Receipt], string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "epoch_1.data")
	require.NoError(t, os.WriteFile(path, eventReaderHeader(), 0o644))

	file, err := os.OpenFile(path, os.O_RDONLY, 0o644)
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })

	db := openCEXRefTestDB(t)
	logger := zerolog.Nop()

	valset := NewValidatorSet(map[ValidatorID]Stake{1: 10, 2: 20, 3: 30})

	valsetData, err := cbor.Marshal(valset)
	require.NoError(t, err)

	var epochKey [4]byte

	binary.BigEndian.PutUint32(epochKey[:], cexRefTestEpoch)

	require.NoError(t, db.Update(t.Context(), func(tx kv.RwTx) error {
		return tx.Put(ValsetBucket, epochKey[:], valsetData)
	}))

	wrapper := &MdbxEventStreamWrapper[*CustomTransaction[Receipt], Receipt]{
		streamPath:        path,
		eventReader:       &EventReader{dataFile: file, pollInterval: time.Millisecond, position: 8},
		chainID:           1,
		logger:            &logger,
		appchainDB:        db,
		votingBlocks:      NewVotingFromValidatorSet[apptypes.ExternalBlock](valset),
		votingCheckpoints: NewVotingFromValidatorSet[apptypes.Checkpoint](valset),
		currentEpoch:      cexRefTestEpoch,
		config:            DefaultMdbxEventStreamWrapperConfig(),
	}

	return wrapper, path
}

func openCEXRefTestDB(t *testing.T) kv.RwDB {
	t.Helper()

	db, err := kvmdbx.NewMDBX(mdbxlog.New()).
		Path(t.TempDir()).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg { return DefaultTables() }).
		Flags(func(flags uint) uint { return flags | mdbx.NoMemInit }).
		Open()
	require.NoError(t, err)

	t.Cleanup(db.Close)

	return db
}

func appendCEXRefTestEvents(t *testing.T, path string, atropos [32]byte, events ...apptypes.Event) {
	t.Helper()

	encoded := make([][]byte, 0, len(events))

	for _, event := range events {
		raw, err := cbor.Marshal(event)
		require.NoError(t, err)

		encoded = append(encoded, raw)
	}

	require.NoError(t, appendEventReaderBatch(path, atropos, encoded))
}
