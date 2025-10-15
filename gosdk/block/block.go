package block

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"reflect"
	"strconv"

	"github.com/fxamacker/cbor/v2"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

const (
	BlockHashBucket         = "blockhash"   // block-hash -> block
	BlockNumberBucket       = "blocknumber" // block-number -> block
	BlockTransactionsBucket = "blocktxs"    // block-number -> txs
)

var _ apptypes.AppchainBlock = (*Block[apptypes.AppTransaction[apptypes.Receipt], apptypes.Receipt])(
	nil,
)

type Block[appTx apptypes.AppTransaction[R], R apptypes.Receipt] struct {
	BlockNumber  uint64   `json:"number"       cbor:"1,keyasint"`
	BlockHash    [32]byte `json:"hash"         cbor:"2,keyasint"`
	BlockRoot    [32]byte `json:"stateroot"    cbor:"3,keyasint"`
	Timestamp    uint64   `json:"timestamp"    cbor:"4,keyasint"`
	Transactions []appTx  `json:"transactions" cbor:"5,keyasint"`
}

// Number implements apptypes.AppchainBlock.
func (b *Block[appTx, R]) Number() uint64 {
	if b == nil {
		return 0
	}

	return b.BlockNumber
}

func (b *Block[appTx, R]) computeTransactionsHash() [32]byte {
	if b == nil {
		return [32]byte{}
	}

	hasher := sha256.New()

	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], b.BlockNumber)

	if _, err := hasher.Write(buf[:]); err != nil {
		return [32]byte{}
	}

	if _, err := hasher.Write(b.BlockRoot[:]); err != nil {
		return [32]byte{}
	}

	binary.BigEndian.PutUint64(buf[:], b.Timestamp)

	if _, err := hasher.Write(buf[:]); err != nil {
		return [32]byte{}
	}

	for _, tx := range b.Transactions {
		txHash := tx.Hash()
		if _, err := hasher.Write(txHash[:]); err != nil {
			return [32]byte{}
		}
	}

	var out [32]byte
	copy(out[:], hasher.Sum(nil))

	return out
}

// Hash implements apptypes.AppchainBlock.
func (b *Block[appTx, R]) Hash() [32]byte {
	if b == nil {
		return [32]byte{}
	}

	out := b.computeTransactionsHash()
	b.BlockHash = out

	return out
}

// StateRoot implements apptypes.AppchainBlock.
func (b *Block[appTx, R]) StateRoot() [32]byte {
	if b == nil {
		return [32]byte{}
	}

	return sha256.Sum256(b.Bytes())
}

// Bytes implements apptypes.AppchainBlock.
// We use CBOR to keep it consistent with storage/other hashes in this package.
func (b *Block[appTx, R]) Bytes() []byte {
	if b == nil {
		return nil
	}

	data, err := cbor.Marshal(*b)
	if err != nil {
		return nil
	}

	return data
}

// convertToFieldsValues converts Block to FieldsValues.
func (b *Block[appTx, R]) convertToFieldsValues() FieldsValues {
	if b == nil {
		b = &Block[appTx, R]{}
	}

	// Field names from `json` tags in declaration order
	var zero Block[appTx, R]

	t := reflect.TypeOf(zero)

	fieldCount := t.NumField()

	fields := make([]string, 0, fieldCount)
	for i := range fieldCount {
		f := t.Field(i)

		name := f.Tag.Get("json")
		if name == "" || name == "-" {
			name = f.Name
		}

		fields = append(fields, name)
	}
	// Values aligned with fields order
	values := []string{
		strconv.FormatUint(b.Number(), 10),
		fmt.Sprintf("0x%x", b.Hash()),
		fmt.Sprintf("0x%x", b.StateRoot()),
		strconv.FormatUint(b.Timestamp, 10),
		strconv.FormatUint(uint64(len(b.Transactions)), 10),
	}

	return FieldsValues{Fields: fields, Values: values}
}

// FieldsValues represents a set of fields of Block and their corresponding values.
type FieldsValues struct {
	Fields []string
	Values []string
}

func NumberToBytes(input uint64) []byte {
	// Create a byte slice of length 8, as uint64 occupies 8 bytes
	b := make([]byte, 8)

	// Encode the uint64 into the byte slice using Big Endian byte order
	binary.BigEndian.PutUint64(b, input)

	return b
}
