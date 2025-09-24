package receipt

import (
	"errors"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

const ReceiptBucket = "receipts" // tx-hash -> receipt

var ErrNoReceipts = errors.New("no receipts found")

func StoreReceipt[R apptypes.Receipt](tx kv.RwTx, receipt R) error {
	key := receipt.TxHash()

	value, err := cbor.Marshal(receipt)
	if err != nil {
		return err
	}

	return tx.Put(ReceiptBucket, key[:], value)
}

func GetReceipt[R apptypes.Receipt](tx kv.Tx, txHash []byte, receipt R) (R, error) {
	value, err := tx.GetOne(ReceiptBucket, txHash)
	if err != nil {
		return receipt, err
	}

	if len(value) == 0 {
		return receipt, ErrNoReceipts
	}

	if err := cbor.Unmarshal(value, &receipt); err != nil {
		return receipt, err
	}

	return receipt, nil
}
