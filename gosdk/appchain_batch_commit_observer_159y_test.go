package gosdk

import (
	"context"
	"testing"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	dto "github.com/prometheus/client_model/go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

type step159YBatchCommitObserverProcessor struct {
	processCalls  int
	observerCalls int
	checkpoint    apptypes.Checkpoint
	observerErr   error
	processErr    error
}

type step159YFailingCommitDB struct{ kv.RwDB }

func (db step159YFailingCommitDB) BeginRw(ctx context.Context) (kv.RwTx, error) {
	tx, err := db.RwDB.BeginRw(ctx)
	if err != nil {
		return nil, err
	}

	return step159YFailingCommitTx{RwTx: tx}, nil
}

type step159YFailingCommitTx struct{ kv.RwTx }

func (step159YFailingCommitTx) Commit() error {
	return step159YCommitError{}
}

type step159YCommitError struct{}

func (step159YCommitError) Error() string { return "step 15.9.Y forced commit failure" }

type step159YTestError string

func (err step159YTestError) Error() string { return string(err) }

func (processor *step159YBatchCommitObserverProcessor) ProcessBatch(
	context.Context,
	apptypes.Batch[prepareBatchTestTx, prepareBatchTestReceipt],
	kv.RwTx,
) ([]prepareBatchTestReceipt, []apptypes.ExternalTransaction, error) {
	processor.processCalls++

	return nil, nil, processor.processErr
}

func (processor *step159YBatchCommitObserverProcessor) AfterBatchCommit(
	_ context.Context,
	checkpoint apptypes.Checkpoint,
) error {
	processor.observerCalls++
	processor.checkpoint = checkpoint

	return processor.observerErr
}

func TestStep159YBatchCommitObserverRunsAfterSuccessfulCommit(t *testing.T) {
	t.Parallel()

	processor := &step159YBatchCommitObserverProcessor{}
	err := runStep159YBatch(t, processor)
	require.NoError(t, err)
	require.Equal(t, 1, processor.processCalls)
	require.Equal(t, 1, processor.observerCalls)
	require.Equal(t, uint64(1), processor.checkpoint.BlockNumber)
	metric, err := step159YObserverMetricValue(t.Name(), "success")
	require.NoError(t, err)
	require.InDelta(t, float64(1), metric, 0)
}

func TestStep159YBatchCommitObserverErrorDoesNotFailCommittedBatch(t *testing.T) {
	t.Parallel()

	processor := &step159YBatchCommitObserverProcessor{
		observerErr: step159YTestError("observer failed"),
	}
	err := runStep159YBatch(t, processor)
	require.NoError(t, err)
	require.Equal(t, 1, processor.processCalls)
	require.Equal(t, 1, processor.observerCalls)
	metric, metricErr := step159YObserverMetricValue(t.Name(), "error")
	require.NoError(t, metricErr)
	require.InDelta(t, float64(1), metric, 0)
}

func TestStep159YBatchCommitObserverIsNotCalledWhenBatchRollsBack(t *testing.T) {
	t.Parallel()

	processor := &step159YBatchCommitObserverProcessor{
		processErr: step159YTestError("process failed"),
	}
	err := runStep159YBatch(t, processor)
	require.ErrorContains(t, err, "process failed")
	require.Equal(t, 1, processor.processCalls)
	require.Zero(t, processor.observerCalls)
	metric, metricErr := step159YObserverMetricValue(t.Name(), "success")
	require.NoError(t, metricErr)
	require.Zero(t, metric)
}

func TestStep159YBatchCommitObserverIsNotCalledWhenCommitFails(t *testing.T) {
	t.Parallel()

	processor := &step159YBatchCommitObserverProcessor{}
	err := runStep159YBatchWithCommitFailure(t, processor)
	require.ErrorContains(t, err, "forced commit failure")
	require.Equal(t, 1, processor.processCalls)
	require.Zero(t, processor.observerCalls)
	metric, metricErr := step159YObserverMetricValue(t.Name(), "success")
	require.NoError(t, metricErr)
	require.Zero(t, metric)
}

func TestStep159YBatchWithoutCommitObserverPreservesExistingBehavior(t *testing.T) {
	t.Parallel()

	processor := &preparedBatchTestProcessor{}
	err := runStep159YBatch(t, processor)
	require.NoError(t, err)
	require.Equal(t, 1, processor.processCalls)
	metric, metricErr := step159YObserverMetricValue(t.Name(), "success")
	require.NoError(t, metricErr)
	require.Zero(t, metric)
}

func TestStep159YBatchCommitObserverUsesExactPreparedProcessor(t *testing.T) {
	t.Parallel()

	prepared := &step159YBatchCommitObserverProcessor{}
	parent := &prepareBatchTestProcessor{prepared: prepared}
	err := runStep159YBatch(t, parent)
	require.NoError(t, err)
	require.Equal(t, 1, parent.prepareCalls)
	require.Zero(t, parent.processCalls)
	require.Equal(t, 1, prepared.processCalls)
	require.Equal(t, 1, prepared.observerCalls)
	metric, metricErr := step159YObserverMetricValue(t.Name(), "success")
	require.NoError(t, metricErr)
	require.InDelta(t, float64(1), metric, 0)
}

func runStep159YBatch[
	BP BatchProcessor[prepareBatchTestTx, prepareBatchTestReceipt],
](t *testing.T, processor BP) error {
	t.Helper()

	return runStep159YBatchWithDBWrapper(t, processor, func(db kv.RwDB) kv.RwDB {
		return db
	})
}

func runStep159YBatchWithCommitFailure[
	BP BatchProcessor[prepareBatchTestTx, prepareBatchTestReceipt],
](t *testing.T, processor BP) error {
	t.Helper()

	return runStep159YBatchWithDBWrapper(t, processor, func(db kv.RwDB) kv.RwDB {
		return step159YFailingCommitDB{RwDB: db}
	})
}

func runStep159YBatchWithDBWrapper[
	BP BatchProcessor[prepareBatchTestTx, prepareBatchTestReceipt],
](t *testing.T, processor BP, wrapDB func(kv.RwDB) kv.RwDB) error {
	t.Helper()

	db, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(t.TempDir()).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg { return DefaultTables() }).
		Open()
	if err != nil {
		return err
	}

	t.Cleanup(db.Close)

	appchainDB := wrapDB(db)

	logger := zerolog.Nop()
	appchain := Appchain[
		prepareBatchTestTx,
		BP,
		prepareBatchTestBlock,
		prepareBatchTestReceipt,
	]{
		storage: &Storage[prepareBatchTestTx, prepareBatchTestReceipt]{
			appchainDB: appchainDB,
		},
		config:         &AppchainConfig{ChainID: 42, Logger: &logger},
		batchProcessor: processor,
		blockBuilder: func(_ uint64, _ [32]byte, _ [32]byte, _ apptypes.Batch[prepareBatchTestTx, prepareBatchTestReceipt]) prepareBatchTestBlock {
			return prepareBatchTestBlock{}
		},
		rootCalculator: NewStubRootCalculator(),
	}

	previousBlockNumber := uint64(0)
	previousBlockHash := [32]byte{}
	eventStream := &MdbxEventStreamWrapper[prepareBatchTestTx, prepareBatchTestReceipt]{}
	votingBlocks := NewVoting[apptypes.ExternalBlock](uint256.NewInt(0))
	votingCheckpoints := NewVoting[apptypes.Checkpoint](uint256.NewInt(0))

	return appchain.processBatch(
		t.Context(),
		apptypes.Batch[prepareBatchTestTx, prepareBatchTestReceipt]{},
		&previousBlockNumber,
		&previousBlockHash,
		eventStream,
		votingBlocks,
		votingCheckpoints,
		t.Name(),
		"42",
	)
}

func step159YObserverMetricValue(validatorID, result string) (float64, error) {
	metric := dto.Metric{}

	counter := BatchCommitObserverCalls.WithLabelValues(validatorID, "42", result)
	if err := counter.Write(&metric); err != nil {
		return 0, err
	}

	return metric.GetCounter().GetValue(), nil
}
