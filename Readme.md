# Pelagos Go SDK
## Introduction

Pelagos Go SDK is the toolkit Pelagos validators expect you to use when you author an appchain. An appchain is your application-specific runtime packaged as a Docker container that runs alongside the validator stack, consumes consensus snapshots, and publishes deterministic side effects. The Go SDK supplies the runtime harness for loading consensus batches, sequencing your transactions, coordinating multichain reads, and emitting external transactions that other chains must see.

This README concentrates on that Go-centric workflow. It assumes you will start from the public example repository, adapt its `docker-compose` topology, and keep all deterministic logic inside the container that hosts your appchain. 

## Repository orientation

The Go SDK lives under `gosdk/`. The directories and modules below map to the core capabilities your appchain will plug together:

- **Runtime harness (`gosdk/appchain.go`)** – orchestrates configuration, storage wiring, gRPC servers, and the main execution loop that pulls consensus batches into your state transition.
- **Core appchain types (`gosdk/apptypes/`)** – supplies shared interfaces for transactions, receipts, batches, and external payloads so your business logic can interoperate with the validator stack.
- **Transaction pool (`gosdk/txpool/`)** – manages queued transactions, batching, and hash verification so validators stay in sync with the payloads your appchain will execute.
- **Multichain access (`gosdk/multichain.go`)** – opens deterministic, read-only windows into Pelagos-hosted data sets for other chains (EVM, Solana, etc.) and enforces the blocking semantics your runtime expects before processing a batch.
- **Subscription control (`gosdk/subscriber.go`)** – lets you declare which external contracts, addresses, or topics must be tracked, ensuring multichain readers deliver the events your appchain requires.
- **Receipt storage (`gosdk/receipt/`)** – persists execution receipts keyed by transaction hash so clients can audit outcomes and fetchers can relay external payloads.
- **External transaction builders (`gosdk/external/`)** – helps encode cross-chain payloads (EVM, Solana, additional targets) that your batch processor may emit after state transitions succeed.
- **JSON-RPC server (`gosdk/rpc/`)** – provides a scaffold for transaction submission, state queries, and health endpoints that you can extend with appchain-specific methods.
- **Token helpers (`gosdk/library/tokens/`)** – includes reference codecs for reading token balances and transfers from multichain data sets when your appchain logic depends on them.
- **Generated gRPC bindings (`gosdk/proto/`)** – packages the emitter and health service stubs so your runtime can expose the same APIs validators call to stream execution outputs and monitor liveness.

## Quick start via the example appchain

