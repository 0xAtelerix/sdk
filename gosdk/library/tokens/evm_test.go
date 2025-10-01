package tokens

import (
	"math/big"
	"sort"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/library/tests"
)

func TestExtractErc20Transfer(t *testing.T) {
	t.Parallel()

	token := tests.Addr(0xAA)
	from := tests.Addr(0x01)
	to := tests.Addr(0x02)
	amt := big.NewInt(123456789)

	lg := gethtypes.Log{
		Address: token,
		Topics:  []common.Hash{SigTransfer, tests.AddrTopic(from), tests.AddrTopic(to)},
		Data:    tests.MustPack(t, erc20Data, amt),
		Index:   0,
	}
	rc := gethtypes.Receipt{
		TxHash: common.HexToHash("0x1"),
		Logs:   []*gethtypes.Log{&lg},
	}

	out := ExtractErcTransfers(&rc)
	require.Len(t, out, 1)

	x := out[0]
	require.Equal(t, ERC20, x.Balances.Standard)
	require.Equal(t, token.Hex(), x.Mint)
	require.Equal(t, from.Hex(), x.FromOwner)
	require.Equal(t, to.Hex(), x.ToOwner)
	require.Nil(t, x.Balances.TokenID)
	require.Empty(t, x.Balances.IDs)
	require.Empty(t, x.Balances.Values)
	require.Equal(t, 0, x.Amount.Cmp(amt))
	require.Equal(t, rc.TxHash, x.Balances.TxHash)
	require.Equal(t, lg.Index, x.Balances.LogIndex)
}

func TestExtractErc721Transfer(t *testing.T) {
	t.Parallel()

	token := tests.Addr(0xBB)
	from := tests.Addr(0x03)
	to := tests.Addr(0x04)
	tokenID := big.NewInt(42)

	lg := gethtypes.Log{
		Address: token,
		Topics: []common.Hash{
			SigTransfer,
			tests.AddrTopic(from),
			tests.AddrTopic(to),
			common.BigToHash(tokenID),
		},
		Data:  nil, // ERC-721 tokenId is indexed, no data
		Index: 1,
	}
	rc := gethtypes.Receipt{
		TxHash: common.HexToHash("0x2"),
		Logs:   []*gethtypes.Log{&lg},
	}

	out := ExtractErcTransfers(&rc)
	require.Len(t, out, 1)

	x := out[0]
	require.Equal(t, ERC721, x.Balances.Standard)
	require.Equal(t, token.Hex(), x.Mint)
	require.Equal(t, from.Hex(), x.FromOwner)
	require.Equal(t, to.Hex(), x.ToOwner)
	require.NotNil(t, x.Balances.TokenID)
	require.Equal(t, 0, x.Balances.TokenID.Cmp(tokenID))

	require.Equal(t, big.NewInt(1), x.Amount)
}

func TestExtractErc1155Single(t *testing.T) {
	t.Parallel()

	token := tests.Addr(0xCC)
	from := tests.Addr(0x05)
	to := tests.Addr(0x06)
	id := big.NewInt(7)
	val := big.NewInt(99)

	lg := gethtypes.Log{
		Address: token,
		Topics:  []common.Hash{SigTransferSingle, {}, tests.AddrTopic(from), tests.AddrTopic(to)},
		Data:    tests.MustPack(t, erc1155DataS, id, val),
		Index:   2,
	}
	rc := gethtypes.Receipt{
		TxHash: common.HexToHash("0x3"),
		Logs:   []*gethtypes.Log{&lg},
	}

	out := ExtractErcTransfers(&rc)
	require.Len(t, out, 1)

	x := out[0]
	require.Equal(t, ERC1155, x.Balances.Standard)
	require.Equal(t, token.Hex(), x.Mint)
	require.Equal(t, from.Hex(), x.FromOwner)
	require.Equal(t, to.Hex(), x.ToOwner)
	require.NotNil(t, x.Balances.TokenID)
	require.Equal(t, 0, x.Balances.TokenID.Cmp(id))
	require.Equal(t, 0, x.Amount.Cmp(val))
}

