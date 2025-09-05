package receipt

import (
	"encoding/json"
	"errors"

	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

const ReceiptBucket = "receipts" // tx-hash -> receipt

var ErrNoReceipts = errors.New("no receipts found")

func StoreReceipt[R apptypes.Receipt](tx kv.RwTx, receipt R) error {
	key := receipt.TxHash()

	value, err := json.Marshal(receipt)
	if err != nil {
		return err
	}

	return tx.Put(ReceiptBucket, key[:], value)
}

// Only used to test cases as of now, For user we have rpc method
func getReceipt[R apptypes.Receipt](tx kv.Tx, txHash []byte, receipt R) (R, error) {
	value, err := tx.GetOne(ReceiptBucket, txHash)
	if err != nil {
		return receipt, err
	}

	if len(value) == 0 {
		return receipt, ErrNoReceipts
	}

	if err := json.Unmarshal(value, &receipt); err != nil {
		return receipt, err
	}

	return receipt, nil
}
