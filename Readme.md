# Pelagos Go SDK

## Introduction

Pelagos Go SDK is the toolkit Pelagos validators expect you to use when you author an appchain. An appchain is your application-specific runtime packaged as a Docker container that runs alongside the validator stack, consumes consensus snapshots, and publishes deterministic side effects. The Go SDK supplies the runtime harness for loading consensus batches, sequencing your transactions, coordinating multichain reads, and emitting external transactions that other chains must see.

This README concentrates on that Go-centric workflow. It assumes you will start from the public [example](https://github.com/0xAtelerix/example) repository, adapt its `docker-compose` topology, and keep all deterministic logic inside the container that hosts your appchain.

## Table of contents

- [Introduction](#introduction)
- [TL;DR Quickstart](#tldr-quickstart)
- [Repository orientation](#repository-orientation)
- [Quickstart](#quickstart)
- [Data directory structure](#data-directory-structure)
- [State management and batch processing](#state-management-and-batch-processing)
- [Configuring required chains](#configuring-required-chains)
- [Multichain data access](#multichain-data-access)
- [External transactions](#external-transactions)
- [Designing custom transaction formats](#designing-custom-transaction-formats)
- [Extending JSON-RPC APIs](#extending-json-rpc-apis)
- [Testing and debugging routines](#testing-and-debugging-routines)
- [Determinism checklist](#determinism-checklist)
- [FAQ](#faq)
- [Glossary](#glossary)

## TL;DR Quickstart

Use the public example as a starting point. Replace placeholders with your own values as you customize.

```bash
# 1) Clone the example template next to this SDK repo
git clone https://github.com/0xAtelerix/example my-appchain
cd my-appchain

# 2) Launch validator + fetcher + your appchain container
docker compose up --build

# 3) Probe health (HTTP server in your appchain's process)
# Replace <rpc_host>:<rpc_port> with the port you configured
curl -s http://<rpc_host>:<rpc_port>/health

# 4) Send a transaction via JSON-RPC (method names provided by the SDK helpers)
curl -s -X POST http://<rpc_host>:<rpc_port>/rpc -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"sendTransaction","params":[{"sender":"0xabc...","value":1}]}'

# 5) Rebuild just your appchain service as you iterate
docker compose build appchain && docker compose up appchain
```

## Repository orientation

The Go SDK lives under `gosdk/`. The directories and modules below map to the core capabilities your appchain will plug together:

- **Runtime harness (`gosdk/appchain.go`)** – orchestrates configuration, storage wiring, gRPC servers, and the main execution loop that pulls consensus batches into your state transition.
- **Initialization (`gosdk/init.go`)** – provides `InitApp()` to bootstrap all common components (databases, multichain access, subscriber) with sensible defaults. Also defines path helpers for consistent directory layout.
- **Core appchain types (`gosdk/apptypes/`)** – supplies shared interfaces for transactions, receipts, batches, and external payloads so your business logic can interoperate with the validator stack.
- **Transaction pool (`gosdk/txpool/`)** – manages queued transactions, batching, and hash verification so validators stay in sync with the payloads your appchain will execute.
- **Multichain access (`gosdk/multichain.go`, `gosdk/multichain_sql.go`)** – opens deterministic, read-only windows into Pelagos-hosted data sets for other chains (EVM, Solana, etc.). Supports both MDBX and SQLite backends via `MultichainStateAccessor` interface.
- **Subscription control (`gosdk/subscriber.go`)** – lets you declare which external contracts, addresses, or topics must be tracked, ensuring multichain readers deliver the events your appchain requires.
- **Receipt storage (`gosdk/receipt/`)** – persists execution receipts keyed by transaction hash so clients can audit outcomes and fetchers can relay external payloads.
- **External transaction builders (`gosdk/external/`)** – helps encode cross-chain payloads (EVM, Solana, additional targets) that your batch processor may emit after state transitions succeed.
- **JSON-RPC server (`gosdk/rpc/`)** – provides a scaffold for transaction submission, state queries, and health endpoints that you can extend with appchain-specific methods.
- **Token helpers (`gosdk/library/tokens/`)** – includes reference codecs for reading token balances and transfers from multichain data sets when your appchain logic depends on them.
- **Generated gRPC bindings (`gosdk/proto/`)** – packages the emitter and health service stubs so your runtime can expose the same APIs validators call to stream execution outputs and monitor liveness.

## Quickstart

The fastest path to a working Pelagos appchain is to fork [`0xAtelerix/example`](https://github.com/0xAtelerix/example) and use it as your integration harness.

1. **Fork and clone the template.** Create a GitHub fork under your organization, then clone it locally alongside this SDK repository. The example already vendors `gosdk` as a module dependency and is structured for direct customization.

2. **Review the Docker composition.** The root `docker-compose.yml` spins up:
    - a validator service with the consensus stack and MDBX volumes that mirror what runs in production,
    - a fetcher that requests transaction batches and external payloads from validators,
    - your appchain container, built from the local Dockerfile, which links to the validator network, and supporting services (PostgreSQL, Redis, or other caches) when the example demonstrates richer workflows.

3. **Run the stack.** From the example repository, execute `docker-compose up --build`. This compiles your appchain binary, builds the container image, and starts the validator, fetcher, and appchain services. Keep the compose logs open; they show consensus progress, batch ingestion, and RPC traffic for diagnosis.

4. **Insert your business logic.** Modify the Go modules inside the example project to:
    - implement your `Transaction` and `Receipt` types,
    - implement your `ExternalBlockProcessor` to handle external chain data,
    - extend the JSON-RPC server for custom submission or query endpoints,
    - subscribe to external datasets through `MultichainStateAccess` when your appchain must block on foreign chain data.
      The example keeps these hooks in isolated packages so you can replace them without rewriting the compose workflow.

5. **Iterate with docker-compose.** Rebuild the appchain container (`docker-compose build appchain`) or restart the service (`docker-compose up --build appchain`) to verify deterministic behavior against the validator snapshot stream.

Once the template behaves as expected locally, you can push the forked repository to your Pelagos validator partners for staging. They reuse the same compose topology, ensuring your appchain container integrates with the network exactly as tested.

## Data directory structure

The SDK and pelacli share a common data directory (default: `./data`). Understanding this structure helps with debugging and configuration:

```
./data/
├── multichain/           # External chain data (written by pelacli, read by appchain)
│   ├── 11155111/         # Ethereum Sepolia
│   │   └── sqlite        # SQLite database with blocks and receipts
│   └── 80002/            # Polygon Amoy
│       └── sqlite
├── events/               # Consensus events (written by pelacli, read by appchain)
├── fetcher/              # Transaction batches (written by pelacli, read by appchain)
│   └── 42/               # Per-appchain (chainID=42)
│       └── mdbx.dat
├── appchain/             # Appchain state (written by appchain)
│   └── 42/
│       └── mdbx.dat
└── local/                # Local node data like txpool (written by appchain)
    └── 42/
        └── mdbx.dat
```

Path helpers in `gosdk/init.go`:
- `gosdk.ChainDBPath(dataDir, chainID)` → `{dataDir}/multichain/{chainID}`
- `gosdk.TxBatchPath(dataDir, chainID)` → `{dataDir}/fetcher/{chainID}`
- `gosdk.AppchainDBPath(dataDir, chainID)` → `{dataDir}/appchain/{chainID}`
- `gosdk.LocalDBPath(dataDir, chainID)` → `{dataDir}/local/{chainID}`

## State management and batch processing

The SDK provides a streamlined initialization flow through `gosdk.InitApp()` that sets up all common components with sensible defaults. Here's how a typical appchain main function looks:

```go
func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Initialize all common components
    storage, config, err := gosdk.InitApp[MyTransaction[MyReceipt], MyReceipt](
        ctx,
        gosdk.InitConfig{
            ChainID:        42,
            DataDir:        "./data",
            EmitterPort:    ":9090",
            RequiredChains: []uint64{uint64(gosdk.EthereumSepoliaChainID), uint64(gosdk.PolygonAmoyChainID)}, // Sepolia + Polygon Amoy
            CustomTables:   application.Tables(),
        },
    )
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to init appchain")
    }
    defer storage.Close()

    // Create appchain with SDK's default batch processor
    appchain := gosdk.NewAppchain(
        storage,
        config,
        gosdk.NewDefaultBatchProcessor[MyTransaction[MyReceipt], MyReceipt](
            NewExtBlockProcessor(storage.Multichain()),
            storage.Multichain(),
            storage.Subscriber(),
        ),
        MyBlockConstructor,
    )

    // Run appchain
    go appchain.Run(ctx)

    // Start RPC server...
}
```

`InitApp()` returns:
- `Storage` – contains all databases and storage components (appchain DB, txpool, multichain accessor, subscriber)
- `Config` – the fully populated `AppchainConfig` with paths and settings

The `Storage` object provides accessor methods:
- `storage.AppchainDB()` – main appchain database for blocks, checkpoints, and receipts
- `storage.TxPool()` – transaction pool for managing pending transactions
- `storage.Multichain()` – multichain state accessor for external chain data (EVM, Solana)
- `storage.Subscriber()` – manages external chain subscriptions for filtering
- `storage.TxBatchDB()` – read-only database for transaction batches from pelacli

### Implementing your transaction type

Your transaction type must implement `apptypes.AppTransaction`. Each transaction carries your domain-specific fields and a `Process` method that mutates state:

The same example test also shows how to bootstrap multichain reads. You have two backends:

- **MDBX (default)** – open with `NewMultichainStateAccessDB` or `NewMultichainStateAccessDBWith` (pass a custom opener). Then wrap with `NewMultichainStateAccess`.
- **SQLite** – if your fetcher produces SQLite snapshots, open with `NewMultichainStateAccessSQLDB` and wrap with `NewMultichainStateAccessSQL` (implements the same `MultichainReader` interface).

Once you have a `MultichainReader`, construct `MultichainStateAccess` (MDBX) or use the SQLite reader directly and hand it to `BatchProcesser`. Turning that into a real workflow involves three extra steps.

```go
type MyTransaction[R MyReceipt] struct {
    Sender string `cbor:"1,keyasint"`
    Value  int    `cbor:"2,keyasint"`
}

func (tx MyTransaction[R]) Hash() [32]byte {
    // Return deterministic hash of transaction
    return sha256.Sum256([]byte(tx.Sender + strconv.Itoa(tx.Value)))
}

func (tx MyTransaction[R]) Process(rw kv.RwTx) (R, []apptypes.ExternalTransaction, error) {
    // Read current state
    current := readCounter(rw, tx.Sender)

    // Update state
    next := current + uint64(tx.Value)
    writeCounter(rw, tx.Sender, next)

    // Return receipt and any external transactions to emit
    return MyReceipt{NewBalance: next}, nil, nil
}
```

### Batch processing

The SDK provides a `BatchProcessor` interface for processing batches. Most apps use `DefaultBatchProcessor` which:
1. Processes each app transaction by calling `tx.Process()`
2. Filters external blocks by your subscriptions
3. Delegates matched external blocks to your `ExternalBlockProcessor`

For custom batch processing logic, implement the `BatchProcessor` interface:

```go
type BatchProcessor[appTx apptypes.AppTransaction[R], R apptypes.Receipt] interface {
    ProcessBatch(
        ctx context.Context,
        batch apptypes.Batch[appTx, R],
        dbtx kv.RwTx,
    ) ([]R, []apptypes.ExternalTransaction, error)
}
```

Usage:

```go
// Default - use SDK's DefaultBatchProcessor
appchain := gosdk.NewAppchain(
    storage,
    config,
    gosdk.NewDefaultBatchProcessor[MyTx, MyReceipt](
        myExtBlockProcessor,
        storage.Multichain(),
        storage.Subscriber(),
    ),
    blockBuilder,
)

// Custom - provide your own BatchProcessor implementation
appchain := gosdk.NewAppchain(
    storage,
    config,
    myCustomBatchProcessor,
    blockBuilder,
)
```

When implementing a custom `BatchProcessor`, you have full control over batch processing logic. You can:
- Use `ExternalBlockProcessor` internally (like `DefaultBatchProcessor` does)
- Handle external blocks directly in `ProcessBatch`
- Use a completely different architecture

### Implementing ExternalBlockProcessor

To handle data from external chains (EVM, Solana), implement the `ExternalBlockProcessor` interface:

```go
// Verify your type implements the interface
var _ gosdk.ExternalBlockProcessor = &ExtBlockProcessor{}

type ExtBlockProcessor struct {
    multichain gosdk.MultichainStateAccessor
}

func NewExtBlockProcessor(multichain gosdk.MultichainStateAccessor) *ExtBlockProcessor {
    return &ExtBlockProcessor{multichain: multichain}
}

func (p *ExtBlockProcessor) ProcessBlock(
    block apptypes.ExternalBlock,
    dbtx kv.RwTx,
) ([]apptypes.ExternalTransaction, error) {
    ctx := context.Background()
    var externalTxs []apptypes.ExternalTransaction

    switch {
    case gosdk.IsEvmChain(apptypes.ChainType(block.ChainID)):
        // Get EVM block and receipts from multichain database
        evmBlock, err := p.multichain.EVMBlock(ctx, block)
        if err != nil {
            return nil, err
        }

        receipts, err := p.multichain.EVMReceipts(ctx, block)
        if err != nil {
            return nil, err
        }

        // Process logs from receipts
        for _, receipt := range receipts {
            for _, log := range receipt.Logs {
                // Handle events, update state via dbtx
                // Optionally emit cross-chain transactions
            }
        }

        // Use evmBlock for block-level data (timestamp, etc.)
        _ = evmBlock

    case gosdk.IsSolanaChain(apptypes.ChainType(block.ChainID)):
        solBlock, err := p.multichain.SolanaBlock(ctx, block)
        if err != nil {
            return nil, err
        }

        // Process Solana transactions
        for _, tx := range solBlock.Transactions {
            // Handle program interactions
            _ = tx
        }
    }

    return externalTxs, nil
}
```

The SDK automatically:
- Opens an MDBX write transaction
- Calls your transaction's `Process()` for each tx in the batch
- Calls your `ExternalBlockProcessor.ProcessBlock()` for matched external blocks
- Stores receipts
- Calculates the new state root
- Builds the block header
- Commits the transaction

If an error occurs, the write transaction is rolled back and the batch is retried.

## Configuring required chains

The SDK needs to know which external chains your appchain will read data from. This is configured via the `RequiredChains` field in `InitConfig`:

```go
storage, config, err := gosdk.InitApp[MyTx, MyReceipt](
    ctx,
    gosdk.InitConfig{
        ChainID:        42,
        DataDir:        "./data",
        RequiredChains: []uint64{uint64(gosdk.EthereumSepoliaChainID), uint64(gosdk.PolygonAmoyChainID)}, // Ethereum Sepolia + Polygon Amoy
    },
)
```

### Default behavior

**If you omit `RequiredChains` or provide an empty slice, the SDK defaults to Ethereum Sepolia (chain ID 11155111).** This ensures:
- Zero-config demos work out of the box
- Alignment with pelacli's default configuration
- New appchains can test multichain features immediately

### Supported chains

The SDK validates chain IDs against known EVM and Solana chains:
- See `gosdk/chain_consts.go` for the full list of supported networks

### How it works

During `InitApp()`:
1. SDK resolves `RequiredChains` (defaults to Sepolia if empty)
2. For each chain, waits for pelacli to populate `{dataDir}/multichain/{chainID}/`
3. Opens multichain database connections for those chains
4. Returns `MultichainStateAccessor` via `storage.Multichain()`

If a required chain's data isn't available (e.g., pelacli hasn't synced it), `InitApp()` will wait until the data directory appears.

### Production configuration

In production, explicitly list the chains your appchain needs:

```go
RequiredChains: []uint64{1, 137}, // Ethereum + Polygon mainnet
```

This prevents waiting for unnecessary chain data and makes your appchain's dependencies clear.

## Multichain data access

During startup, subscribe to the addresses and contracts you care about:

```go
subscriber.SubscribeEthContract(gosdk.EthereumSepoliaChainID, gosdk.EthereumAddress{/* bytes */})
subscriber.SubscribeSolanaAddress(gosdk.SolanaDevnetChainID, gosdk.SolanaAddress{/* bytes */})
```

These declarations tell the fetcher which external data must be present before a batch is processed. The fetchers gate `ProcessBlock` until the referenced block is present, so your logic can safely assume the data exists.

## External transactions

To emit cross-chain transactions, return them from `ProcessBlock`:

```go
// Inside ProcessBlock, after processing an event:
extTx, err := external.NewExTxBuilder(
    abiEncodedPayload,
    gosdk.EthereumSepoliaChainID,
).Build()
if err != nil {
    return nil, err
}
externalTxs = append(externalTxs, extTx)
```

After the batch commits:
1. Validators aggregate the emitted external transactions
2. They reach quorum on which transactions should leave the network
3. The TSS (threshold-signing) appchain applies signatures
4. Transactions are broadcast to target chains

## Designing custom transaction formats

Your transaction type must implement `apptypes.AppTransaction`:

```go
type AppTransaction[R Receipt] interface {
    Hash() [32]byte
    Process(kv.RwTx) (receipt R, externalTxs []ExternalTransaction, err error)
}
```

Guidelines:
1. **Use deterministic encoding.** Use CBOR tags for consistent serialization across validators.
2. **Derive hashes deterministically.** The txpool indexes by `tx.Hash()`, so ensure it covers all serialized fields.
3. **Validate in Process.** Return errors for malformed inputs to prevent state mutations.
4. **Track lifecycle with receipts.** Implement `apptypes.Receipt` so clients can query outcomes.

## Extending JSON-RPC APIs

The SDK provides a composable RPC server:

```go
// Create server
rpcServer := rpc.NewStandardRPCServer(nil)

// Add middleware
rpcServer.AddMiddleware(myMiddleware)

// Add standard methods (sendTransaction, getBlock, getReceipt, etc.)
rpc.AddStandardMethods[MyTx, MyReceipt, MyBlock](
    rpcServer,
    storage.AppchainDB(),
    storage.TxPool(),
    chainID,
)

// Add custom methods
myCustomRPC := NewCustomRPC(rpcServer, storage.AppchainDB())
myCustomRPC.AddRPCMethods()

// Start server
rpcServer.StartHTTPServer(ctx, ":8080")
```

### JSON-RPC quick example

```bash
# Send transaction
curl -s -X POST http://localhost:8080/rpc -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"sendTransaction","params":[{"sender":"0xabc...","value":1}]}'

# Get pending transactions
curl -s -X POST http://localhost:8080/rpc -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"getPendingTransactions","params":[]}'

# Get transaction status
curl -s -X POST http://localhost:8080/rpc -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":3,"method":"getTransactionStatus","params":["<tx_hash>"]}'
```

## Testing and debugging routines

1. **Run tests.** Use `go test ./...` or `make tests` to run unit and integration tests.

2. **Use the example test as a template.** See `gosdk/example_app_test.go` for how to set up an appchain with in-memory storage for testing.

3. **Replay consensus snapshots.** Point your config at archived validator outputs to replay problematic epochs.

4. **Inspect MDBX state.** Use the multichain accessor methods (`EVMBlock`, `EVMReceipts`, `SolanaBlock`) for ad-hoc queries.

5. **Enable observability.** Set a logger via `InitConfig.Logger` and optionally configure Prometheus metrics.

## Determinism checklist

The SDK assumes every validator replays identical state transitions. Keep your logic deterministic:

- Avoid `time.Now`, random values, or network I/O inside `Process`/`ProcessBlock`.
- Do all writes through the provided `kv.RwTx`.
- Iterate maps deterministically (sort keys before ranging when order matters).
- Derive nonces/ids from batch inputs, not local state.
- Use stable transaction encoding (explicit CBOR tags).
- Keep logging outside state mutation paths.

## FAQ

**What runs where?** Your appchain logic runs inside your Docker image. Validators run consensus, fetchers, and host your image alongside them.

**How do I access other chains?** Use `MultichainStateAccessor` after declaring subscriptions. The SDK blocks `ProcessBlock` until referenced external blocks are present.

**How do I emit cross-chain payloads?** Return `apptypes.ExternalTransaction` from your `ProcessBlock`. Validators and the TSS appchain handle signing and broadcast.

**How do I submit transactions?** Start the JSON-RPC server and use the standard methods added via `rpc.AddStandardMethods`.

## Glossary

- **Appchain**: Your deterministic runtime packaged as a Docker image.
- **Batch**: Ordered set of transactions the runtime processes atomically.
- **BatchProcessor**: Interface for processing batches of transactions and external blocks.
- **DefaultBatchProcessor**: SDK's default `BatchProcessor` implementation that handles transaction processing and external block filtering.
- **ExternalBlockProcessor**: Interface you implement to handle external chain data (used by `DefaultBatchProcessor`).
- **External block**: Reference to finalized data from another chain used in processing.
- **Fetcher**: Service that writes transaction batches and external chain data to databases for appchain consumption.
- **Storage**: Contains all databases and storage components (appchain DB, txpool, multichain accessor, subscriber). Returned by `InitApp()`.
- **MultichainStateAccessor**: Interface for reading external chain data (EVM blocks/receipts, Solana blocks). Implementations: `MultichainStateAccessSQL` (SQLite), `MultichainStateAccess` (MDBX).
- **Pelacli**: CLI tool that runs the consensus stub and fetcher for local development.
- **Subscriber**: Component that tracks which external addresses/contracts your appchain cares about.
- **TSS appchain**: Threshold-signing service that transports external transactions.