func TestExtractErc1155Batch(t *testing.T) {
	t.Parallel()

	token := tests.Addr(0xDD)
	from := tests.Addr(0x07)
	to := tests.Addr(0x08)
	ids := []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)}
	vals := []*big.Int{big.NewInt(10), big.NewInt(20), big.NewInt(30)}

	lg := gethtypes.Log{
		Address: token,
		Topics:  []common.Hash{SigTransferBatch, {}, tests.AddrTopic(from), tests.AddrTopic(to)},
		Data:    tests.MustPack(t, erc1155DataB, ids, vals),
		Index:   3,
	}
	rc := gethtypes.Receipt{
		TxHash: common.HexToHash("0x4"),
		Logs:   []*gethtypes.Log{&lg},
	}

	out := ExtractErcTransfers(&rc)
	require.Len(t, out, len(ids)) // one row per (id,value)

	x := out[0]
	require.Equal(t, ERC1155, x.Balances.Standard)
	require.Equal(t, token.Hex(), x.Mint)
	require.Equal(t, from.String(), x.FromOwner)
	require.Equal(t, to.String(), x.ToOwner)
	require.Len(t, x.Balances.IDs, len(ids))

	// sort by tokenId to make assertions stable
	sort.Slice(out, func(i, j int) bool {
		return out[i].Balances.TokenID.Cmp(out[j].Balances.TokenID) < 0
	})

	for i := range ids {
		x := out[i]
		require.Equal(t, ERC1155, x.Balances.Standard)
		require.Equal(t, token.Hex(), x.Mint)
		require.Equal(t, from.Hex(), x.FromOwner)
		require.Equal(t, to.Hex(), x.ToOwner)
		require.Equal(t, 0, x.Balances.TokenID.Cmp(ids[i]))
		require.Equal(t, 0, x.Amount.Cmp(vals[i]))
	}
}

func TestMixedLogsOneReceipt(t *testing.T) {
	t.Parallel()

	token20 := tests.Addr(0xA1)
	token721 := tests.Addr(0xA2)
	from := tests.Addr(0x10)
	to := tests.Addr(0x11)

	erc20 := gethtypes.Log{
		Address: token20,
		Topics:  []common.Hash{SigTransfer, tests.AddrTopic(from), tests.AddrTopic(to)},
		Data:    tests.MustPack(t, erc20Data, big.NewInt(5)),
		Index:   0,
	}
	erc721 := gethtypes.Log{
		Address: token721,
		Topics: []common.Hash{
			SigTransfer,
			tests.AddrTopic(from),
			tests.AddrTopic(to),
			common.BigToHash(big.NewInt(77)),
		},
		Index: 1,
	}

	rc := gethtypes.Receipt{
		TxHash: common.HexToHash("0x5"),
		Logs:   []*gethtypes.Log{&erc20, &erc721},
	}

	out := ExtractErcTransfers(&rc)
	require.Len(t, out, 2)

	// stable ordering by log index
	require.Equal(t, erc20.Index, out[0].Balances.LogIndex)
	require.Equal(t, erc721.Index, out[1].Balances.LogIndex)
}

func TestIgnoresNonTransferLogs(t *testing.T) {
	t.Parallel()

	// random signature
	otherSig := crypto.Keccak256Hash([]byte("SomethingElse(bytes32)"))

	lg := gethtypes.Log{
		Address: tests.Addr(0xEE),
		Topics:  []common.Hash{otherSig},
		Index:   7,
	}
	rc := gethtypes.Receipt{
		TxHash: common.HexToHash("0x6"),
		Logs:   []*gethtypes.Log{&lg},
	}

	out := ExtractErcTransfers(&rc)
	require.Empty(t, out)
}

