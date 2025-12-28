package gosdk

import (
	"context"
	"crypto/sha256"
	"database/sql"
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
)

func TestExampleAppchain(t *testing.T) {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	ctx := log.Logger.WithContext(t.Context())

	tmp := t.TempDir()

	// Create events directory (normally created by pelacli)
	eventsPath := EventsPath(tmp)
	require.NoError(t, os.MkdirAll(eventsPath, 0o755))

	// Create txbatch database (normally created by pelacli)
	// We create and close it so InitApp can open it
	txBatchPath := TxBatchPath(tmp)
	require.NoError(t, os.MkdirAll(txBatchPath, 0o755))

	txBatchDB, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(txBatchPath).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return TxBucketsTables()
		}).
		Open()
	require.NoError(t, err)
	txBatchDB.Close() // Close it so InitApp can open it

	storage, config, err := InitApp[ExampleTransaction[ExampleReceipt], ExampleReceipt](
		ctx,
		InitConfig{
			ChainID:     DefaultAppchainID,
			DataDir:     tmp,
			EmitterPort: DefaultEmitterPort,
		},
	)
	require.NoError(t, err)

	log.Info().Msg("Creating appchain...")

	// Create no-op multichain for test (test doesn't process external blocks)
	multichain := NewMultichainStateAccessSQL(make(map[apptypes.ChainType]*sql.DB))

	appchainExample := NewAppchain(
		storage,
		config,
		NewDefaultBatchProcessor[ExampleTransaction[ExampleReceipt], ExampleReceipt](
			NewExtBlockProcessor(multichain),
			multichain,
			storage.Subscriber(),
		),
		func(_ uint64, _ [32]byte, _ [32]byte, _ apptypes.Batch[ExampleTransaction[ExampleReceipt], ExampleReceipt]) *ExampleBlock {
			return &ExampleBlock{}
		},
	)

	defer appchainExample.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
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
	MultiChain MultichainStateAccessor
}

func NewExtBlockProcessor(multiChain MultichainStateAccessor) *ExtBlockProcessor {
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
