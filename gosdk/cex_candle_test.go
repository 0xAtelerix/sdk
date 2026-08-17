package gosdk

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// NO_TEST_DOUBLE: pure codec data, no substituted owner.

const testCandleTimeframeMS = uint64(900_000)

func testCandleBars() []apptypes.CEXCandleBar {
	base := uint64(1_786_800_600_000)

	first := apptypes.CEXCandleBar{
		BarStartMS: base,
		BarCloseMS: base + testCandleTimeframeMS,
		Open:       "100.5", High: "101.25", Low: "99.9", Close: "101",
		Volume: "0",
	}
	// Gap on purpose: venues legitimately omit empty intervals.
	second := apptypes.CEXCandleBar{
		BarStartMS: base + 3*testCandleTimeframeMS,
		BarCloseMS: base + 4*testCandleTimeframeMS,
		Open:       "101", High: "102", Low: "100.75", Close: "101.5",
		Volume: "12.75",
	}

	return []apptypes.CEXCandleBar{first, second}
}

func TestEncodeDecodeCEXCandleBarsRoundTrip(t *testing.T) {
	t.Parallel()

	bars := testCandleBars()

	payload, digest, err := EncodeCEXCandleBars(bars, testCandleTimeframeMS)
	require.NoError(t, err)
	require.NotEmpty(t, payload)
	require.NotEqual(t, [32]byte{}, digest)

	decoded, err := DecodeCEXCandleBars(payload, testCandleTimeframeMS)
	require.NoError(t, err)
	require.Equal(t, bars, decoded)

	again, sameDigest, err := EncodeCEXCandleBars(decoded, testCandleTimeframeMS)
	require.NoError(t, err)
	require.Equal(t, payload, again)
	require.Equal(t, digest, sameDigest)
}

func TestEncodeCEXCandleBarsRejectsInvalidBatch(t *testing.T) {
	t.Parallel()

	bars := testCandleBars()
	bars[0].High = "1" // below open: OHLC sanity must fail

	_, _, err := EncodeCEXCandleBars(bars, testCandleTimeframeMS)
	require.ErrorIs(t, err, apptypes.ErrCEXCandleInvalid)
}

func TestDecodeCEXCandleBarsRejectsTrailingAndForeignTimeframe(t *testing.T) {
	t.Parallel()

	payload, _, err := EncodeCEXCandleBars(testCandleBars(), testCandleTimeframeMS)
	require.NoError(t, err)

	_, err = DecodeCEXCandleBars(append(payload, 0x00), testCandleTimeframeMS)
	require.Error(t, err)

	// The same bytes are off-grid under a different timeframe: identity is
	// (payload, timeframe), never payload alone.
	_, err = DecodeCEXCandleBars(payload, 3_600_000)
	require.ErrorIs(t, err, apptypes.ErrCEXCandleInvalid)
}

func TestDecodeCEXCandleBarsRejectsGarbage(t *testing.T) {
	t.Parallel()

	_, err := DecodeCEXCandleBars([]byte{0xff, 0x00, 0x13}, testCandleTimeframeMS)
	require.Error(t, err)
}