func TestDefaultRegistry_ERC20(t *testing.T) {
	t.Parallel()

	reg := NewDefaultEvmTransferRegistry()

	token := tests.Addr(0xAA)
	from := tests.Addr(0x01)
	to := tests.Addr(0x02)
	amt := big.NewInt(123456789)

	lg := &gethtypes.Log{
		Address: token,
		Topics:  []common.Hash{SigTransfer, tests.AddrTopic(from), tests.AddrTopic(to)},
		Data:    tests.MustPack(t, erc20Data, amt),
		Index:   0,
	}
	tx := common.HexToHash("0x1")

	res, matched, err := reg.HandleLog(lg, tx)
	require.NoError(t, err)
	require.True(t, matched)
	require.Len(t, res, 1)

	evm, ok := res[0].(EvmTransfer)
	require.True(t, ok)
	require.Equal(t, ERC20, evm.Balances.Standard)
	require.Equal(t, token.Hex(), evm.Mint)
	require.Equal(t, from.Hex(), evm.FromOwner)
	require.Equal(t, to.Hex(), evm.ToOwner)
	require.Equal(t, 0, evm.Amount.Cmp(amt))
	require.Equal(t, tx, evm.Balances.TxHash)
	require.Equal(t, lg.Index, evm.Balances.LogIndex)
}

func TestDefaultRegistry_ERC721(t *testing.T) {
	t.Parallel()

	reg := NewDefaultEvmTransferRegistry()

	token := tests.Addr(0xBB)
	from := tests.Addr(0x03)
	to := tests.Addr(0x04)
	tokenID := big.NewInt(42)

	lg := &gethtypes.Log{
		Address: token,
		Topics: []common.Hash{
			SigTransfer,
			tests.AddrTopic(from),
			tests.AddrTopic(to),
			common.BigToHash(tokenID),
		},
		Data:  nil, // ERC-721 tokenId is indexed, no data
		Index: 1,
	}

	tx := common.HexToHash("0x2")

	res, matched, err := reg.HandleLog(lg, tx)
	require.NoError(t, err)
	require.True(t, matched)
	require.Len(t, res, 1)

	evm, ok := res[0].(EvmTransfer)
	require.True(t, ok)
	require.Equal(t, ERC721, evm.Balances.Standard)
	require.Equal(t, token.Hex(), evm.Mint)
	require.Equal(t, from.Hex(), evm.FromOwner)
	require.Equal(t, to.Hex(), evm.ToOwner)
	require.NotNil(t, evm.Balances.TokenID)
	require.Equal(t, 0, evm.Balances.TokenID.Cmp(tokenID))
	require.Equal(t, big.NewInt(1), evm.Amount)
}

func TestDefaultRegistry_ERC1155_Single(t *testing.T) {
	t.Parallel()

	reg := NewDefaultEvmTransferRegistry()

	token := tests.Addr(0xCC)
	from := tests.Addr(0x05)
	to := tests.Addr(0x06)
	id := big.NewInt(7)
	val := big.NewInt(99)

	lg := &gethtypes.Log{
		Address: token,
		Topics:  []common.Hash{SigTransferSingle, {}, tests.AddrTopic(from), tests.AddrTopic(to)},
		Data:    tests.MustPack(t, erc1155DataS, id, val),
		Index:   2,
	}
	tx := common.HexToHash("0x3")

	res, matched, err := reg.HandleLog(lg, tx)
	require.NoError(t, err)
	require.True(t, matched)
	require.Len(t, res, 1)

	evm, ok := res[0].(EvmTransfer)
	require.True(t, ok)
	require.Equal(t, ERC1155, evm.Balances.Standard)
	require.Equal(t, token.Hex(), evm.Mint)
	require.Equal(t, from.Hex(), evm.FromOwner)
	require.Equal(t, to.Hex(), evm.ToOwner)
	require.Equal(t, 0, evm.Balances.TokenID.Cmp(id))
	require.Equal(t, 0, evm.Amount.Cmp(val))
}

