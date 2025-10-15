package block

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

type testReceiptError struct{}

func (testReceiptError) TxHash() [32]byte                 { return [32]byte{} }
func (testReceiptError) Status() apptypes.TxReceiptStatus { return apptypes.ReceiptUnknown }
func (testReceiptError) Error() string                    { return "" }

// TODO consider to import testTx from receipt package instead of redefining it here
type testTx struct {
	HashValue [32]byte `cbor:"1,keyasint"`
}

func (t testTx) Hash() [32]byte { return t.HashValue }

func (testTx) Process(kv.RwTx) (testReceiptError, []apptypes.ExternalTransaction, error) {
	return testReceiptError{}, nil, nil
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

	return testTx{HashValue: h}
}

func buildBlock(
	number uint64,
	root [32]byte,
	timestamp uint64,
	txs []testTx,
) *Block[testTx, testReceiptError] {
	block := &Block[testTx, testReceiptError]{
		BlockNumber:  number,
		BlockRoot:    root,
		Timestamp:    timestamp,
		Transactions: txs,
	}

	block.Hash()

	return block
}

func encodeBlock(b *Block[testTx, testReceiptError]) []byte {
	raw, err := cbor.Marshal(*b)
	if err != nil {
		return nil
	}

	return raw
}

func expectedHash(b *Block[testTx, testReceiptError]) [32]byte {
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

func expectedStateRoot(b *Block[testTx, testReceiptError]) [32]byte {
	raw := encodeBlock(b)

	return sha256.Sum256(raw)
}

func encodeBlockWithCorruptedTransactions(b *Block[testTx, testReceiptError]) []byte {
	type corruptedBlock struct {
		BlockNumber  uint64   `cbor:"1,keyasint"`
		BlockHash    [32]byte `cbor:"2,keyasint"`
		BlockRoot    [32]byte `cbor:"3,keyasint"`
		Timestamp    uint64   `cbor:"4,keyasint"`
		Transactions []string `cbor:"5,keyasint"`
	}

	payload := corruptedBlock{
		BlockNumber:  b.BlockNumber,
		BlockHash:    b.BlockHash,
		BlockRoot:    b.BlockRoot,
		Timestamp:    b.Timestamp,
		Transactions: []string{"corrupted"},
	}

	raw, err := cbor.Marshal(payload)
	if err != nil {
		return nil
	}

	return raw
}
