package gosdk

import (
	"context"
	"testing"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

type prepareBatchTestReceipt struct{}

func (prepareBatchTestReceipt) TxHash() [32]byte {
	return [32]byte{1}
}

func (prepareBatchTestReceipt) Status() apptypes.TxReceiptStatus {
	return apptypes.ReceiptConfirmed
}

func (prepareBatchTestReceipt) Error() string {
	return ""
}

type prepareBatchTestTx struct{}

func (prepareBatchTestTx) Hash() [32]byte {
	return [32]byte{2}
}

func (prepareBatchTestTx) Process(
	kv.RwTx,
) (prepareBatchTestReceipt, []apptypes.ExternalTransaction, error) {
	return prepareBatchTestReceipt{}, nil, nil
}

type prepareBatchTestBlock struct{}

func (prepareBatchTestBlock) Hash() [32]byte {
	return [32]byte{3}
}

func (prepareBatchTestBlock) StateRoot() [32]byte {
	return [32]byte{4}
}

type prepareBatchTestProcessor struct {
	prepareCalls int
	processCalls int
	prepared     BatchProcessor[prepareBatchTestTx, prepareBatchTestReceipt]
}

func (processor *prepareBatchTestProcessor) PrepareBatchProcessor(
	context.Context,
	apptypes.Batch[prepareBatchTestTx, prepareBatchTestReceipt],
) (BatchProcessor[prepareBatchTestTx, prepareBatchTestReceipt], error) {
	processor.prepareCalls++

	return processor.prepared, nil
}

func (processor *prepareBatchTestProcessor) ProcessBatch(
	context.Context,
	apptypes.Batch[prepareBatchTestTx, prepareBatchTestReceipt],
	kv.RwTx,
) ([]prepareBatchTestReceipt, []apptypes.ExternalTransaction, error) {
	processor.processCalls++

	return nil, nil, nil
}

type preparedBatchTestProcessor struct {
	processCalls int
}

func (processor *preparedBatchTestProcessor) ProcessBatch(
	context.Context,
	apptypes.Batch[prepareBatchTestTx, prepareBatchTestReceipt],
	kv.RwTx,
) ([]prepareBatchTestReceipt, []apptypes.ExternalTransaction, error) {
	processor.processCalls++

	return nil, nil, nil
}

func TestAppchainPrepareBatchProcessorUsesOptionalPreparedProcessor(t *testing.T) {
	t.Parallel()

	prepared := &preparedBatchTestProcessor{}
	processor := &prepareBatchTestProcessor{prepared: prepared}
	appchain := Appchain[
		prepareBatchTestTx,
		*prepareBatchTestProcessor,
		prepareBatchTestBlock,
		prepareBatchTestReceipt,
	]{
		batchProcessor: processor,
	}

	got, err := appchain.prepareBatchProcessor(
		t.Context(),
		apptypes.Batch[prepareBatchTestTx, prepareBatchTestReceipt]{},
	)
	require.NoError(t, err)
	require.Same(t, prepared, got)
	require.Equal(t, 1, processor.prepareCalls)
}

func TestAppchainPrepareBatchProcessorFallsBackToOriginalProcessor(t *testing.T) {
	t.Parallel()

	processor := &preparedBatchTestProcessor{}
	appchain := Appchain[
		prepareBatchTestTx,
		*preparedBatchTestProcessor,
		prepareBatchTestBlock,
		prepareBatchTestReceipt,
	]{
		batchProcessor: processor,
	}

	got, err := appchain.prepareBatchProcessor(
		t.Context(),
		apptypes.Batch[prepareBatchTestTx, prepareBatchTestReceipt]{},
	)
	require.NoError(t, err)
	require.Same(t, processor, got)
}
