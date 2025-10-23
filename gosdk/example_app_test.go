package gosdk

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
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

	config := MakeAppchainConfig(42, map[apptypes.ChainType]string{
		// EthereumChainID: args.EthereumBlocksPath, //TODO
		// SolanaChainID:   args.SolBlocksPath, //TODO
	})

	tmp := t.TempDir()

	localDB, err := mdbx.NewMDBX(mdbxlog.New()).
		InMem(tmp).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return txpool.Tables()
		}).
		Open()
	require.NoError(t, err)

	txPool := txpool.NewTxPool[ExampleTransaction[ExampleReceipt], ExampleReceipt](localDB)

	// инициализируем базу на нашей стороне
	appchainDBPath := filepath.Join(tmp, config.AppchainDBPath)
	appchainDB, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(appchainDBPath).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return DefaultTables()
		}).
		Open()
	require.NoError(t, err)

	txStreamDirPath := filepath.Join(tmp, config.TxStreamDir)

	txBatchDB, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(txStreamDirPath).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return TxBucketsTables()
		}).
		Open()
	if err != nil {
		log.Panic().Err(err).Msg("Failed to tx batch mdbx database")
	}

	subscriber, err := NewSubscriber(t.Context(), appchainDB)
	require.NoError(t, err)

	chainDBs, err := NewMultichainStateAccessDB(config.MultichainStateDB)
	require.NoError(t, err)

	multichainDB := NewMultichainStateAccess(chainDBs)

	require.NoError(t, err)

	stateTransition := NewBatchProcesser[ExampleTransaction[ExampleReceipt], ExampleReceipt](
		NewStateTransition(multichainDB),
		multichainDB,
		subscriber,
	)

	log.Info().Msg("Creating appchain...")

	appchainExample := NewAppchain(
		stateTransition,
		func(_ uint64, _ [32]byte, _ [32]byte, _ apptypes.Batch[ExampleTransaction[ExampleReceipt], ExampleReceipt]) *ExampleBlock {
			return &ExampleBlock{}
		},
		txPool,
		config,
		appchainDB,
		subscriber,
		multichainDB,
		txBatchDB,
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

type ExampleBatchProcesser[appTx apptypes.AppTransaction[R], R apptypes.Receipt] struct{}

func (ExampleBatchProcesser[appTx, R]) ProcessBatch(
	_ apptypes.Batch[appTx, R],
	_ kv.RwTx,
) ([]R, []apptypes.ExternalTransaction, error) {
	return nil, nil, nil
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

type StateTransition struct {
	MultiChain *MultichainStateAccess
}

func NewStateTransition(multiChain *MultichainStateAccess) *StateTransition {
	return &StateTransition{
		MultiChain: multiChain,
	}
}

// how to external chains blocks
func (*StateTransition) ProcessBlock(
	_ apptypes.ExternalBlock,
	_ kv.RwTx,
) ([]apptypes.ExternalTransaction, error) {
	return nil, nil
}
