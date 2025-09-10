package gosdk

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/txpool"
)

func TestExampleAppchain(t *testing.T) {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	config := MakeAppchainConfig(42)

	stateTransition := BatchProcesser[ExampleTransaction[ExampleReceipt], ExampleReceipt]{}

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
	appchainDB, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(config.AppchainDBPath).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return DefaultTables()
		}).
		Open()
	require.NoError(t, err)

	subscriber, err := NewSubscriber(t.Context(), appchainDB)
	require.NoError(t, err)

	log.Info().Msg("Starting appchain...")

	appchainExample, err := NewAppchain(
		stateTransition,
		func(_ uint64, _ [32]byte, _ [32]byte, _ apptypes.Batch[ExampleTransaction[ExampleReceipt], ExampleReceipt]) *ExampleBlock {
			return &ExampleBlock{}
		},
		txPool,
		config,
		appchainDB,
		subscriber,
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	// Run appchain in goroutine
	runErr := make(chan error, 1)

	go func() {
		runErr <- appchainExample.Run(ctx, nil)
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

func (r ExampleReceipt) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func (r ExampleReceipt) Unmarshal(data []byte) error {
	return json.Unmarshal(data, &r)
}

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

func (*ExampleBlock) Marshal() ([]byte, error) {
	return nil, nil
}

func (*ExampleBlock) Unmarshal([]byte) error {
	return nil
}
