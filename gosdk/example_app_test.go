//nolint:testableexamples // an example app
package gosdk

import (
	"context"
	"crypto/sha256"
	"os"
	"strconv"
	"time"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/txpool"
)

type ExampleTransaction struct {
	Sender string
	Value  int
}

func (c ExampleTransaction) Hash() [32]byte {
	s := c.Sender + strconv.Itoa(c.Value)

	return sha256.Sum256([]byte(s))
}

func (ExampleTransaction) Process(_ kv.RwTx) ([]apptypes.ExternalTransaction, error) {
	return nil, nil
}

type ExampleBatchProcesser[appTx apptypes.AppTransaction] struct{}

func (ExampleBatchProcesser[appTx]) ProcessBatch(
	_ apptypes.Batch[appTx],
	_ kv.RwTx,
) ([]apptypes.ExternalTransaction, error) {
	return nil, nil
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

func ExampleAppchain() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	config := MakeAppchainConfig(42)

	stateTransition := BatchProcesser[ExampleTransaction]{}

	tmp := os.TempDir() + "/txpool_test"

	defer func() {
		dirErr := os.RemoveAll(tmp)
		if dirErr != nil {
			log.Warn().Err(dirErr).Msgf("Failed to remove: %q", tmp)
		}
	}()

	localDB, err := mdbx.NewMDBX(mdbxlog.New()).
		InMem(tmp).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return txpool.Tables()
		}).
		Open()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to local mdbx database")
	}

	txPool := txpool.NewTxPool[ExampleTransaction](localDB)

	// инициализируем базу на нашей стороне
	appchainDB, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(config.AppchainDBPath).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return DefaultTables()
		}).
		Open()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to appchain mdbx database")
	}

	log.Info().Msg("Starting appchain...")

	appchainExample, err := NewAppchain(
		stateTransition,
		func(_ uint64, _ [32]byte, _ [32]byte, _ apptypes.Batch[ExampleTransaction]) *ExampleBlock {
			return &ExampleBlock{}
		},
		txPool,
		config,
		appchainDB,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to start appchain")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	// Run appchain in goroutine
	runErr := make(chan error, 1)

	go func() {
		runErr <- appchainExample.Run(ctx, nil)
	}()

	err = <-runErr

	log.Info().Err(err).Msg("Appchain exited")
}