The fastest path to a working Pelagos appchain is to fork [`0xAtelerix/example`](https://github.com/0xAtelerix/example) and use it as your integration harness.

1. **Fork and clone the template.** Create a GitHub fork under your organization, then clone it locally alongside this SDK repository. The example already vendors `gosdk` as a module dependency and is structured for direct customization.
2. **Review the Docker composition.** The root `docker-compose.yml` spins up:
    - a validator service with the consensus stack and MDBX volumes that mirror what runs in production,
    - a fetcher that requests transaction batches and external payloads from validators,
    - your appchain container, built from the local Dockerfile, which links to the validator(and supporting services (PostgreSQL, Redis, or other caches) when the example demonstrates richer workflows).
3. **Run the stack.** From the example repository, execute `docker-compose up --build`. This compiles your appchain binary, builds the container image, and starts the validator, fetcher, and appchain services. Keep the compose logs open; they surface consensus progress, batch ingestion, and RPC traffic that help diagnose configuration issues.
4. **Insert your business logic.** Modify the Go modules inside the example project to:
    - implement your `Transaction` and `Receipt` types,
    - implement your `StateTransitionInterface` and batch processor,
    - extend the JSON-RPC server for custom submission or query endpoints,
    - subscribe to external datasets through `MultichainStateAccess` when your appchain must block on foreign chain data.
      The example keeps these hooks in isolated packages so you can replace them without rewriting the compose workflow.
5. **Iterate with docker-compose.** Each code change only requires rebuilding the appchain container (`docker-compose build appchain`) or restarting the stack (`docker-compose up --build appchain`) to verify deterministic behavior against the validator snapshot stream.

Once the template behaves as expected locally, you can push the forked repository to your Pelagos validator partners for staging. They reuse the same compose topology, ensuring your appchain container integrates with the network exactly as tested.

## State management and batch processing

To see how the pieces line up, take the minimal appchain scaffold from `gosdk/example_app_test.go` and imagine turning it into a counter service. The example already wires together the SDK primitives:

- `ExampleTransaction` implements `apptypes.AppTransaction`. Each transaction carries a sender and the amount they want to add to their counter.
- `ExampleReceipt` satisfies `apptypes.Receipt`, so the runtime can persist execution outcomes and let clients query them later.
- `ExampleBatchProcesser` is plugged into `NewAppchain` so that every consensus batch is pushed through your state transition.

With those building blocks, expanding the test into a working appchain is a matter of filling in the placeholders. Assume you created an MDBX bucket called `bucketCounters` and helper functions `readCounter`/`writeCounter` that load and persist balances:

```go
func (tx ExampleTransaction[ExampleReceipt]) Process(rw kv.RwTx) (ExampleReceipt, []apptypes.ExternalTransaction, error) {
    bucket, _ := rw.RwCursor(bucketCounters)
    current := readCounter(bucket, tx.Sender)
    next := current + uint64(tx.Value)
    writeCounter(bucket, tx.Sender, next)
    return ExampleReceipt{ /* include the new balance if you wish */ }, nil, nil
}

func (p ExampleBatchProcesser[ExampleTransaction[ExampleReceipt], ExampleReceipt]) ProcessBatch(
    batch apptypes.Batch[ExampleTransaction[ExampleReceipt], ExampleReceipt],
    rw kv.RwTx,
) ([]ExampleReceipt, []apptypes.ExternalTransaction, error) {
    receipts := make([]ExampleReceipt, 0, len(batch.Transactions))
    for _, tx := range batch.Transactions {
        receipt, _, err := tx.Process(rw)
        if err != nil {
            return nil, nil, err
        }
		
        receipts = append(receipts, receipt)
    }
    return receipts, nil, nil
}
```

The SDK handles the rest. `Appchain.Run` opens an MDBX write transaction, calls `ProcessBatch`, stores the receipts you returned, calculates the new state root, builds the block header with `ExampleBlock`, and finally commits. If an error bubbles up at any stage, the write transaction is rolled back and the same batch is retried until it succeeds. Because every mutation happens through the `kv.RwTx` instance that `ProcessBatch` receives, the state changes are deterministic and replayable on any validator.

As your business logic grows you can extend `ProcessBatch` to derive external transactions, update metrics, or split work across helper functions. The runtime contract stays the same: consume the ordered transactions in the batch, emit receipts and external transactions, and let the SDK handle durability.

## Multichain data access and synchronization

The same example test also shows how to bootstrap multichain reads: it creates a `Subscriber`, opens the MDBX databases via `NewMultichainStateAccessDB`, and then constructs `MultichainStateAccess`. Turning that into a real workflow involves three extra steps.

1. **Describe the data you need.** During startup, call the subscriber helpers before you run the appchain. `appchainCtx` is the context you pass into `Appchain.Run`, so canceling it tears down the subscriptions cleanly:

   ```go
   subscriber.SubscribeEthContract(appchainCtx, common.HexToAddress(counterManager))
   subscriber.SubscribeSolanaAddress(appchainCtx, mySolanaProgram)
   ```

   These declarations tell the fetcher which external logs and blocks must be present before a batch is handed to your runtime. You only subscribe to what your logic actually reads, which keeps MDBX snapshots small and deterministic.

2. **React to finalized blocks.** The consensus snapshot only references external data via `apptypes.ExternalBlock`:

   ```go
   type ExternalBlock struct {
       ChainID     uint64   `cbor:"1,keyasint"`
       BlockNumber uint64   `cbor:"2,keyasint"`
       BlockHash   [32]byte `cbor:"3,keyasint"`
   }
   ```

   When `ProcessBlock` fires, use that triple to pull the payload through `MultichainStateAccess`. The helper waits until fetchers populate the MDBX snapshot, so "block not found" never surfaces (the snippet below aliases the SDK import as `gosdk`):

   ```go
   func (s *StateTransition) ProcessBlock(
       ctx context.Context,
       ref apptypes.ExternalBlock,
       rw kv.RwTx,
   ) ([]apptypes.ExternalTransaction, error) {
       chain := apptypes.ChainType(ref.ChainID)

       switch {
       case gosdk.IsEvmChain(chain):
           ethBlock, err := s.MultiChain.EthBlock(ctx, ref)
           if err != nil {
               return nil, err
           }
           receipts, err := s.MultiChain.EthReceipts(ctx, ref)
           if err != nil {
               return nil, err
           }
           handleEthEffects(rw, ethBlock, receipts)

       case gosdk.IsSolanaChain(chain):
           solBlock, err := s.MultiChain.SolanaBlock(ctx, ref)
           if err != nil {
               return nil, err
           }
           handleSolanaEffects(rw, solBlock)
       }

       return nil, nil
   }
   ```

   Because the fetchers gate `ProcessBlock` until the referenced block is present, your logic can concentrate on decoding events and updating state within the same MDBX transaction the batch processor opened.

Following this pattern you can enrich the example appchain with multichain guardrails: subscribe to the precise contracts you care about, process external blocks deterministically, and surface their effects through your own transaction handlers or emitted external payloads.

## External transactions

When your appchain finishes processing a consensus batch it can queue additional work for other chains. Those cross-chain intents are represented by `apptypes.ExternalTransaction`, a tiny structure that only captures the destination `ChainID` and the raw `Tx` bytes you encoded. The SDK deliberately keeps the schema this small so the appchain remains in full control of the on-wire format.

1. **Build the payload inside your state transition.** From `Process` or `ProcessBatch`, return a slice of `ExternalTransaction` values alongside receipts. You can construct them manually or use the helper from `gosdk/external`:

   ```go
   ext, err := external.NewExTxBuilder().
       EthereumSepolia().
       SetPayload(abiEncodedMessage).
       Build()
   if err != nil {
       return nil, nil, err
   }
   externalTxs = append(externalTxs, ext)
   ```

   The builder simply fills in well-known Pelagos chain IDs (Ethereum, Polygon, BSC, Solana main/dev nets, and so on) and copies your deterministic payload bytes. How you encode those bytes—ABI, Borsh, JSON—is entirely up to the appchain and must match the contract/program that will decode them.
2. **Allow validators and the TSS appchain to relay them.** After the batch commits, validators aggregate the emitted `ExternalTransaction` objects, reach quorum on the set that should leave the network, and hand them to the threshold-signing (TSS) appchain. That service applies the necessary signature material and broadcasts the transactions to the target chains.
3. **Let operators retry transport failures.** Validators and the TSS appchain are responsible for resubmitting payloads that fail for transient reasons such as gas price spikes or RPC outages. If a transaction reverts for logical reasons, address the issue in your state transition and emit a corrected payload in a later batch.

Because the SDK treats the payload as opaque bytes, include any ids or replay protection directly in your encoding. Keep the serialization deterministic so all validators agree on the content hash they sign, and document the expected fee handling for operators who monitor the TSS queues.

## Designing custom transaction formats

Every appchain defines its own transaction schema. The SDK only requires that your type implements `apptypes.AppTransaction`, meaning you expose a deterministic `Hash()` and a `Process` method that mutates state and returns a receipt/external payloads. Within those guardrails you can pick whichever field layout, encoding, and validation logic match your product.

1. **Pick a deterministic encoding.** The stock transaction pool persists pending payloads by CBOR-encoding the transaction struct you hand it. Exported fields with explicit `cbor:"key"` tags give you total control over byte layout, so multiple validators marshal the exact same payload. If you prefer an alternate on-wire format (ABI, JSON, Borsh), convert it at the network edge—e.g., decode incoming RPC requests into your Go struct, then let the txpool store the canonical CBOR form.

2. **Derive and verify hashes up front.** Because the txpool indexes entries by `tx.Hash()`, choose a hashing scheme that matches how clients identify transactions (Keccak, SHA-256, etc.) and make sure the value covers every field you serialize. Use the `Process` method to reject malformed inputs before they can mutate state; returning an error automatically keeps the batch from committing and surfaces the failure to operators.

3. **Wire anti-spam checks into submission paths.** Your custom RPC handlers can throttle or rate-limit before forwarding payloads to `TxPool.AddTransaction`, while `Process` can enforce business rules such as nonce gaps or balance minimums. The txpool makes it easy to inspect current entries (`GetPendingTransactions`) or prune them when quotas are exceeded, keeping validator queues lean.

4. **Track lifecycle and receipts.** Pair your transaction type with a receipt struct that implements `apptypes.Receipt` so callers can read deterministic outcomes after execution. The pool reports coarse status through `GetTransactionStatus`, letting you expose JSON-RPC methods or metrics that show whether a payload is still pending, batched, or fully processed. Combine those signals with structured logging inside `Process` to build end-to-end traceability for debugging and audits.

## Extending JSON-RPC APIs

Your appchain’s RPC surface is how wallets, indexers, and backend services submit transactions or query state. The SDK ships a composable server scaffold in [`gosdk/rpc`](gosdk/rpc/README.md) so you can start from a minimal JSON-RPC 2.0 implementation and layer on only the handlers you need.

1. **Bootstrap the standard server.** Create a server instance with `rpc.NewStandardRPCServer()` during appchain startup (often alongside your txpool and MDBX wiring). The helper exposes `StartHTTPServer`, which takes a context and listen address so you can tie shutdown to the same lifecycle signals that stop `Appchain.Run`.

2. **Register the built-in method sets.** Call `rpc.AddStandardMethods[YourTx, YourReceipt](server, appchainDB, txpool)` to expose the default suite—`sendTransaction`, `getTransactionStatus`, `getPendingTransactions`, and `getTransactionReceipt`. If you prefer a narrower surface, pick from `AddTransactionMethods`, `AddTxPoolMethods`, or `AddReceiptMethods`. All of these helpers live in the [`gosdk/rpc` README](gosdk/rpc/README.md) with complete usage samples.

3. **Add custom endpoints.** Use `server.AddCustomMethod` to register domain-specific calls that read from your MDBX state or query derived caches. Handlers receive a `context.Context` plus raw parameters, so you can layer validation, authentication, or tracing before touching storage.

4. **Expose health and observability hooks.** The standard server already mounts a `/health` endpoint. Pair it with Prometheus metrics or structured logs emitted from your handlers so operators can monitor latency, error rates, and txpool backlog. When running under Docker Compose, map the HTTP port in `docker-compose.yml` so local tooling can probe the service.
