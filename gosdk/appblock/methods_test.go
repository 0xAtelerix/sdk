package appblock

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk"
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

var _ apptypes.AppchainBlock = (*testBlock)(nil)

type testBlock struct {
	BlockNumber uint64   `json:"number"`
	Miner       string   `json:"miner"`
	Note        string   `json:"note"`
	Txs         []testTx `json:"txs"`
}

func (b *testBlock) Number() uint64 {
	return b.BlockNumber
}

func (*testBlock) Hash() [32]byte      { return [32]byte{} }
func (*testBlock) StateRoot() [32]byte { return [32]byte{} }
func (*testBlock) Bytes() []byte       { return nil }

var _ apptypes.AppchainBlock = (*blockWithoutTxs)(nil)

type blockWithoutTxs struct {
	BlockNumber string `json:"number"`
}

func (b *blockWithoutTxs) Number() uint64 {
	if b == nil || b.BlockNumber == "" {
		return 0
	}

	n, err := strconv.ParseUint(b.BlockNumber, 10, 64)
	if err != nil {
		return 0
	}

	return n
}

func (*blockWithoutTxs) Hash() [32]byte      { return [32]byte{} }
func (*blockWithoutTxs) StateRoot() [32]byte { return [32]byte{} }
func (*blockWithoutTxs) Bytes() []byte       { return nil }

type valueBlock struct {
	BlockNumber uint64 `json:"number"`
}

func (b valueBlock) Number() uint64    { return b.BlockNumber }
func (valueBlock) Hash() [32]byte      { return [32]byte{} }
func (valueBlock) StateRoot() [32]byte { return [32]byte{} }
func (valueBlock) Bytes() []byte       { return nil }

func TestGetAppBlockByNumber_Success(t *testing.T) {
	original := &testBlock{
		BlockNumber: 123,
		Miner:       "alice",
		Note:        "ok",
		Txs:         nil,
	}

	encoded, err := cbor.Marshal(original)
	require.NoError(t, err)

	target := &testBlock{}

	fv, err := GetAppBlockByNumber(uint64(15), encoded, target)
	require.NoError(t, err)
	require.Equal(t, []string{"number", "miner", "note", "txs"}, fv.Fields)
	require.Equal(t, []string{"123", "alice", "ok", "[]"}, fv.Values)
	require.Equal(t, original.BlockNumber, target.BlockNumber)
	require.Equal(t, original.Miner, target.Miner)
	require.Equal(t, original.Note, target.Note)
}

func TestGetAppBlockByNumber_NilTarget(t *testing.T) {
	encoded, err := cbor.Marshal(&testBlock{})
	require.NoError(t, err)

	var target *testBlock

	fv, err := GetAppBlockByNumber(uint64(15), encoded, target)
	require.Error(t, err)
	require.Empty(t, fv.Fields)
	require.Empty(t, fv.Values)
}

func TestStoreAppBlockAndRetrieve(t *testing.T) {
	dir := t.TempDir()

	db, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(filepath.Join(dir, "appblockdb")).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return kv.TableCfg{
				gosdk.BlocksBucket: {},
			}
		}).
		Open()
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Close()
	})

	block := &testBlock{BlockNumber: 99, Miner: "bob", Note: "testing"}
	require.NoError(t, StoreAppBlock(context.Background(), db, 42, block))

	var payload []byte // target payload

	err = db.View(context.Background(), func(tx kv.Tx) error {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, 42)

		var getErr error

		payload, getErr = tx.GetOne(gosdk.BlocksBucket, key)

		return getErr
	})
	require.NoError(t, err)
	require.NotEmpty(t, payload)

	target := &testBlock{}
	fv, err := GetAppBlockByNumber(uint64(42), payload, target)
	require.NoError(t, err)
	require.Equal(t, []string{"number", "miner", "note", "txs"}, fv.Fields)
	require.Equal(t, []string{"99", "bob", "testing", "[]"}, fv.Values)
	require.Equal(t, block.BlockNumber, target.BlockNumber)
	require.Equal(t, block.Miner, target.Miner)
	require.Equal(t, block.Note, target.Note)
}

