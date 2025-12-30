package fields

import "encoding/json"

type CustomFields interface {
	json.RawMessage | EthereumCustomFields
}
type EVMCustomKind uint8

const (
	CustomRawJSON EVMCustomKind = iota
	CustomEthereumFields
)
