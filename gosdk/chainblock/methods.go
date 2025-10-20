package chainblock

import "github.com/0xAtelerix/sdk/gosdk/apptypes"

// GetFieldsValues wraps the provided target in a ChainBlock and returns the field
// names and stringified values derived from the payload. When target is nil an
// error is returned.
func GetFieldsValues[T any](chainType apptypes.ChainType, target *T) (FieldsValues, error) {
	cb, err := NewChainBlock(chainType, target)
	if err != nil {
		return FieldsValues{}, err
	}

	return cb.ToFieldsAndValues(), nil
}
