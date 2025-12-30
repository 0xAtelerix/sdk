package gosdk

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/library"
	emitterproto "github.com/0xAtelerix/sdk/gosdk/proto"
)

type Storage[AppTx apptypes.AppTransaction[R], R apptypes.Receipt] struct {
	appchainDB     kv.RwDB
	txBatchDB      kv.RoDB
	txPoolDB       kv.RwDB
	txPool         apptypes.TxPoolInterface[AppTx, R]
	multichain     MultichainStateAccessor
	subscriber     *Subscriber
	eventStreamDir string
	txStreamDir    string
}

// NewStorage creates a Storage instance from manually constructed components.
func NewStorage[AppTx apptypes.AppTransaction[R], R apptypes.Receipt](
	appchainDB kv.RwDB,
	txBatchDB kv.RoDB,
	txPoolDB kv.RwDB,
	txPool apptypes.TxPoolInterface[AppTx, R],
	multichain MultichainStateAccessor,
	subscriber *Subscriber,
	eventStreamDir string,
	txStreamDir string,
) *Storage[AppTx, R] {
	return &Storage[AppTx, R]{
		appchainDB:     appchainDB,
		txBatchDB:      txBatchDB,
		txPoolDB:       txPoolDB,
		txPool:         txPool,
		multichain:     multichain,
		subscriber:     subscriber,
		eventStreamDir: eventStreamDir,
		txStreamDir:    txStreamDir,
	}
}

func (s *Storage[AppTx, R]) AppchainDB() kv.RwDB {
	return s.appchainDB
}

func (s *Storage[AppTx, R]) TxBatchDB() kv.RoDB {
	return s.txBatchDB
}

func (s *Storage[AppTx, R]) TxPool() apptypes.TxPoolInterface[AppTx, R] {
	return s.txPool
}

func (s *Storage[AppTx, R]) Multichain() MultichainStateAccessor {
	return s.multichain
}

func (s *Storage[AppTx, R]) Subscriber() *Subscriber {
	return s.subscriber
}

func (s *Storage[AppTx, R]) SetTxBatchDB(db kv.RoDB) {
	s.txBatchDB = db
}

func (s *Storage[AppTx, R]) Close() {
	if s.multichain != nil {
		s.multichain.Close()
	}

	if s.txBatchDB != nil {
		s.txBatchDB.Close()
	}

	if s.txPoolDB != nil {
		s.txPoolDB.Close()
	}

	if s.appchainDB != nil {
		s.appchainDB.Close()
	}
}

// InitDevValidatorSet initializes a single-validator set for local development.
func InitDevValidatorSet(ctx context.Context, db kv.RwDB) error {
	valset := &ValidatorSet{Set: map[ValidatorID]Stake{0: 100}}

	var epochKey [4]byte
	binary.BigEndian.PutUint32(epochKey[:], 1)

	valsetData, err := cbor.Marshal(valset)
	if err != nil {
		return fmt.Errorf("failed to marshal validator set: %w", err)
	}

	return db.Update(ctx, func(tx kv.RwTx) error {
		return tx.Put(ValsetBucket, epochKey[:], valsetData)
	})
}

func WriteBlock(rwtx kv.RwTx, blockNumber uint64, blockBytes []byte) error {
	number := make([]byte, 8)
	binary.BigEndian.PutUint64(number, blockNumber)

	return rwtx.Put(BlocksBucket, number, blockBytes)
}

func WriteBlockTransactions[appTx apptypes.AppTransaction[R], R apptypes.Receipt](
	rwtx kv.RwTx,
	blockNumber uint64,
	txs []appTx,
) error {
	blockNumBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(blockNumBytes, blockNumber)

	txsBytes, err := cbor.Marshal(txs)
	if err != nil {
		return fmt.Errorf("%w: %w", library.ErrTransactionsMarshalling, err)
	}

	if err := rwtx.Put(BlockTransactionsBucket, blockNumBytes, txsBytes); err != nil {
		return fmt.Errorf("%w: %w", library.ErrBlockTransactionsWrite, err)
	}

	for i, tx := range txs {
		txHash := tx.Hash()
		lookupEntry := make([]byte, 12)
		binary.BigEndian.PutUint64(lookupEntry[0:8], blockNumber)
		binary.BigEndian.PutUint32(lookupEntry[8:12], uint32(i))

		if err := rwtx.Put(TxLookupBucket, txHash[:], lookupEntry); err != nil {
			return fmt.Errorf("%w (tx %x): %w", library.ErrTransactionLookupWrite, txHash[:4], err)
		}
	}

	return nil
}

