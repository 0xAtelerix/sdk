package gosdk

import (
	"context"
	"github.com/0xAtelerix/sdk/gosdk/txpool"
	"github.com/0xAtelerix/sdk/gosdk/types"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"time"
)

type ExampleTransaction struct {
	Sender string
	Value  int
}

type ExapleBatchProcesser[appTx types.AppTransaction] struct{}

func (b ExapleBatchProcesser[appTx]) ProcessBatch(batch types.Batch[appTx], dbtx kv.RwTx) ([]types.ExternalTransaction, error) {
	return nil, nil
}

type ExampleBlock struct{}

func (e *ExampleBlock) Number() uint64 {
	return 0
}
func (e *ExampleBlock) Hash() [32]byte {
	return [32]byte{}
}
func (e *ExampleBlock) StateRoot() [32]byte {
	return [32]byte{}
}
func (e *ExampleBlock) Bytes() []byte {
	return []byte{}
}
func (e *ExampleBlock) Marshal() ([]byte, error) {
	return nil, nil
}
func (e *ExampleBlock) Unmarshal([]byte) error {
	return nil
}

func ExampleAppchain() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	config := MakeAppchainConfig(42)

	stateTransition := BatchProcesser[*ExampleTransaction]{}

	tmp := os.TempDir() + "/txpool_test"
	defer os.RemoveAll(tmp)
	localDB, err := mdbx.NewMDBX(mdbxlog.New()).InMem(tmp).WithTableCfg(func(defaultBuckets kv.TableCfg) kv.TableCfg {
		return txpool.TxPoolTables
	}).
		Open()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to local mdbx database")
	}
	txPool := txpool.NewTxPool[*ExampleTransaction](localDB)

	log.Info().Msg("Starting appchain...")
	appchainExample, err := NewAppchain(
		stateTransition,
		func(blockNumber uint64, stateRoot [32]byte, previousBlockHash [32]byte, txsBatch types.Batch[*ExampleTransaction]) *ExampleBlock {
			return &ExampleBlock{}
		},
		txPool,
		config)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to start appchain")
	}

	ctx, cancel := context.WithTimeout(context.TODO(), time.Minute)
	defer cancel()
	// Run appchain in goroutine
	runErr := make(chan error, 1)
	go func() {
		runErr <- appchainExample.Run(ctx)
	}()

	err = <-runErr

	log.Info().Err(err).Msg("Appchain exited")

}
