# Order-book identity registry — what belongs in it

`cex_order_book_identity.json` is not "every market the venues list". It carries a
deliberate subset, and the exclusions are load-bearing. This file records why, so a
refresh does not re-add things that were removed on purpose.

A symbol absent from the registry fails strategy canonicalisation with
*"must identify a registered CEX venue"* (`validateRegistrySignalIdentity`), so
under-inclusion blocks real markets. Over-inclusion is worse: it makes a market look
authorable that the consumer cannot actually trade.

## Hard constraints — a symbol that violates these cannot work

These are enforced in code downstream. Adding such a row produces a market that
resolves here and then fails in `smart_example`.

### 1. Printable ASCII only

`cextypes.NewCEXSymbol` rejects non-ASCII outright:

```
NewCEXSymbol("BTCUSDT")   -> "BTCUSDT"  err=<nil>
NewCEXSymbol("币安人生USDT") -> ""        err=cex orderbook key field must be printable ascii
```

Binance lists several Chinese-named perps and spot pairs (`币安人生`, `我踏马来了`,
`牛来`, `龙虾`). They must not be added. The registry has never carried a non-ASCII
row, and that is not an accident.

### 2. The quote asset must be splittable

`cextypes.canonicalQuoteSuffixes` is the single quote-suffix list used to split a
symbol into base/quote. Its own comment states the consequence: without a matching
suffix *"the symbol cannot be split into base/quote and never reaches scale
resolution."*

Currently: `USDT0 USDT USDC USDH USDE BUSD USD BTC ETH ARCH CASH DBLN DENIM HORSE
NQ2 NQ TZERO USDAA USDEEE USDG USDL USDOK USDYP USDZZ XAG`

Binance quotes that are **not** in that list, with live pair counts at the time of
writing — all unusable until the suffix list is extended first:

| quote | pairs | | quote | pairs |
|---|---|---|---|---|
| TRY | 308 | | JPY | 27 |
| U | 47 | | BRL | 18 |
| FDUSD | 34 | | BNB | 7 |
| USD1 | 30 | | EUR | 29 |
| IDR | 29 | | | |

Extending `canonicalQuoteSuffixes` is the prerequisite, not the registry entry.
Order in that list is load-bearing (longest-first), so it is not a free append.

## Policy constraints — technically possible, deliberately excluded

These would resolve and split fine. They are out of scope by decision, and adding
them silently widens what the platform claims to support.

### 3. Binance quote asset restricted to USDT and USDC

Set by #56, *"add all binance + mexc USDT/USDC pairs"*. This is Binance/MEXC-scoped: Hyperliquid legitimately carries `USDT0`, `USDH` and `USDE` quotes. `BTC` and `ETH` quotes are
splittable, so roughly 50 Binance cross pairs could be added — they are not, because
settlement and PnL are denominated in stablecoin quotes.

**Check after any refresh:** the registry's Binance quote assets must be exactly
`{USDT, USDC}`.

### 4. Binance perps: `contractType == PERPETUAL` only

Every pre-existing Binance perp row is `PERPETUAL`. Binance also lists:

- **`TRADIFI_PERPETUAL`** — 181 equity/commodity perps (AAPL, TSLA, NVDA, XAU,
  plus HK/KR/CN equities). Excluded for two open problems, not squeamishness:
  `cex.json` `decimals` is *native chain decimals* and an equity has none, and no
  trading-hours or halt handling exists anywhere in `application/` while equities
  are not 24/7.
- **`CURRENT_QUARTER` / `NEXT_QUARTER`** — dated delivery contracts. Expiry and
  settlement are not modelled.

Note the asymmetry, which is correct and easy to get wrong in both directions:
Binance **spot** has no `contractType`, and the registry already carries 45
tokenized-equity spot pairs (`AMDB`, `ARMB`, `AVGOB`, `BABAB`, `COINB`, …). Tokenized
equities are therefore **included on spot** and **excluded on perp**.

**Check after any refresh:** every Binance perp row is `PERPETUAL`.

### Included, despite looking excludable: multiplier contracts

`1000PEPE`, `1000SHIB`, `1000BONK`, `1MBABYDOGE` and similar are Binance contract
multipliers, not tokens — one contract is 1000 PEPE. They **are** in scope: the
registry already carried `1000CAT`, `1000CHEEMS`, `1000SATS` and `1MBABYDOGE` before
any recent refresh, on both spot and perp.

Do not filter them out for looking synthetic. Their base assets have no `cex.json`
scale entry, but that is the general Binance scale gap, not a reason to withhold the
identity.

### 5. Hyperliquid: skip `isDelisted`

`meta` keeps delisted perps in the universe with `isDelisted` set. A shipped delisted
row is a dead market that every membership check approves, and the venue then rejects
the order with an opaque error instead of naming the delisting.

Removal of an already-shipped delisted row is a separate question: symbol ids are
persisted, so deleting a row can orphan stored data. `VINEUSDC` is currently present
and delisted for that reason.

### 6. Binance: `status == TRADING` only

`SETTLING` (perp) and `BREAK` (spot) are Binance's wind-down and halted states. A row
in either is authorable but not tradeable — the same failure mode as a shipped
delisted Hyperliquid perp.

The shipped file currently carries **7 `SETTLING` perps** (`ACXUSDT HFTUSDT ICXUSDT
SCRTUSDT STORJUSDT VANRYUSDT VICUSDT`) and **21 `BREAK` spots**. Those predate this
policy and are not removed for the same persisted-id reason as `VINEUSDC`, but no
refresh should add more.

## Structural rules for adding rows

- **Additions only.** Symbol ids are persisted downstream. Never renumber. Verify
  programmatically that every pre-existing row is byte-identical after a refresh.
- **One id per `(exchange, label)`, shared across market types.** The loader enforces
  this and rejects violations with *"legacy symbol id diverges across market types"*.
  409 labels currently appear in both spot and perp.
- **New ids continue above the exchange-wide maximum**, not the per-market maximum.
- **Hyperliquid rows need `venue_asset_ids`.** A symbol row alone is not enough:
  `ResolveVenueAssetSymbolID` needs the venue's own id — perp universe index, or
  `10000 + spot index` for spot. Without it the consumer fails with *"authored asset
  id resolves nothing"* and Hyperliquid spot markets lose their authoring authority.
- **Bump `version`.**
- `TestOrderBookIDRegistryCoversCompleteStudioMarketCatalogSnapshot` pins per-market
  counts. MEXC's count is a useful control: if it moves, the refresh touched more
  than intended.

## Refresh checklist

Sources: Binance `/api/v3/exchangeInfo` and `/fapi/v1/exchangeInfo` filtered to
`status == TRADING`; Hyperliquid `meta` / `spotMeta` skipping `isDelisted`.

Assert all of these after generating, before opening a PR:

1. Binance quote assets are exactly `{USDT, USDC}`
2. Every Binance perp row is `PERPETUAL`
3. No non-ASCII label or base asset
4. Zero pre-existing rows mutated
5. No duplicate `(exchange, market_type, label)` or `(exchange, market_type, symbol_id)`
6. Every **newly added** Hyperliquid row has a `venue_asset_ids` entry for its
   network. This does **not** hold for the file as a whole: 14 of 518 Hyperliquid
   rows have no mainnet id — the 12 fixtures listed in §5 plus spot `BTCUSDC` and
   `SPXUSDC`, which are testnet-only. Assert it over the diff, not the file.