func TestDefaultRegistry_ERC1155_Batch(t *testing.T) {
	t.Parallel()

	reg := NewDefaultEvmTransferRegistry()

	token := tests.Addr(0xDD)
	from := tests.Addr(0x07)
	to := tests.Addr(0x08)
	ids := []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)}
	vals := []*big.Int{big.NewInt(10), big.NewInt(20), big.NewInt(30)}

	lg := &gethtypes.Log{
		Address: token,
		Topics:  []common.Hash{SigTransferBatch, {}, tests.AddrTopic(from), tests.AddrTopic(to)},
		Data:    tests.MustPack(t, erc1155DataB, ids, vals),
		Index:   3,
	}
	tx := common.HexToHash("0x4")

	res, matched, err := reg.HandleLog(lg, tx)
	require.NoError(t, err)
	require.True(t, matched)
	require.Len(t, res, len(ids))

	// sort by tokenId for stable checks
	sort.Slice(res, func(i, j int) bool {
		ai := res[i].(EvmTransfer).Balances.TokenID
		aj := res[j].(EvmTransfer).Balances.TokenID

		return ai.Cmp(aj) < 0
	})

	for i := range ids {
		evm, ok := res[i].(EvmTransfer)
		require.True(t, ok)
		require.Equal(t, ERC1155, evm.Balances.Standard)
		require.Equal(t, token.Hex(), evm.Mint)
		require.Equal(t, from.Hex(), evm.FromOwner)
		require.Equal(t, to.Hex(), evm.ToOwner)
		require.Equal(t, 0, evm.Balances.TokenID.Cmp(ids[i]))
		require.Equal(t, 0, evm.Amount.Cmp(vals[i]))
	}
}

func TestRegisterEvent_WETH_Deposit_Generic(t *testing.T) {
	t.Parallel()

	type wethDeposit struct {
		Dst common.Address `abi:"dst"`
		Wad *big.Int       `abi:"wad"`
	}

	abiJSON := `[
	  {"type":"event","name":"Deposit","inputs":[
	    {"indexed":true,"name":"dst","type":"address"},
	    {"indexed":false,"name":"wad","type":"uint256"}
	  ]}
	]`
	a, err := abi.JSON(strings.NewReader(abiJSON))
	require.NoError(t, err)

	// single registry whose result type is Event[wethDeposit]
	reg := NewRegistry[AppEvent]()
	_, err = RegisterEvent[wethDeposit](reg, a, "Deposit", "weth.deposit")
	require.NoError(t, err)

	eventFilter := Filter[wethDeposit]

	// build a matching log
	weth := tests.Addr(0xEE)
	dst := tests.Addr(0x33)
	wad := big.NewInt(777)
	sig := crypto.Keccak256Hash([]byte("Deposit(address,uint256)"))

	lg := &gethtypes.Log{
		Address: weth,
		Topics:  []common.Hash{sig, tests.AddrTopic(dst)},
		Data:    tests.MustPack(t, abi.Arguments{{Type: tests.MustType(t, "uint256")}}, wad),
		Index:   9,
	}
	tx := common.HexToHash("0xabc")

	out, matched, derr := reg.HandleLog(lg, tx)
	require.NoError(t, derr)
	require.True(t, matched)
	require.Len(t, out, 1)

	events := eventFilter(out)
	require.Len(t, events, 1)

	ev := events[0]

	require.Equal(t, "weth.deposit", ev.EventKind)
	require.Equal(t, weth.Hex(), ev.Contract)
	require.Equal(t, tx.Hex(), ev.TxHash)
	require.Equal(t, uint(9), ev.LogIndex)

	// payload (decoded event T)
	require.Equal(t, dst, ev.SubscribedEvent.Dst)
	require.Zero(t, ev.SubscribedEvent.Wad.Cmp(wad))
}

func TestRegisterEvent_UniV2_Sync_Generic(t *testing.T) {
	t.Parallel()

	// UniswapV2 Pair Sync(uint112 reserve0, uint112 reserve1)
	type UniV2Sync struct {
		Reserve0 *big.Int `abi:"reserve0"`
		Reserve1 *big.Int `abi:"reserve1"`
	}

	abiJSON := `[
	  {"type":"event","name":"Sync","inputs":[
	    {"indexed":false,"name":"reserve0","type":"uint112"},
	    {"indexed":false,"name":"reserve1","type":"uint112"}
	  ]}
	]`
	a, err := abi.JSON(strings.NewReader(abiJSON))
	require.NoError(t, err)

	reg := NewRegistry[AppEvent]()
	_, err = RegisterEvent[UniV2Sync](reg, a, "Sync", "univ2.sync")
	require.NoError(t, err)

	eventFilter := Filter[UniV2Sync]

	// Build a log
	pair := tests.Addr(0xAA)
	sig := crypto.Keccak256Hash([]byte("Sync(uint112,uint112)"))
	r0 := new(big.Int).SetUint64(12345)
	r1 := new(big.Int).SetUint64(67890)

	lg := &gethtypes.Log{
		Address: pair,
		Topics:  []common.Hash{sig},
		Data: tests.MustPack(t, abi.Arguments{
			{Type: tests.MustType(t, "uint112")},
			{Type: tests.MustType(t, "uint112")},
		}, r0, r1),
		Index: 2,
	}
	tx := common.HexToHash("0x555")

	out, matched, derr := reg.HandleLog(lg, tx)
	require.NoError(t, derr)
	require.True(t, matched)
	require.Len(t, out, 1)

	events := eventFilter(out)
	require.Len(t, events, 1)

	ev := events[0]

	require.Equal(t, "univ2.sync", ev.EventKind)
	require.Equal(t, pair.Hex(), ev.Contract)
	require.Equal(t, tx.Hex(), ev.TxHash)
	require.Equal(t, uint(2), ev.LogIndex)
	require.Zero(t, ev.SubscribedEvent.Reserve0.Cmp(r0))
	require.Zero(t, ev.SubscribedEvent.Reserve1.Cmp(r1))
}

