package apptypes

import (
	"bytes"
	"context"
	"fmt"

	"github.com/blocto/solana-go-sdk/client"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"

	sdkerrors "github.com/0xAtelerix/sdk/gosdk/library/errors"
)

type AppTransaction[R Receipt] interface {
	Hash() [32]byte
	Process(dbTx kv.RwTx) (R, []ExternalTransaction, error)
}

type Receipt interface {
	TxHash() [32]byte
	Status() TxReceiptStatus
	Error() string
}

// Batch is the serializable appchain batch payload.
//
// It contains app transactions, external block references, checkpoints, and
// CEX refs. Cross-appchain transactions are still intentionally left as a TODO
// below, matching the original contract note.
type Batch[appTx AppTransaction[R], R Receipt] struct {
	Atropos        [32]byte         `cbor:"1,keyasint"`
	Transactions   []appTx          `cbor:"2,keyasint"`
	ExternalBlocks []*ExternalBlock `cbor:"3,keyasint"`
	Checkpoints    []*Checkpoint    `cbor:"4,keyasint"`
	// todo add crossappchain tx
	// ExternalTransactions [][]byte
	EndOffset        int64             `cbor:"5,keyasint"`
	TxEndOffset      int64             `cbor:"6,keyasint"` // txReader.position после чтения
	CEXOrderBookRefs []CEXOrderBookRef `cbor:"7,keyasint"`
	// HyperliquidAllMidsRefs carries typed public-data producer samples.
	// These are not CEX order books and must not be routed through
	// CEXOrderBookRefs.
	HyperliquidAllMidsRefs []HyperliquidAllMidsRef `cbor:"8,keyasint"`
	// CEXMarketTradeBatchRefs carries immutable compact trade-batch references.
	// It is independent from order-book and Hyperliquid allMids references.
	CEXMarketTradeBatchRefs []CEXMarketTradeBatchRef `cbor:"9,keyasint"`
}

type ExternalEntity interface {
	GetEntityID() ExternalID
}

type ExternalFullBlock interface {
	gethtypes.Block | client.Block | *gethtypes.Block | *client.Block
}

type ExternalData interface {
	ExternalFullBlock | ExternalReceipt
}

type ExternalReceipt interface {
	client.BlockTransaction | gethtypes.Receipt
}

type ExternalBlock struct {
	ChainID     uint64   `cbor:"1,keyasint"`
	BlockNumber uint64   `cbor:"2,keyasint"`
	BlockHash   [32]byte `cbor:"3,keyasint"`
}

func MakeExternalBlock(chainID uint64, blockNumber uint64, blockHash [32]byte) ExternalBlock {
	return ExternalBlock{
		ChainID:     chainID,
		BlockNumber: blockNumber,
		BlockHash:   blockHash,
	}
}

func (e ExternalBlock) GetEntityID() ExternalID {
	return ExternalID(e)
}

type ExternalID struct {
	ChainID     uint64   `cbor:"1,keyasint"`
	BlockNumber uint64   `cbor:"2,keyasint"`
	BlockHash   [32]byte `cbor:"3,keyasint"`
}

// AppchainBlock is the minimal appchain block interface used by the SDK.
//
// For DAG-style appchains, this can represent the interval between two
// atroposes, not only a linear block.
type AppchainBlock interface {
	Hash() [32]byte
	StateRoot() [32]byte
}

type StoredAppchainBlock[appBlock AppchainBlock] struct {
	Root  [32]byte `cbor:"1,keyasint"`
	Block appBlock `cbor:"2,keyasint"`
}

type AppchainBlockConstructor[appTx AppTransaction[R], R Receipt, block AppchainBlock] func(
	blockNumber uint64,
	stateRoot [32]byte,
	previousBlockHash [32]byte,
	txsBatch Batch[appTx, R]) block

// ExternalTransaction stores an external chain transaction payload.
//
// For connected L1/L2 chains, callers must be able to unmarshal the Tx field
// into the native transaction type. For inter-appchain payloads, Tx is inserted
// as-is.
type ExternalTransaction struct {
	ChainID ChainType `cbor:"1,keyasint"`
	Tx      []byte    `cbor:"2,keyasint"`
}

