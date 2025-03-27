package gosdk

import (
	"encoding/json"
	"fmt"
	"github.com/0xAtelerix/sdk/gosdk/types"
)

type EventStreamWrapper[appTx types.AppTransaction] struct {
	eventStream *EventReader
}

func (ews *EventStreamWrapper[appTx]) GetNewBatchesBlocking(limit int) ([]types.Batch[appTx], error) {
	batch, err := ews.eventStream.GetNewBatchesNonBlocking(limit)
	if err != nil {
		return nil, err
	}
	res := make([]types.Batch[appTx], len(batch))
	for i := range batch {
		res[i].Transactions = make([]appTx, len(batch[i].Events))
		for j, txBytes := range batch[i].Events {
			err := json.Unmarshal(txBytes, &res[i].Transactions[j])
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal batch %w", err)
			}
		}

	}
	return res, nil
}