//nolint:gochecknoglobals // there's no way to do the same with functions
var (
	uint256T, _  = abi.NewType("uint256", "", nil)
	uint256sT, _ = abi.NewType("uint256[]", "", nil)

	erc20Data    = abi.Arguments{{Type: uint256T}}
	erc1155DataS = abi.Arguments{{Type: uint256T}, {Type: uint256T}}
	erc1155DataB = abi.Arguments{{Type: uint256sT}, {Type: uint256sT}}
)

// UniswapV2 Pair Swap event:
// event Swap(address indexed sender, uint amount0In, uint amount1In, uint amount0Out, uint amount1Out, address indexed to);
type uniV2Swap struct {
	Sender     common.Address `abi:"sender"`
	Amount0In  *big.Int       `abi:"amount0In"`
	Amount1In  *big.Int       `abi:"amount1In"`
	Amount0Out *big.Int       `abi:"amount0Out"`
	Amount1Out *big.Int       `abi:"amount1Out"`
	To         common.Address `abi:"to"`
}

func TestCustomEvent_UniswapV2_Swap_DecodeDirect(t *testing.T) {
	t.Parallel()

	// Build a synthetic log
	pair := tests.Addr(0xAA)
	sender := tests.Addr(0x01)
	dst := tests.Addr(0x02)

	sig := crypto.Keccak256Hash([]byte("Swap(address,uint256,uint256,uint256,uint256,address)"))
	dataArgs := abi.Arguments{
		{Type: tests.MustType(t, "uint256")}, // amount0In
		{Type: tests.MustType(t, "uint256")}, // amount1In
		{Type: tests.MustType(t, "uint256")}, // amount0Out
		{Type: tests.MustType(t, "uint256")}, // amount1Out
	}

	data := tests.MustPack(
		t,
		dataArgs,
		big.NewInt(0),
		big.NewInt(1000),
		big.NewInt(500),
		big.NewInt(0),
	)

	lg := &gethtypes.Log{
		Address: pair,
		Topics:  []common.Hash{sig, tests.AddrTopic(sender), tests.AddrTopic(dst)},
		Data:    data,
		Index:   0,
	}

	// user's code
	const abiJSON = `[
	  {"type":"event","name":"Swap","inputs":[
	    {"indexed":true,"name":"sender","type":"address"},
	    {"indexed":false,"name":"amount0In","type":"uint256"},
	    {"indexed":false,"name":"amount1In","type":"uint256"},
	    {"indexed":false,"name":"amount0Out","type":"uint256"},
	    {"indexed":false,"name":"amount1Out","type":"uint256"},
	    {"indexed":true,"name":"to","type":"address"}
	  ]}
	]`

	// todo: Memoization
	a, err := abi.JSON(strings.NewReader(abiJSON))
	require.NoError(t, err)

	ev, matched, err := DecodeEventInto[uniV2Swap](a, "Swap", lg)
	require.NoError(t, err)
	require.True(t, matched)
	require.Equal(t, sender, ev.Sender)
	require.Equal(t, dst, ev.To)
	require.Zero(t, ev.Amount0In.Cmp(big.NewInt(0)))
	require.Zero(t, ev.Amount1In.Cmp(big.NewInt(1000)))
	require.Zero(t, ev.Amount0Out.Cmp(big.NewInt(500)))
	require.Zero(t, ev.Amount1Out.Cmp(big.NewInt(0)))
}

