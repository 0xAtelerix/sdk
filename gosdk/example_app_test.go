package gosdk

import (
	"context"
	"crypto/sha256"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/txpool"
)

func TestExampleAppchain(t *testing.T) {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	tmp := t.TempDir()

	config := AppchainConfig{
		ChainID:           DefaultAppchainID,
		EmitterPort:       DefaultEmitterPort,
		AppchainDBPath:    AppchainDBPath(tmp, DefaultAppchainID),
		EventStreamDir:    EventsPath(tmp),
		TxStreamDir:       TxBatchPath(tmp, DefaultAppchainID),
		MultichainStateDB: map[apptypes.ChainType]string{},
	}

	localDB, err := mdbx.NewMDBX(mdbxlog.New()).
		InMem(tmp).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return txpool.Tables()
		}).
		Open()
	require.NoError(t, err)

	txPool := txpool.NewTxPool[ExampleTransaction[ExampleReceipt]](localDB)

	appchainDB, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(config.AppchainDBPath).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return DefaultTables()
		}).
		Open()
	require.NoError(t, err)

	txBatchDB, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(config.TxStreamDir).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return TxBucketsTables()
		}).
		Open()
	require.NoError(t, err)

	subscriber, err := NewSubscriber(t.Context(), appchainDB)
	require.NoError(t, err)

	chainDBs, err := NewMultichainStateAccessDB(config.MultichainStateDB)
	require.NoError(t, err)

	multichainDB := NewMultichainStateAccess(chainDBs)

	// Create InitResult manually for test
	appInit := &InitResult{
		Config:     config,
		AppchainDB: appchainDB,
		LocalDB:    localDB,
		TxBatchDB:  txBatchDB,
		Multichain: multichainDB,
		Subscriber: subscriber,
	}

	log.Info().Msg("Creating appchain...")

	appchainExample := NewAppchain(
		appInit,
		NewDefaultBatchProcessor[ExampleTransaction[ExampleReceipt], ExampleReceipt](
			NewExtBlockProcessor(multichainDB),
			multichainDB,
			subscriber,
		),
		func(_ uint64, _ [32]byte, _ [32]byte, _ apptypes.Batch[ExampleTransaction[ExampleReceipt], ExampleReceipt]) *ExampleBlock {
			return &ExampleBlock{}
		},
		txPool,
	)

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	// Run appchain in goroutine
	runErr := make(chan error, 1)

	log.Info().Msg("Starting appchain...")

	go func() {
		assert.NotPanics(t, func() {
			runErr <- appchainExample.Run(ctx)
		})
	}()

	err = <-runErr

	appchainExample.Shutdown()

	log.Info().Err(err).Msg("Appchain exited")
}

type ExampleTransaction[R ExampleReceipt] struct {
	Sender string
	Value  int
}

func (c ExampleTransaction[R]) Hash() [32]byte {
	s := c.Sender + strconv.Itoa(c.Value)

	return sha256.Sum256([]byte(s))
}

func (ExampleTransaction[R]) Process(
	_ kv.RwTx,
) (receipt R, txs []apptypes.ExternalTransaction, err error) {
	return
}

type ExampleReceipt struct{}

func (ExampleReceipt) TxHash() [32]byte {
	return [32]byte{}
}

func (ExampleReceipt) Status() apptypes.TxReceiptStatus {
	return apptypes.ReceiptConfirmed
}

func (ExampleReceipt) Error() string {
	return ""
}

type ExampleBlock struct{}

func (*ExampleBlock) Number() uint64 {
	return 0
}

func (*ExampleBlock) Hash() [32]byte {
	return [32]byte{}
}

func (*ExampleBlock) StateRoot() [32]byte {
	return [32]byte{}
}

func (*ExampleBlock) Bytes() []byte {
	return []byte{}
}

// Verify ExtBlockProcessor implements ExternalBlockProcessor interface.
var _ ExternalBlockProcessor = &ExtBlockProcessor{}

type ExtBlockProcessor struct {
	MultiChain *MultichainStateAccess
}

func NewExtBlockProcessor(multiChain *MultichainStateAccess) *ExtBlockProcessor {
	return &ExtBlockProcessor{
		MultiChain: multiChain,
	}
}

func (*ExtBlockProcessor) ProcessBlock(
	_ apptypes.ExternalBlock,
	_ kv.RwTx,
) ([]apptypes.ExternalTransaction, error) {
	return nil, nil
}
