package gosdk

import "github.com/ledgerwatch/erigon-lib/kv"

type StubRootCalculator struct{}

func NewStubRootCalculator() *StubRootCalculator {
	return &StubRootCalculator{}
}

func (*StubRootCalculator) StateRootCalculator(_ kv.RwTx) ([32]byte, error) {
	return [32]byte{}, nil
}
