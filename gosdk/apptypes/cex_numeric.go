package apptypes

import (
	"fmt"
	"math/big"
	"slices"

	sdkerrors "github.com/0xAtelerix/sdk/gosdk/library/errors"
)

const (
	// UInt256BEBytes is the fixed width for one unsigned 256-bit word.
	UInt256BEBytes = 32
	// CEXPackedNumericLevelBytes stores one fixed-width price/quantity pair.
	CEXPackedNumericLevelBytes = UInt256BEBytes * 2

	ErrUInt256BENegative             sdkerrors.SDKError = "uint256 big-endian value cannot be negative"
	ErrUInt256BEOverflow             sdkerrors.SDKError = "uint256 big-endian value exceeds 32 bytes"
	ErrCEXPackedNumericSideMalformed sdkerrors.SDKError = "malformed packed cex numeric side"
)

// UInt256BE is one fixed-width unsigned 256-bit big-endian value.
type UInt256BE [UInt256BEBytes]byte

// NewUInt256BE converts a non-negative bigint into a fixed-width 32-byte word.
func NewUInt256BE(value *big.Int) (UInt256BE, error) {
	if value == nil || value.Sign() == 0 {
		return UInt256BE{}, nil
	}

	if value.Sign() < 0 {
		return UInt256BE{}, ErrUInt256BENegative
	}

	raw := value.Bytes()
	if len(raw) > UInt256BEBytes {
		return UInt256BE{}, ErrUInt256BEOverflow
	}

	var out UInt256BE
	copy(out[UInt256BEBytes-len(raw):], raw)

	return out, nil
}

// BigInt returns the numeric value as a new bigint.
func (u UInt256BE) BigInt() *big.Int {
	return new(big.Int).SetBytes(u[:])
}

// IsZero reports whether the word is zero.
func (u UInt256BE) IsZero() bool {
	return u == UInt256BE{}
}

// String renders the canonical base-10 integer form.
func (u UInt256BE) String() string {
	return u.BigInt().String()
}

// CEXNumericPriceLevel is one canonical numeric price/quantity level.
type CEXNumericPriceLevel struct {
	Price    UInt256BE `cbor:"1,keyasint"`
	Quantity UInt256BE `cbor:"2,keyasint"`
}

// CEXNumericOrderBookSnapshot is the v2 canonical numeric CEX snapshot shape.
type CEXNumericOrderBookSnapshot struct {
	Exchange      string                 `cbor:"1,keyasint"`
	Symbol        string                 `cbor:"2,keyasint"`
	BaseAsset     string                 `cbor:"3,keyasint"`
	QuoteAsset    string                 `cbor:"4,keyasint"`
	BaseCEXScale  uint8                  `cbor:"5,keyasint"`
	QuoteCEXScale uint8                  `cbor:"6,keyasint"`
	LastUpdateID  uint64                 `cbor:"7,keyasint"`
	Bids          []CEXNumericPriceLevel `cbor:"8,keyasint"`
	Asks          []CEXNumericPriceLevel `cbor:"9,keyasint"`
	FetchedAt     int64                  `cbor:"10,keyasint"`
}

// CEXPackedNumericSide is one fixed-width packed snapshot side for storage.
type CEXPackedNumericSide struct {
	raw []byte
}

// NewCEXPackedNumericSide encodes canonical numeric levels into the storage shape.
func NewCEXPackedNumericSide(levels []CEXNumericPriceLevel) CEXPackedNumericSide {
	if len(levels) == 0 {
		return CEXPackedNumericSide{}
	}

	raw := make([]byte, len(levels)*CEXPackedNumericLevelBytes)
	for i, level := range levels {
		offset := i * CEXPackedNumericLevelBytes
		copy(raw[offset:offset+UInt256BEBytes], level.Price[:])
		copy(raw[offset+UInt256BEBytes:offset+CEXPackedNumericLevelBytes], level.Quantity[:])
	}

	return CEXPackedNumericSide{raw: raw}
}

// ParseCEXPackedNumericSide validates and stores a packed storage payload.
func ParseCEXPackedNumericSide(raw []byte) (CEXPackedNumericSide, error) {
	if len(raw)%CEXPackedNumericLevelBytes != 0 {
		return CEXPackedNumericSide{}, fmt.Errorf(
			"%w: bytes=%d",
			ErrCEXPackedNumericSideMalformed,
			len(raw),
		)
	}

	return CEXPackedNumericSide{raw: slices.Clone(raw)}, nil
}

// Bytes returns a defensive copy of the packed payload.
func (s CEXPackedNumericSide) Bytes() []byte {
	return slices.Clone(s.raw)
}

// LevelCount reports the number of packed levels.
func (s CEXPackedNumericSide) LevelCount() int {
	return len(s.raw) / CEXPackedNumericLevelBytes
}

// Levels decodes the packed payload into canonical numeric levels.
func (s CEXPackedNumericSide) Levels() ([]CEXNumericPriceLevel, error) {
	if len(s.raw)%CEXPackedNumericLevelBytes != 0 {
		return nil, fmt.Errorf(
			"%w: bytes=%d",
			ErrCEXPackedNumericSideMalformed,
			len(s.raw),
		)
	}

	if len(s.raw) == 0 {
		return nil, nil
	}

	levels := make([]CEXNumericPriceLevel, 0, s.LevelCount())
	for offset := 0; offset < len(s.raw); offset += CEXPackedNumericLevelBytes {
		var level CEXNumericPriceLevel
		copy(level.Price[:], s.raw[offset:offset+UInt256BEBytes])
		copy(
			level.Quantity[:],
			s.raw[offset+UInt256BEBytes:offset+CEXPackedNumericLevelBytes],
		)
		levels = append(levels, level)
	}

	return levels, nil
}