func TestUnmarshallIntoTarget_Success(t *testing.T) {
	original := &testBlock{
		BlockNumber: 7,
		Note:        "ok",
	}

	encoded, err := cbor.Marshal(original)
	require.NoError(t, err)

	target := &testBlock{}

	require.NoError(t, unmarshallIntoTarget(encoded, target))
	require.Equal(t, original.BlockNumber, target.BlockNumber)
	require.Equal(t, original.Note, target.Note)
}

func TestUnmarshallIntoTarget_EmptyPayload(t *testing.T) {
	target := &testBlock{}
	err := unmarshallIntoTarget([]byte{}, target)
	require.ErrorContains(t, err, "block payload is empty")
}

type testTx struct {
	From string `json:"from" cbor:"1,keyasint"`
	To   string `json:"to"   cbor:"2,keyasint"`
}

func (t testTx) Hash() [32]byte {
	return sha256.Sum256([]byte(t.From + t.To))
}

func (t testTx) Process(kvTx kv.RwTx) (blockTestReceipt, []apptypes.ExternalTransaction, error) {
	ext := make([]apptypes.ExternalTransaction, 0, 1)

	if kvTx != nil {
		hash := t.Hash()
		ext = append(ext, apptypes.ExternalTransaction{Tx: hash[:]})
	}

	var err error
	if t.From == "" {
		err = errMissingSender
	}

	return blockTestReceipt{hash: t.Hash()}, ext, err
}

type blockTestReceipt struct {
	hash [32]byte
}

func (r blockTestReceipt) TxHash() [32]byte {
	return r.hash
}

func (blockTestReceipt) Status() apptypes.TxReceiptStatus {
	return apptypes.ReceiptConfirmed
}

func (blockTestReceipt) Error() string {
	return ""
}

func TestGetTransactionsFromBlock_WithEmbeddedTransactions(t *testing.T) {
	db := newTestDB(t, kv.TableCfg{
		gosdk.BlocksBucket: {},
	})

	block := &testBlock{
		BlockNumber: 1,
		Txs: []testTx{
			{From: "alice", To: "bob"},
			{From: "carol", To: "dan"},
		},
		Note: "embedded",
	}

	require.NoError(t, StoreAppBlock(context.Background(), db, 1, block))

	txs, ok, err := GetTransactionsFromBlock[testTx, blockTestReceipt](
		context.Background(),
		db,
		1,
		&testBlock{},
	)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, block.Txs, txs)
}

func TestGetTransactionsFromBlock_NilTxsField(t *testing.T) {
	db := newTestDB(t, kv.TableCfg{
		gosdk.BlocksBucket: {},
	})

	block := &testBlock{
		BlockNumber: 2,
		Txs:         nil,
		Note:        "nil txs",
	}

	require.NoError(t, StoreAppBlock(context.Background(), db, 2, block))

	txs, ok, err := GetTransactionsFromBlock[testTx, blockTestReceipt](
		context.Background(),
		db,
		2,
		&testBlock{},
	)
	require.NoError(t, err)
	require.True(t, ok)
	require.Empty(t, txs)
}

func TestGetTransactionsFromBlock_MissingTransactionsField(t *testing.T) {
	db := newTestDB(t, kv.TableCfg{
		gosdk.BlocksBucket: {},
	})

	require.NoError(
		t,
		StoreAppBlock(context.Background(), db, 4, &blockWithoutTxs{BlockNumber: "4"}),
	)

	txs, ok, err := GetTransactionsFromBlock[testTx, blockTestReceipt](
		context.Background(),
		db,
		4,
		&blockWithoutTxs{},
	)
	require.Error(t, err)
	require.False(t, ok)
	require.Nil(t, txs)
	require.ErrorIs(t, err, errTransactionsMissing)
}

func TestGetTransactionsFromBlock_BlockNotFound(t *testing.T) {
	db := newTestDB(t, kv.TableCfg{
		gosdk.BlocksBucket: {},
	})

	txs, ok, err := GetTransactionsFromBlock[testTx, blockTestReceipt](
		context.Background(),
		db,
		99,
		&testBlock{},
	)
	require.Error(t, err)
	require.False(t, ok)
	require.Nil(t, txs)
	require.ErrorIs(t, err, errBlockNotFound)
}

func newTestDB(t *testing.T, tables kv.TableCfg) kv.RwDB {
	t.Helper()

	dir := t.TempDir()

	db, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(filepath.Join(dir, "appblockdb")).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return tables
		}).
		Open()
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Close()
	})

	return db
}
