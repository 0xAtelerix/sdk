package tokens

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func addr(i byte) common.Address {
	var a common.Address

	a[19] = i

	return a
}

func addrTopic(a common.Address) common.Hash {
	var h common.Hash
	copy(h[12:], a[:]) // ABI-encoded indexed address (left-padded to 32)

	return h
}

func mustPack(t *testing.T, args abi.Arguments, vs ...any) []byte {
	t.Helper()

	b, err := args.Pack(vs...)
	require.NoError(t, err)

	return b
}

func TestExtractErc20Transfer(t *testing.T) {
	token := addr(0xAA)
	from := addr(0x01)
	to := addr(0x02)
	amt := big.NewInt(123456789)

	lg := gethtypes.Log{
		Address: token,
		Topics:  []common.Hash{sigERC20ERC721Transfer, addrTopic(from), addrTopic(to)},
		Data:    mustPack(t, erc20Data, amt),
		Index:   0,
	}
	rc := gethtypes.Receipt{
		TxHash: common.HexToHash("0x1"),
		Logs:   []*gethtypes.Log{&lg},
	}

	out := ExtractErcTransfers(rc)
	require.Len(t, out, 1)

	x := out[0]
	require.Equal(t, ERC20, x.Standard)
	require.Equal(t, token, x.Token)
	require.Equal(t, from, x.From)
	require.Equal(t, to, x.To)
	require.Zero(t, x.TokenID)
	require.Equal(t, amt, x.Amount)
	require.Equal(t, rc.TxHash, x.TxHash)
	require.Equal(t, lg.Index, x.LogIndex)
}

func TestExtractErc721Transfer(t *testing.T) {
	token := addr(0xBB)
	from := addr(0x03)
	to := addr(0x04)
	tokenID := big.NewInt(42)

	lg := gethtypes.Log{
		Address: token,
		Topics: []common.Hash{
			sigERC20ERC721Transfer,
			addrTopic(from),
			addrTopic(to),
			common.BigToHash(tokenID),
		},
		Data:  nil, // ERC-721 tokenId is indexed, no data
		Index: 1,
	}
	rc := gethtypes.Receipt{
		TxHash: common.HexToHash("0x2"),
		Logs:   []*gethtypes.Log{&lg},
	}

	out := ExtractErcTransfers(rc)
	require.Len(t, out, 1)

	x := out[0]
	require.Equal(t, ERC721, x.Standard)
	require.Equal(t, token, x.Token)
	require.Equal(t, from, x.From)
	require.Equal(t, to, x.To)
	require.NotNil(t, x.TokenID)

	var nilBigInt *big.Int
	require.Equal(t, nilBigInt, x.Amount) // usually nil; extractor may leave zero
	require.Equal(t, tokenID, x.TokenID)
}

func TestExtractErc1155Single(t *testing.T) {
	token := addr(0xCC)
	from := addr(0x05)
	to := addr(0x06)
	id := big.NewInt(7)
	val := big.NewInt(99)

	lg := gethtypes.Log{
		Address: token,
		Topics:  []common.Hash{sigERC1155Single, {}, addrTopic(from), addrTopic(to)},
		Data:    mustPack(t, erc1155DataS, id, val),
		Index:   2,
	}
	rc := gethtypes.Receipt{
		TxHash: common.HexToHash("0x3"),
		Logs:   []*gethtypes.Log{&lg},
	}

	out := ExtractErcTransfers(rc)
	require.Len(t, out, 1)

	x := out[0]
	require.Equal(t, ERC1155, x.Standard)
	require.Equal(t, token, x.Token)
	require.Equal(t, from, x.From)
	require.Equal(t, to, x.To)
	require.Equal(t, id, x.TokenID)
	require.Equal(t, val, x.Amount)
}

func TestExtractErc1155Batch(t *testing.T) {
	token := addr(0xDD)
	from := addr(0x07)
	to := addr(0x08)
	ids := []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)}
	vals := []*big.Int{big.NewInt(10), big.NewInt(20), big.NewInt(30)}

	lg := gethtypes.Log{
		Address: token,
		Topics:  []common.Hash{sigERC1155Batch, {}, addrTopic(from), addrTopic(to)},
		Data:    mustPack(t, erc1155DataB, ids, vals),
		Index:   3,
	}
	rc := gethtypes.Receipt{
		TxHash: common.HexToHash("0x4"),
		Logs:   []*gethtypes.Log{&lg},
	}

	out := ExtractErcTransfers(rc)
	require.Len(t, out, 1)

	x := out[0]
	require.Equal(t, ERC1155, x.Standard)
	require.Equal(t, token, x.Token)
	require.Equal(t, from, x.From)
	require.Equal(t, to, x.To)
	require.Len(t, x.IDs, len(ids))
	require.Len(t, x.Values, len(vals))

	for i := range ids {
		require.Equal(t, 0, x.IDs[i].Cmp(ids[i]))
		require.Equal(t, 0, x.Values[i].Cmp(vals[i]))
	}
}

func TestMixedLogsOneReceipt(t *testing.T) {
	token20 := addr(0xA1)
	token721 := addr(0xA2)
	from := addr(0x10)
	to := addr(0x11)

	erc20 := gethtypes.Log{
		Address: token20,
		Topics:  []common.Hash{sigERC20ERC721Transfer, addrTopic(from), addrTopic(to)},
		Data:    mustPack(t, erc20Data, big.NewInt(5)),
		Index:   0,
	}
	erc721 := gethtypes.Log{
		Address: token721,
		Topics: []common.Hash{
			sigERC20ERC721Transfer,
			addrTopic(from),
			addrTopic(to),
			common.BigToHash(big.NewInt(77)),
		},
		Index: 1,
	}

	rc := gethtypes.Receipt{
		TxHash: common.HexToHash("0x5"),
		Logs:   []*gethtypes.Log{&erc20, &erc721},
	}

	out := ExtractErcTransfers(rc)
	require.Len(t, out, 2)

	// stable ordering by log index
	require.Equal(t, erc20.Index, out[0].LogIndex)
	require.Equal(t, erc721.Index, out[1].LogIndex)
}

func TestIgnoresNonTransferLogs(t *testing.T) {
	// random signature
	otherSig := crypto.Keccak256Hash([]byte("SomethingElse(bytes32)"))

	lg := gethtypes.Log{
		Address: addr(0xEE),
		Topics:  []common.Hash{otherSig},
		Index:   7,
	}
	rc := gethtypes.Receipt{
		TxHash: common.HexToHash("0x6"),
		Logs:   []*gethtypes.Log{&lg},
	}

	out := ExtractErcTransfers(rc)
	require.Empty(t, out)
}
