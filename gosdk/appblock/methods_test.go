package appblock

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"path/filepath"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk"
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

func TestGetAppBlockByNumber_Success(t *testing.T) {
	type payload struct {
		ID   string `json:"id"`
		Note string
	}

	original := payload{ID: "123", Note: "ok"}
	encoded, err := cbor.Marshal(original)
	require.NoError(t, err)

	target := &payload{}

	fv, err := GetAppBlockByNumber(uint64(15), encoded, target)
	require.NoError(t, err)
	require.Equal(t, []string{"id", "Note"}, fv.Fields)
	require.Equal(t, []string{"123", "ok"}, fv.Values)
	require.Equal(t, original, *target)
}

func TestGetAppBlockByNumber_NilTarget(t *testing.T) {
	encoded, err := cbor.Marshal(struct{}{})
	require.NoError(t, err)

	fv, err := GetAppBlockByNumber[*string](uint64(15), encoded, nil)
	require.Error(t, err)
	require.Empty(t, fv.Fields)
	require.Empty(t, fv.Values)
}

func TestGetAppBlockByNumber_NonPointerTarget(t *testing.T) {
	encoded, err := cbor.Marshal(struct{}{})
	require.NoError(t, err)

	fv, err := GetAppBlockByNumber(uint64(15), encoded, struct{}{})
	require.Error(t, err)
	require.Empty(t, fv.Fields)
	require.Empty(t, fv.Values)
}

func TestStoreAppBlockAndRetrieve(t *testing.T) {
	type testTargetBlock struct {
		Height uint64 `json:"height"`
		Miner  string `json:"miner"`
		Note   string `json:"note"`
	}

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

	block := &testTargetBlock{Height: 99, Miner: "bob", Note: "testing"}
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

	target := &testTargetBlock{}
	fv, err := GetAppBlockByNumber(uint64(42), payload, target)
	require.NoError(t, err)
	require.Equal(t, []string{"height", "miner", "note"}, fv.Fields)
	require.Equal(t, []string{"99", "bob", "testing"}, fv.Values)
	require.Equal(t, block.Height, target.Height)
	require.Equal(t, block.Miner, target.Miner)
	require.Equal(t, block.Note, target.Note)
}

func TestUnmarshallIntoTarget_Success(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
		Note string `json:"note"`
	}

	original := payload{Name: "carol", Note: "ok"}
	encoded, err := cbor.Marshal(original)
	require.NoError(t, err)

	target := &payload{}

	require.NoError(t, unmarshallIntoTarget(encoded, target))
	require.Equal(t, original, *target)
}

func TestUnmarshallIntoTarget_ValidationErrors(t *testing.T) {
	encoded, err := cbor.Marshal(struct{ Value int }{Value: 42})
	require.NoError(t, err)

	err = unmarshallIntoTarget[any](encoded, nil)
	require.ErrorContains(t, err, "target cannot be nil")

	err = unmarshallIntoTarget(encoded, struct{}{})
	require.ErrorContains(t, err, "target must be a pointer")

	var target *struct{ Value int }

	err = unmarshallIntoTarget(encoded, target)
	require.ErrorContains(t, err, "target must be a non-nil pointer")
}

func TestUnmarshallIntoTarget_EmptyPayload(t *testing.T) {
	type payload struct {
		Value int `json:"value"`
	}

	target := &payload{}
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

type testBlockTemplate struct {
	Number string   `json:"number"`
	Txs    []testTx `json:"txs"`
	Note   string   `json:"note"`
}

func TestGetTransactionsFromBlock_WithEmbeddedTransactions(t *testing.T) {
	db := newTestDB(t, kv.TableCfg{
		gosdk.BlocksBucket: {},
	})

	block := &testBlockTemplate{
		Number: "1",
		Txs: []testTx{
			{From: "alice", To: "bob"},
			{From: "carol", To: "dan"},
		},
		Note: "embedded",
	}

	require.NoError(t, StoreAppBlock(context.Background(), db, 1, block))

	txs, ok, err := GetTransactionsFromBlock[testTx](
		context.Background(),
		db,
		1,
		&testBlockTemplate{},
	)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, block.Txs, txs)
}

func TestGetTransactionsFromBlock_NilTxsField(t *testing.T) {
	db := newTestDB(t, kv.TableCfg{
		gosdk.BlocksBucket: {},
	})

	block := &testBlockTemplate{
		Number: "2",
		Txs:    nil,
		Note:   "nil txs",
	}

	require.NoError(t, StoreAppBlock(context.Background(), db, 2, block))

	txs, ok, err := GetTransactionsFromBlock[testTx](
		context.Background(),
		db,
		2,
		&testBlockTemplate{},
	)
	require.NoError(t, err)
	require.True(t, ok)
	require.Empty(t, txs)
}

func TestGetTransactionsFromBlock_MissingTransactionsField(t *testing.T) {
	type blockWithoutTxs struct {
		Number string `json:"number"`
	}

	db := newTestDB(t, kv.TableCfg{
		gosdk.BlocksBucket: {},
	})

	require.NoError(t, StoreAppBlock(context.Background(), db, 4, &blockWithoutTxs{Number: "4"}))

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
		&testBlockTemplate{},
	)
	require.Error(t, err)
	require.False(t, ok)
	require.Nil(t, txs)
	require.ErrorIs(t, err, errBlockNotFound)
}

func TestGetTransactionsFromBlock_InvalidTemplate(t *testing.T) {
	db := newTestDB(t, kv.TableCfg{
		gosdk.BlocksBucket: {},
	})

	txs, ok, err := GetTransactionsFromBlock[testTx, blockTestReceipt](
		context.Background(),
		db,
		1,
		123,
	)
	require.Error(t, err)
	require.False(t, ok)
	require.Nil(t, txs)
	require.ErrorContains(t, err, "unsupported block payload type")
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
