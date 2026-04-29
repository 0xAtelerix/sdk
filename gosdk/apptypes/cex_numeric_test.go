package apptypes

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewUInt256BERoundTrip(t *testing.T) {
	t.Parallel()

	maxValue := new(big.Int).Sub(
		new(big.Int).Lsh(big.NewInt(1), 256),
		big.NewInt(1),
	)

	word, err := NewUInt256BE(maxValue)
	require.NoError(t, err)
	require.Equal(t, maxValue.String(), word.String())
	require.Equal(t, 0, maxValue.Cmp(word.BigInt()))
	require.False(t, word.IsZero())
}

func TestNewUInt256BERejectsNegative(t *testing.T) {
	t.Parallel()

	_, err := NewUInt256BE(big.NewInt(-1))
	require.ErrorIs(t, err, ErrUInt256BENegative)
}

func TestNewUInt256BERejectsOverflow(t *testing.T) {
	t.Parallel()

	overflow := new(big.Int).Lsh(big.NewInt(1), 256)

	_, err := NewUInt256BE(overflow)
	require.ErrorIs(t, err, ErrUInt256BEOverflow)
}

func TestCEXPackedNumericSideRoundTrip(t *testing.T) {
	t.Parallel()

	levels := []CEXNumericPriceLevel{
		{
			Price:    requireUInt256BE(t, big.NewInt(123456789)),
			Quantity: requireUInt256BE(t, big.NewInt(987654321)),
		},
		{
			Price:    requireUInt256BE(t, new(big.Int).Lsh(big.NewInt(1), 200)),
			Quantity: requireUInt256BE(t, new(big.Int).Lsh(big.NewInt(1), 128)),
		},
	}

	packed := NewCEXPackedNumericSide(levels)
	require.Len(t, packed.Bytes(), len(levels)*CEXPackedNumericLevelBytes)
	require.Equal(t, len(levels), packed.LevelCount())

	decoded, err := packed.Levels()
	require.NoError(t, err)
	require.Equal(t, levels, decoded)

	reparsed, err := ParseCEXPackedNumericSide(packed.Bytes())
	require.NoError(t, err)

	decodedReparsed, err := reparsed.Levels()
	require.NoError(t, err)
	require.Equal(t, levels, decodedReparsed)
}

func TestParseCEXPackedNumericSideRejectsMalformedLength(t *testing.T) {
	t.Parallel()

	_, err := ParseCEXPackedNumericSide(make([]byte, CEXPackedNumericLevelBytes-1))
	require.ErrorIs(t, err, ErrCEXPackedNumericSideMalformed)
}

func FuzzCEXPackedNumericSideRoundTrip(f *testing.F) {
	f.Add(uint64(1), uint64(2), uint64(3), uint64(4))

	f.Fuzz(func(
		t *testing.T,
		price1 uint64,
		qty1 uint64,
		price2 uint64,
		qty2 uint64,
	) {
		t.Parallel()

		levels := []CEXNumericPriceLevel{
			{
				Price:    requireUInt256BE(t, new(big.Int).SetUint64(price1)),
				Quantity: requireUInt256BE(t, new(big.Int).SetUint64(qty1)),
			},
			{
				Price:    requireUInt256BE(t, new(big.Int).SetUint64(price2)),
				Quantity: requireUInt256BE(t, new(big.Int).SetUint64(qty2)),
			},
		}

		packed := NewCEXPackedNumericSide(levels)
		decoded, err := packed.Levels()
		require.NoError(t, err)
		require.Equal(t, levels, decoded)

		reparsed, err := ParseCEXPackedNumericSide(packed.Bytes())
		require.NoError(t, err)
		require.Equal(t, packed.Bytes(), reparsed.Bytes())
	})
}

func requireUInt256BE(t *testing.T, value *big.Int) UInt256BE {
	t.Helper()

	word, err := NewUInt256BE(value)
	require.NoError(t, err)

	return word
}

func FuzzParseCEXPackedNumericSide(f *testing.F) {
	f.Add([]byte{0x01, 0x02, 0x03})
	f.Add(make([]byte, CEXPackedNumericLevelBytes))

	f.Fuzz(func(t *testing.T, raw []byte) {
		t.Parallel()

		packed, err := ParseCEXPackedNumericSide(raw)
		if len(raw)%CEXPackedNumericLevelBytes != 0 {
			require.ErrorIs(t, err, ErrCEXPackedNumericSideMalformed)

			return
		}

		require.NoError(t, err)

		levels, decodeErr := packed.Levels()
		require.NoError(t, decodeErr)
		require.Len(t, levels, len(raw)/CEXPackedNumericLevelBytes)
	})
}
