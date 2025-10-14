package block

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

type testReceipt struct{}

func (testReceipt) TxHash() [32]byte                 { return [32]byte{} }
func (testReceipt) Status() apptypes.TxReceiptStatus { return apptypes.ReceiptUnknown }
func (testReceipt) Error() string                    { return "" }

type testTx struct {
	hash [32]byte
}

func (t testTx) Hash() [32]byte { return t.hash }

func (testTx) Process(kv.RwTx) (testReceipt, []apptypes.ExternalTransaction, error) {
	return testReceipt{}, nil, nil
}

func filled(b byte) [32]byte {
	var out [32]byte
	for i := range out {
		out[i] = b
	}

	return out
}

func newTestTx(seed byte) testTx {
	var h [32]byte

	h[0] = seed

	return testTx{hash: h}
}

func buildBlock(
	number uint64,
	root [32]byte,
	timestamp uint64,
	txs []testTx,
) *Block[testTx, testReceipt] {
	return &Block[testTx, testReceipt]{
		BlockNumber:  number,
		BlockRoot:    root,
		Timestamp:    timestamp,
		Transactions: txs,
	}
}

func encodeBlock(b *Block[testTx, testReceipt]) []byte {
	raw, err := cbor.Marshal(*b)
	if err != nil {
		return nil
	}

	return raw
}

func expectedHash(b *Block[testTx, testReceipt]) [32]byte {
	hasher := sha256.New()

	var buf [8]byte

	binary.BigEndian.PutUint64(buf[:], b.BlockNumber)
	_, _ = hasher.Write(buf[:])
	_, _ = hasher.Write(b.BlockRoot[:])
	binary.BigEndian.PutUint64(buf[:], b.Timestamp)
	_, _ = hasher.Write(buf[:])

	for _, tx := range b.Transactions {
		txHash := tx.Hash()
		_, _ = hasher.Write(txHash[:])
	}

	var out [32]byte
	copy(out[:], hasher.Sum(nil))

	return out
}

func expectedStateRoot(b *Block[testTx, testReceipt]) [32]byte {
	raw := encodeBlock(b)

	return sha256.Sum256(raw)
}