type SwapResult struct {
	Pair       string
	Sender     string
	To         string
	Amount0In  *big.Int
	Amount1In  *big.Int
	Amount0Out *big.Int
	Amount1Out *big.Int
}

func TestCustomEvent_UniswapV2_Swap_Registry(t *testing.T) {
	t.Parallel()

	const abiJSON = `[
	  {"type":"event","name":"Swap","inputs":[
	    {"indexed":true,"name":"sender","type":"address"},
	    {"indexed":false,"name":"amount0In","type":"uint256"},
	    {"indexed":false,"name":"amount1In","type":"uint256"},
	    {"indexed":false,"name":"amount0Out","type":"uint256"},
	    {"indexed":false,"name":"amount1Out","type":"uint256"},
	    {"indexed":true,"name":"to","type":"address"}
	  ]}
	]`

	a, err := abi.JSON(strings.NewReader(abiJSON))
	require.NoError(t, err)

	// Build registry producing SwapResult
	reg := NewRegistry[SwapResult]()
	sig := crypto.Keccak256Hash([]byte("Swap(address,uint256,uint256,uint256,uint256,address)"))

	// Register decoder + mapper
	Register[uniV2Swap, SwapResult](reg, sig, a, "Swap",
		func(ev uniV2Swap, meta Meta) ([]SwapResult, error) {
			return []SwapResult{{
				Pair:       meta.Contract.Hex(),
				Sender:     ev.Sender.Hex(),
				To:         ev.To.Hex(),
				Amount0In:  new(big.Int).Set(ev.Amount0In),
				Amount1In:  new(big.Int).Set(ev.Amount1In),
				Amount0Out: new(big.Int).Set(ev.Amount0Out),
				Amount1Out: new(big.Int).Set(ev.Amount1Out),
			}}, nil
		},
	)

	// Build a log
	pair := tests.Addr(0xAB)
	sender := tests.Addr(0x0A)
	dst := tests.Addr(0x0B)
	dataArgs := abi.Arguments{
		{Type: tests.MustType(t, "uint256")}, // amount0In
		{Type: tests.MustType(t, "uint256")}, // amount1In
		{Type: tests.MustType(t, "uint256")}, // amount0Out
		{Type: tests.MustType(t, "uint256")}, // amount1Out
	}
	data := tests.MustPack(t, dataArgs, big.NewInt(10), big.NewInt(0), big.NewInt(0), big.NewInt(9))

	lg := &gethtypes.Log{
		Address: pair,
		Topics:  []common.Hash{sig, tests.AddrTopic(sender), tests.AddrTopic(dst)},
		Data:    data,
		Index:   5,
	}
	rc := &gethtypes.Receipt{TxHash: common.HexToHash("0x123")}

	out, matched, err := reg.HandleLog(lg, rc.TxHash)
	require.NoError(t, err)
	require.True(t, matched)
	require.Len(t, out, 1)

	got := out[0]
	require.Equal(t, pair.Hex(), got.Pair)
	require.Equal(t, sender.Hex(), got.Sender)
	require.Equal(t, dst.Hex(), got.To)
	require.Zero(t, got.Amount0In.Cmp(big.NewInt(10)))
	require.Zero(t, got.Amount1Out.Cmp(big.NewInt(9)))
}

// ERC-20 Approval(owner, spender, value)
type erc20Approval struct {
	Owner   common.Address `abi:"owner"`
	Spender common.Address `abi:"spender"`
	Value   *big.Int       `abi:"value"`
}

type ApprovalResult struct {
	Token   string
	Owner   string
	Spender string
	Value   *big.Int
}

// WETH Deposit(dst, wad)
type wethDeposit struct {
	Dst common.Address `abi:"dst"`
	Wad *big.Int       `abi:"wad"`
}

type DepositResult struct {
	Token string
	Dst   string
	Wad   *big.Int
}