// RootCalculator calculates the state root from an MDBX transaction.
//
// State-root ownership is intentionally abstract here: the concrete appchain
// may provide a replaceable implementation. The open design options preserved
// from the original note are:
// 1. one state table with prefixes for different modules;
// 2. multiple state tables plus an explicit list of which tables participate in
// the state root.
type RootCalculator interface {
	StateRootCalculator(tx kv.RwTx) ([32]byte, error)
}

// AppchainTxPoolBatch identifies the hash of the transaction-pool batch to process.
type AppchainTxPoolBatch struct {
	ChainID uint64   `cbor:"1,keyasint"`
	Hash    [32]byte `cbor:"2,keyasint"`
}

type DB interface {
	Write() // some changes
	Read()  // some read actions
	Commit(checkpoint Checkpoint) error
}

// TxPoolInterface определяет методы для работы с пулом транзакций
type TxPoolInterface[T AppTransaction[R], R Receipt] interface {
	// AddTransaction добавляет транзакцию в пул
	AddTransaction(ctx context.Context, tx T) error

	// GetTransaction получает транзакцию по хэшу
	GetTransaction(ctx context.Context, hash []byte) (T, error)

	// RemoveTransaction удаляет транзакцию из пула
	RemoveTransaction(ctx context.Context, hash []byte) error

	// GetPendingTransactions возвращает все транзакции
	GetPendingTransactions(ctx context.Context) ([]T, error)

	CreateTransactionBatch(ctx context.Context) ([]byte, [][]byte, error)

	GetTransactionStatus(ctx context.Context, hash []byte) (TxStatus, error)

	// Close закрывает хранилище транзакций
	Close() error
}

// Checkpoint captures finalization of an appchain state transition.
type Checkpoint struct {
	ChainID                  uint64   `json:"chainId"                  cbor:"1,keyasint"`
	BlockNumber              uint64   `json:"blockNumber"              cbor:"2,keyasint"`
	BlockHash                [32]byte `json:"blockHash"                cbor:"3,keyasint"`
	StateRoot                [32]byte `json:"stateRoot"                cbor:"4,keyasint"`
	ExternalTransactionsRoot [32]byte `json:"externalTransactionsRoot" cbor:"5,keyasint"`
}

func MakeCheckpoint(chainID uint64, blockNumber uint64, blockHash [32]byte) Checkpoint {
	return Checkpoint{
		ChainID:     chainID,
		BlockNumber: blockNumber,
		BlockHash:   blockHash,
	}
}

func (c Checkpoint) GetEntityID() ExternalID {
	return ExternalID{
		ChainID:     c.ChainID,
		BlockNumber: c.BlockNumber,
		BlockHash:   c.BlockHash,
	}
}

type Event struct {
	// todo возможно тут должно быть MedianTime
	Base          BaseEvent `json:"base"          cbor:"1,keyasint"`
	CreationTime  uint64    `json:"creationTime"  cbor:"2,keyasint"`
	PrevEpochHash *[32]byte `json:"prevEpochHash" cbor:"3,keyasint"`

	// Transaction batches already sent to other validators, with signatures
	// proving that those validators received them.
	TxPool []AppchainTxPoolBatch `json:"txPool" cbor:"4,keyasint"`

	// Appchain state updates: new state root, block, and external transactions.
	Appchains []Checkpoint `json:"appchains" cbor:"5,keyasint"`

	// External blocks.
	BlockVotes []ExternalBlock `json:"blockVotes" cbor:"6,keyasint"`

	Signature [64]byte `json:"signature" cbor:"7,keyasint"`

	CEXOrderBookRefs []CEXOrderBookRef `json:"cexOrderBookRefs" cbor:"8,keyasint"`

	HyperliquidAllMidsRefs []HyperliquidAllMidsRef `json:"hyperliquidAllMidsRefs" cbor:"9,keyasint"`

	CEXMarketTradeBatchRefs []CEXMarketTradeBatchRef `json:"cexMarketTradeBatchRefs" cbor:"10,keyasint"`
}

