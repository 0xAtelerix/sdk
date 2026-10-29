package cexidentity

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

var errFrozenTableChanged = errors.New(
	"cex_order_book_identity.json is the frozen v9 table and must not change; " +
		"new markets get ids from apptypes.SymbolID",
)

// ValidateFrozen fails unless contents are the frozen v9 table byte for byte.
func ValidateFrozen(contents []byte) error {
	sum := sha256.Sum256(contents)

	got := hex.EncodeToString(sum[:])
	if got != apptypes.CEXOrderBookIdentityV9SHA256 {
		return fmt.Errorf("%w: sha256 %s, want %s",
			errFrozenTableChanged, got, apptypes.CEXOrderBookIdentityV9SHA256)
	}

	return nil
}