func TestCustomEvents_Approval_And_WETH_Deposit(t *testing.T) {
	t.Parallel()

	// ABIs
	erc20ABI, err := abi.JSON(strings.NewReader(`[
	  {"type":"event","name":"Approval","inputs":[
	    {"indexed":true,"name":"owner","type":"address"},
	    {"indexed":true,"name":"spender","type":"address"},
	    {"indexed":false,"name":"value","type":"uint256"}
	  ]}
	]`))
	if err != nil {
		t.Fatal(err)
	}

	wethABI, err := abi.JSON(strings.NewReader(`[
	  {"type":"event","name":"Deposit","inputs":[
	    {"indexed":true,"name":"dst","type":"address"},
	    {"indexed":false,"name":"wad","type":"uint256"}
	  ]}
	]`))
	if err != nil {
		t.Fatal(err)
	}

	// Registry that can emit both result types? Keep it simple: separate registries.
	approvals := NewRegistry[ApprovalResult]()
	deposits := NewRegistry[DepositResult]()

	sigApproval := crypto.Keccak256Hash([]byte("Approval(address,address,uint256)"))
	sigDeposit := crypto.Keccak256Hash([]byte("Deposit(address,uint256)"))

	Register[erc20Approval, ApprovalResult](approvals, sigApproval, erc20ABI, "Approval",
		func(ev erc20Approval, m Meta) ([]ApprovalResult, error) {
			return []ApprovalResult{{
				Token:   m.Contract.Hex(),
				Owner:   ev.Owner.Hex(),
				Spender: ev.Spender.Hex(),
				Value:   new(big.Int).Set(ev.Value),
			}}, nil
		},
	)

	Register[wethDeposit, DepositResult](deposits, sigDeposit, wethABI, "Deposit",
		func(ev wethDeposit, m Meta) ([]DepositResult, error) {
			return []DepositResult{{
				Token: m.Contract.Hex(),
				Dst:   ev.Dst.Hex(),
				Wad:   new(big.Int).Set(ev.Wad),
			}}, nil
		},
	)

	// Build logs
	token := tests.Addr(0xFE)
	owner := tests.Addr(0x11)
	spender := tests.Addr(0x22)
	amount := big.NewInt(1234)

	lgApproval := &gethtypes.Log{
		Address: token,
		Topics:  []common.Hash{sigApproval, tests.AddrTopic(owner), tests.AddrTopic(spender)},
		Data:    tests.MustPack(t, abi.Arguments{{Type: tests.MustType(t, "uint256")}}, amount),
		Index:   0,
	}
	rc := &gethtypes.Receipt{TxHash: common.HexToHash("0xabc")}

	aout, amatched, aerr := approvals.HandleLog(lgApproval, rc.TxHash)
	if aerr != nil || !amatched || len(aout) != 1 {
		t.Fatalf("approval handle failed matched=%v err=%v out=%d", amatched, aerr, len(aout))
	}

	if aout[0].Owner != owner.Hex() || aout[0].Spender != spender.Hex() {
		t.Fatal("approval owner/spender mismatch")
	}

	if aout[0].Value.Cmp(amount) != 0 {
		t.Fatal("approval value mismatch")
	}

	// WETH Deposit
	weth := tests.Addr(0xEE)
	dst := tests.Addr(0x33)
	wad := big.NewInt(777)

	lgDeposit := &gethtypes.Log{
		Address: weth,
		Topics:  []common.Hash{sigDeposit, tests.AddrTopic(dst)},
		Data:    tests.MustPack(t, abi.Arguments{{Type: tests.MustType(t, "uint256")}}, wad),
		Index:   1,
	}

	dout, dmatched, derr := deposits.HandleLog(lgDeposit, rc.TxHash)
	if derr != nil || !dmatched || len(dout) != 1 {
		t.Fatalf("deposit handle failed matched=%v err=%v out=%d", dmatched, derr, len(dout))
	}

	if dout[0].Token != weth.Hex() || dout[0].Dst != dst.Hex() {
		t.Fatal("deposit token/dst mismatch")
	}

	if dout[0].Wad.Cmp(wad) != 0 {
		t.Fatal("deposit wad mismatch")
	}
}