func WriteLastBlock(rwtx kv.RwTx, number uint64, hash [32]byte) error {
	value := make([]byte, 8+32)
	binary.BigEndian.PutUint64(value[:8], number)
	copy(value[8:], hash[:])

	return rwtx.Put(ConfigBucket, []byte(LastBlockKey), value)
}

func GetLastBlock(tx kv.Tx) (uint64, [32]byte, error) {
	value, err := tx.GetOne(ConfigBucket, []byte(LastBlockKey))
	if err != nil {
		return 0, [32]byte{}, err
	}

	if len(value) != 8+32 {
		return 0, [32]byte{}, nil
	}

	number := binary.BigEndian.Uint64(value[:8])

	return number, ([32]byte)(value[8:]), err
}

func WriteExternalTransactions(
	dbTx kv.RwTx,
	blockNumber uint64,
	txs []apptypes.ExternalTransaction,
) ([32]byte, error) {
	root := Merklize(txs)

	value, err := cbor.Marshal(txs)
	if err != nil {
		log.Error().Err(err).Msg("Transaction serialization failed")

		return [32]byte{}, fmt.Errorf("transaction serialization failed: %w", err)
	}

	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, blockNumber)

	if err = dbTx.Put(ExternalTxBucket, key, value); err != nil {
		return [32]byte{}, fmt.Errorf("can't write external transactions to the DB: error %w", err)
	}

	return root, nil
}

func ReadExternalTransactions(
	tx kv.Tx,
	blockNumber uint64,
) ([]apptypes.ExternalTransaction, error) {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, blockNumber)

	value, err := tx.GetOne(ExternalTxBucket, key)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", library.ErrExternalTransactionsGet, err)
	}

	if len(value) == 0 {
		return []apptypes.ExternalTransaction{}, nil
	}

	var txs []apptypes.ExternalTransaction
	if err := cbor.Unmarshal(value, &txs); err != nil {
		return nil, fmt.Errorf("%w: %w", library.ErrExternalTransactionsUnmarshal, err)
	}

	return txs, nil
}

func Merklize(_ []apptypes.ExternalTransaction) [32]byte {
	return [32]byte{}
}

func WriteCheckpoint(ctx context.Context, dbTx kv.RwTx, checkpoint apptypes.Checkpoint) error {
	logger := log.Ctx(ctx)

	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, checkpoint.BlockNumber)

	value, err := cbor.Marshal(checkpoint)
	if err != nil {
		logger.Error().Err(err).Msg("Checkpoint serialization failed")

		return fmt.Errorf("checkpoint serialization failed: %w", err)
	}

	return dbTx.Put(CheckpointBucket, key, value)
}

func CheckpointToProto(cp apptypes.Checkpoint) *emitterproto.CheckpointResponse_Checkpoint {
	return &emitterproto.CheckpointResponse_Checkpoint{
		LatestBlockNumber:  cp.BlockNumber,
		StateRoot:          cp.StateRoot[:],
		BlockHash:          cp.BlockHash[:],
		ExternalTxRootHash: cp.ExternalTransactionsRoot[:],
	}
}

func WriteSnapshotPosition(rwtx kv.RwTx, epoch uint32, pos int64) error {
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, epoch)

	val := make([]byte, 8)
	binary.BigEndian.PutUint64(val, uint64(pos))

	return rwtx.Put(Snapshot, key, val)
}

func ReadSnapshotPosition(tx kv.Tx, epoch uint32) (int64, error) {
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, epoch)

	val, err := tx.GetOne(Snapshot, key)
	if err != nil {
		return 0, err
	}

	if len(val) != 8 {
		return 0, nil // default to beginning
	}

	return int64(binary.BigEndian.Uint64(val)), nil
}

func GetLastStreamPositions(
	ctx context.Context,
	appchainDB kv.RoDB,
) (int64, uint32, error) {
	startEventPos := int64(8)
	epoch := uint32(1)

	err := appchainDB.View(ctx, func(tx kv.Tx) error {
		c, err := tx.Cursor(Snapshot)
		if err != nil {
			return err
		}

		k, v, err := c.Last()
		if err != nil {
			return err
		}

		if len(k) != 4 {
			return nil
		}

		epoch = binary.BigEndian.Uint32(k)

		if len(v) != 8 {
			return nil
		}

		startEventPos = int64(binary.BigEndian.Uint64(v))

		return nil
	})
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get stream positions: %w", err)
	}

	return startEventPos, epoch, nil
}

func WaitFile(ctx context.Context, filePath string, logger *zerolog.Logger) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		_, err := os.Stat(filePath)
		if err == nil {
			return nil
		}

		logger.Warn().Err(err).Str("file", filePath).Msg("waiting file")

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Continue loop to check file again
		}
	}
}
