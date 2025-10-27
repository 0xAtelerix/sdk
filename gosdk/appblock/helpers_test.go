package appblock

import (
	"context"
	"crypto/sha256"
	"path/filepath"
	"reflect"
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

type helperTestReceipt struct{}

func (helperTestReceipt) TxHash() [32]byte                 { return [32]byte{} }
func (helperTestReceipt) Status() apptypes.TxReceiptStatus { return apptypes.ReceiptConfirmed }
func (helperTestReceipt) Error() string                    { return "" }

type helperTestTx[R helperTestReceipt] struct {
	From  string
	Value int
}

func (t helperTestTx[R]) Hash() [32]byte {
	return sha256.Sum256([]byte(t.From + strconv.Itoa(t.Value)))
}

func (helperTestTx[R]) Process(kv.RwTx) (R, []apptypes.ExternalTransaction, error) {
	var r R

	return r, nil, nil
}

type templateWithTxs struct {
	Txs []helperTestTx[helperTestReceipt]
}

type templateWithoutTxs struct {
	Number string
}

type helperBlock struct {
	Value string `json:"value"`
}

func (*helperBlock) Number() uint64      { return 0 }
func (*helperBlock) Hash() [32]byte      { return [32]byte{} }
func (*helperBlock) StateRoot() [32]byte { return [32]byte{} }
func (*helperBlock) Bytes() []byte       { return nil }

func TestStructValueFrom(t *testing.T) {
	cases := map[string]struct {
		input    any
		expectOK bool
	}{
		"nil":                {input: nil, expectOK: false},
		"non-struct":         {input: 1, expectOK: false},
		"struct":             {input: templateWithoutTxs{}, expectOK: true},
		"pointer":            {input: &templateWithoutTxs{}, expectOK: true},
		"pointer to pointer": {input: new(*templateWithoutTxs), expectOK: false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			value, ok := structValueFrom(tc.input)
			if !tc.expectOK {
				require.False(t, ok)

				return
			}

			require.True(t, ok)
			require.Equal(t, reflect.Struct, value.Kind())
		})
	}
}

func TestTxsField(t *testing.T) {
	value := reflect.ValueOf(templateWithTxs{})
	field, ok := txsField(value)
	require.True(t, ok)
	require.Equal(t, reflect.Slice, field.Kind())

	_, ok = txsField(reflect.ValueOf(templateWithoutTxs{}))
	require.False(t, ok)
}

func TestExtractTransactions(t *testing.T) {
	transactions := []helperTestTx[helperTestReceipt]{
		{From: "alice", Value: 1},
	}

	block := &templateWithTxs{Txs: transactions}
	txs, hasField, fieldNil := extractTransactions[helperTestTx[helperTestReceipt]](block)
	require.True(t, hasField)
	require.False(t, fieldNil)
	require.Equal(t, transactions, txs)

	blockNil := &templateWithTxs{Txs: nil}
	_, hasField, fieldNil = extractTransactions[helperTestTx[helperTestReceipt]](blockNil)
	require.True(t, hasField)
	require.True(t, fieldNil)

	_, hasField, fieldNil = extractTransactions[helperTestTx[helperTestReceipt]](
		&templateWithoutTxs{},
	)
	require.False(t, hasField)
	require.False(t, fieldNil)
}

func TestDecodeBlockIntoTarget(t *testing.T) {
	db := newHelperTestDB(t, kv.TableCfg{gosdk.BlocksBucket: {}})

	original := &helperBlock{Value: "ok"}
	require.NoError(
		t,
		storeCBORValue(context.Background(), db, gosdk.BlocksBucket, 1, original, "encode"),
	)

	decoded := &helperBlock{}
	require.NoError(t, decodeBlockIntoTarget(context.Background(), db, 1, decoded))
	require.Equal(t, original.Value, decoded.Value)

	err := decodeBlockIntoTarget(context.Background(), db, 2, decoded)
	require.ErrorIs(t, err, gosdk.ErrAppBlockNotFound)
}

func TestStoreAndFetchCBORValue(t *testing.T) {
	db := newHelperTestDB(t, kv.TableCfg{gosdk.BlocksBucket: {}})

	type payload struct {
		Value string `cbor:"1,keyasint"`
	}

	original := payload{Value: "data"}
	require.NoError(
		t,
		storeCBORValue(context.Background(), db, gosdk.BlocksBucket, 10, original, "encode"),
	)

	bytes, err := fetchBucketValue(context.Background(), db, 10)
	require.NoError(t, err)

	var decoded payload
	require.NoError(t, cbor.Unmarshal(bytes, &decoded))
	require.Equal(t, original, decoded)

	_, err = fetchBucketValue(context.Background(), db, 99)
	require.ErrorIs(t, err, gosdk.ErrAppBlockNotFound)

	_, err = fetchBucketValue(context.Background(), nil, 1)
	require.ErrorIs(t, err, gosdk.ErrAppBlockDatabaseNil)
}

func newHelperTestDB(t *testing.T, tables kv.TableCfg) kv.RwDB {
	t.Helper()

	toCfg := func(kv.TableCfg) kv.TableCfg { return tables }
	dir := t.TempDir()

	db, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(filepath.Join(dir, "helperdb")).
		WithTableCfg(toCfg).
		Open()
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Close()
	})

	return db
}