func (e Event) Bytes() ([]byte, error) {
	var buf bytes.Buffer

	enc := cbor.NewEncoder(&buf)

	if err := enc.Encode(e); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type BaseEvent struct {
	ID      [32]byte   `cbor:"1,keyasint"`
	Epoch   uint32     `cbor:"2,keyasint"`
	Seq     uint32     `cbor:"3,keyasint"`
	Frame   uint32     `cbor:"4,keyasint"`
	Creator uint32     `cbor:"5,keyasint"`
	Lamport uint32     `cbor:"6,keyasint"`
	Parents [][32]byte `cbor:"7,keyasint"`
}

type AppchainAddresses struct {
	ChainID        uint32 `cbor:"1,keyasint"`
	EmitterAddress string `cbor:"2,keyasint"`
}

type CEXPriceLevel struct {
	//nolint:revive // fxamacker/cbor requires this marker to encode the public struct as a compact array.
	_        struct{} `cbor:",toarray"`
	Price    string
	Quantity string
}

type CEXOrderBookSnapshot struct {
	Exchange     string          `cbor:"1,keyasint"`
	Symbol       string          `cbor:"2,keyasint"`
	LastUpdateID int64           `cbor:"3,keyasint"`
	Bids         []CEXPriceLevel `cbor:"4,keyasint"`
	Asks         []CEXPriceLevel `cbor:"5,keyasint"`
	FetchedAt    int64           `cbor:"6,keyasint"`
	ExchangeID   CEXExchangeID   `cbor:"7,keyasint,omitempty"`
	MarketTypeID CEXMarketTypeID `cbor:"8,keyasint,omitempty"`
	SymbolID     CEXSymbolID     `cbor:"9,keyasint,omitempty"`
}

// CEXExchangeID is the numeric exchange dimension id used by fresh CEX
// order-book refs.
type CEXExchangeID uint16

// CEXMarketTypeID is the numeric market-type dimension id used by fresh CEX
// order-book refs.
type CEXMarketTypeID uint8

// CEXSymbolID is the numeric symbol dimension id used by fresh CEX order-book
// refs.
type CEXSymbolID uint32

// CEXOrderBookRef is a lightweight reference to a CEX order book snapshot.
type CEXOrderBookRef struct {
	Exchange  string `cbor:"1,keyasint,omitempty"`
	Symbol    string `cbor:"2,keyasint,omitempty"`
	FetchedAt int64  `cbor:"3,keyasint"`

	ExchangeID   CEXExchangeID   `cbor:"4,keyasint,omitempty"`
	MarketTypeID CEXMarketTypeID `cbor:"5,keyasint,omitempty"`
	SymbolID     CEXSymbolID     `cbor:"6,keyasint,omitempty"`
}

// CEXMarketTradeSide is the exchange-reported aggressor side. It is a typed
// value so stored batches cannot carry arbitrary direction strings.
type CEXMarketTradeSide string

const (
	CEXMarketTradeSideBuy  CEXMarketTradeSide = "buy"
	CEXMarketTradeSideSell CEXMarketTradeSide = "sell"
)

// CEXMarketTrade is the compact per-trade payload stored inside an immutable
// batch. Market identity belongs to the enclosing batch reference, not each
// trade, so the encoded payload contains no repeated market fields.
type CEXMarketTrade struct {
	Price        string             `cbor:"1,keyasint"`
	Size         string             `cbor:"2,keyasint"`
	Side         CEXMarketTradeSide `cbor:"3,keyasint"`
	SourceTimeMS uint64             `cbor:"4,keyasint"`
	TradeID      [16]byte           `cbor:"5,keyasint"`
}

const ErrCEXMarketTradeInvalid sdkerrors.SDKError = "invalid cex market trade"

// ValidateCEXMarketTrades validates a fixed-order batch before storage or
// readback. Equal timestamps are ordered by the immutable 16-byte trade ID.
func ValidateCEXMarketTrades(trades []CEXMarketTrade) error {
	if len(trades) == 0 {
		return fmt.Errorf("%w: empty batch", ErrCEXMarketTradeInvalid)
	}

	var previous CEXMarketTrade

	for i, trade := range trades {
		if !isCanonicalPositiveDecimal(trade.Price) || !isCanonicalPositiveDecimal(trade.Size) {
			return fmt.Errorf(
				"%w: noncanonical price or size at index=%d",
				ErrCEXMarketTradeInvalid,
				i,
			)
		}

		if trade.Side != CEXMarketTradeSideBuy && trade.Side != CEXMarketTradeSideSell {
			return fmt.Errorf("%w: side at index=%d", ErrCEXMarketTradeInvalid, i)
		}

		if trade.SourceTimeMS == 0 || trade.TradeID == [16]byte{} {
			return fmt.Errorf(
				"%w: source time or trade id at index=%d",
				ErrCEXMarketTradeInvalid,
				i,
			)
		}

		if i > 0 && !cexMarketTradeStrictlyAfter(previous, trade) {
			return fmt.Errorf(
				"%w: unordered or duplicate trade at index=%d",
				ErrCEXMarketTradeInvalid,
				i,
			)
		}

		previous = trade
	}

	return nil
}

func cexMarketTradeStrictlyAfter(previous CEXMarketTrade, next CEXMarketTrade) bool {
	if next.SourceTimeMS != previous.SourceTimeMS {
		return next.SourceTimeMS > previous.SourceTimeMS
	}

	return bytes.Compare(next.TradeID[:], previous.TradeID[:]) > 0
}

func isCanonicalPositiveDecimal(value string) bool {
	if value == "" || value[0] == '+' || value[0] == '-' {
		return false
	}

	dot := -1

	for i := range len(value) {
		c := value[i]
		if c == '.' {
			if dot >= 0 || i == 0 || i == len(value)-1 {
				return false
			}

			dot = i

			continue
		}

		if c < '0' || c > '9' {
			return false
		}
	}

	if len(value) > 1 && value[0] == '0' && value[1] != '.' {
		return false
	}

	if dot >= 0 && value[len(value)-1] == '0' {
		return false
	}

	for i := range len(value) {
		if value[i] >= '1' && value[i] <= '9' {
			return true
		}
	}

	return false
}

const ErrCEXMarketTradeBatchRefInvalid sdkerrors.SDKError = "invalid cex market trade batch ref"

// CEXMarketTradeBatchRef identifies one immutable compact CEX trade batch.
// The payload itself remains in the later storage owner; this wire contract
// carries only the exact registered market identity and bounded provenance.
type CEXMarketTradeBatchRef struct {
	ExchangeID        CEXExchangeID   `json:"exchangeId"        cbor:"1,keyasint"`
	MarketTypeID      CEXMarketTypeID `json:"marketTypeId"      cbor:"2,keyasint"`
	SymbolID          CEXSymbolID     `json:"symbolId"          cbor:"3,keyasint"`
	BatchID           uint64          `json:"batchId"           cbor:"4,keyasint"`
	FirstSourceTimeMS uint64          `json:"firstSourceTimeMs" cbor:"5,keyasint"`
	LastSourceTimeMS  uint64          `json:"lastSourceTimeMs"  cbor:"6,keyasint"`
	TradeCount        uint32          `json:"tradeCount"        cbor:"7,keyasint"`
	EncodedBytes      uint32          `json:"encodedBytes"      cbor:"8,keyasint"`
	PayloadSHA256     [32]byte        `json:"payloadSha256"     cbor:"9,keyasint"`
}

// Validate rejects incomplete, unordered, or unregistered trade-batch refs
// before a producer or consumer can treat the reference as durable provenance.
func (r CEXMarketTradeBatchRef) Validate() error {
	if r.ExchangeID == 0 {
		return fmt.Errorf("%w: exchange_id is zero", ErrCEXMarketTradeBatchRefInvalid)
	}

	if _, ok := DefaultOrderBookIDRegistry.ExchangeLabel(r.ExchangeID); !ok {
		return fmt.Errorf(
			"%w: unknown exchange_id=%d",
			ErrCEXMarketTradeBatchRefInvalid,
			r.ExchangeID,
		)
	}

	if r.MarketTypeID == 0 {
		return fmt.Errorf("%w: market_type_id is zero", ErrCEXMarketTradeBatchRefInvalid)
	}

	if _, ok := DefaultOrderBookIDRegistry.MarketTypeLabel(r.MarketTypeID); !ok {
		return fmt.Errorf(
			"%w: unknown market_type_id=%d",
			ErrCEXMarketTradeBatchRefInvalid,
			r.MarketTypeID,
		)
	}

	if r.SymbolID == 0 {
		return fmt.Errorf("%w: symbol_id is zero", ErrCEXMarketTradeBatchRefInvalid)
	}

	if _, ok := DefaultOrderBookIDRegistry.SymbolLabel(
		r.ExchangeID,
		r.MarketTypeID,
		r.SymbolID,
	); !ok {
		return fmt.Errorf(
			"%w: unknown symbol identity=%d/%d/%d",
			ErrCEXMarketTradeBatchRefInvalid,
			r.ExchangeID,
			r.MarketTypeID,
			r.SymbolID,
		)
	}

	if r.BatchID == 0 {
		return fmt.Errorf("%w: batch_id is zero", ErrCEXMarketTradeBatchRefInvalid)
	}

	if r.FirstSourceTimeMS == 0 || r.LastSourceTimeMS == 0 {
		return fmt.Errorf("%w: source time is zero", ErrCEXMarketTradeBatchRefInvalid)
	}

	if r.FirstSourceTimeMS > r.LastSourceTimeMS {
		return fmt.Errorf("%w: source time range is reversed", ErrCEXMarketTradeBatchRefInvalid)
	}

	if r.TradeCount == 0 {
		return fmt.Errorf("%w: trade_count is zero", ErrCEXMarketTradeBatchRefInvalid)
	}

	if r.EncodedBytes == 0 {
		return fmt.Errorf("%w: encoded_bytes is zero", ErrCEXMarketTradeBatchRefInvalid)
	}

	if r.PayloadSHA256 == [32]byte{} {
		return fmt.Errorf("%w: payload_sha256 is zero", ErrCEXMarketTradeBatchRefInvalid)
	}

	return nil
}

type HyperliquidPublicDataKind string

const HyperliquidPublicDataKindAllMids HyperliquidPublicDataKind = "all_mids"

type HyperliquidNetwork string

const (
	HyperliquidNetworkMainnet HyperliquidNetwork = "mainnet"
	HyperliquidNetworkTestnet HyperliquidNetwork = "testnet"
)

type HyperliquidMarketType string

const (
	HyperliquidMarketTypePerp HyperliquidMarketType = "perp"
	HyperliquidMarketTypeSpot HyperliquidMarketType = "spot"
)

// HyperliquidAllMidsRef is a typed public-data sample from Hyperliquid allMids.
// It is intentionally separate from CEXOrderBookRef: allMids carries one market
// scalar, not an order-book snapshot.
type HyperliquidAllMidsRef struct {
	Kind              HyperliquidPublicDataKind `json:"kind"              cbor:"1,keyasint"`
	Network           HyperliquidNetwork        `json:"network"           cbor:"2,keyasint"`
	MarketType        HyperliquidMarketType     `json:"marketType"        cbor:"3,keyasint"`
	AssetID           uint32                    `json:"assetId"           cbor:"4,keyasint"`
	Symbol            string                    `json:"symbol"            cbor:"5,keyasint"`
	MidPriceQuoteUnit string                    `json:"midPriceQuoteUnit" cbor:"6,keyasint"`
	FetchedAtUnixMS   uint64                    `json:"fetchedAtUnixMs"   cbor:"7,keyasint"`
}

type ChainType uint32
