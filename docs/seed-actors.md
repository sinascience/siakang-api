# SIAKANG seed actors

Everything `make db-reset` puts in the database for the SIAKANG marketplace, and
what each row is there to prove. This is the single source QA scripts and FE
mocks both read, so nobody has to guess which actor demonstrates which criterion.

Authority: the frozen contract's `x-seed-data`
(`siakang-pipeline/contract/api-v1.yaml`). If this document and the seeders ever
disagree with it, the contract wins.

Seeders: `internal/database/seeders/market/*.sql`, run by `make seed-market`
(and by `make seed` / `make db-setup` / `make db-reset`, after the core
seeders — the market seeders insert into `core.users`, `core.roles` and
`core.user_roles`, so core must go first).

> **These rows are a test fixture, not sample data.** The coordinates, the
> ratings and the availability flag are chosen so that a specific lapak wins the
> automatic match for a specific reason. Change one and you change what the
> acceptance criteria assert.

## Credentials

**Password for every seeded account: `siakang123`.**

Sign in at `POST /core/v1/auth/signin` with `{"login": "<email>", "password": "siakang123"}`.

| Persona | Login | Full name | User id |
|---|---|---|---|
| customer | `budi@siakang.test` | Budi Santoso | `10000000-0000-0000-0000-000000000011` |
| lapak | `joko@siakang.test` | Joko Prasetyo | `10000000-0000-0000-0000-000000000012` |
| lapak | `sari@siakang.test` | Sari Wulandari | `10000000-0000-0000-0000-000000000013` |
| lapak | `agus@siakang.test` | Agus Setiawan | `10000000-0000-0000-0000-000000000014` |

These four are ordinary `core.users` rows with **no client and no company**.
`GET /core/v1/auth/me` therefore omits `company` and `client` entirely (they are
`omitempty` pointers — absent, not `null`), `permissions` is `[]`, and `roles`
carries `customer` or `lapak`. That is correct: no marketplace flow calls
`/core/v1/auth/switch-company`. Do not "fix" it by giving them a company.

The persona itself is a **global** role assignment — `core.user_roles` with
`company_id IS NULL` — which is why no core code changed to support it.

## Wallets

| User | Opening balance | Ledger |
|---|---|---|
| budi | **5 000 000 IDR** | one `topup` entry, `+5000000`, `balance_after_idr = 5000000` |
| joko | 0 | none |
| sari | 0 | none |
| agus | 0 | none |

Budi's balance is deliberately more than one QA pass needs, so every flow in
`goal.md` can run back to back without a top-up endpoint (sprint 1 has none).
The lapaks start empty and are credited by `payout` entries when orders
complete.

Balance and ledger always move together, in one transaction — that is the rule
the whole simulated-payments design rests on, so even the opening balance has a
real ledger row behind it rather than being a bare number.

## Lapak profiles — the matching fixture

Origin for every distance below is **Budi's seeded coordinate,
`-7.9666, 112.6326`** (Malang). Distances are haversine, computed over these
columns; there is no maps API in sprint 1.

| Lapak | Name | lat | lng | Distance from budi | Rating | `is_available` |
|---|---|---|---|---|---|---|
| joko | Servis Elektronik Pak Joko | -7.9750 | 112.6400 | **1.240 km** | **4.8** | `true` |
| sari | Bersih Kilat Sari | -8.0100 | 112.6800 | **7.109 km** | **4.9** | `true` |
| agus | Tukang Kebun Agus | -7.9670 | 112.6330 | **0.063 km** | **5.0** | `false` |

Lapak profile ids: joko `50000000-…-0001`, sari `50000000-…-0002`,
agus `50000000-…-0003`.

### Expected automatic-match ordering

Matching is **nearest available lapak in the bid's category, ties broken by
higher rating**. Against the fixture above:

| Rank | Lapak | Distance | Rating | Outcome |
|---|---|---|---|---|
| — | agus | 0.063 km | 5.0 | **excluded** — `is_available = false` |
| 1 | **joko** | 1.240 km | 4.8 | **MATCHED** |
| 2 | sari | 7.109 km | 4.9 | loses on distance |

Two things are falsifiable here, and both are the point of the fixture:

- **Sari must lose.** She is rated higher than Joko (4.9 vs 4.8) and is still
  farther away. An implementation that sorts by rating first picks Sari, and the
  criterion fails visibly rather than silently passing.
- **Agus must lose.** He is nearest *and* best rated. The only reason he is not
  the match is `is_available = false`, so an implementation that forgets the
  availability filter picks Agus and fails visibly too.

The rating tie-break itself is never exercised by these three (no two share a
distance) — it is specified, implemented and left untested by the seed. Add a
fourth lapak at Joko's exact coordinate if that ever needs covering.

## Bid categories and lapak coverage

| Category | Slug | Id | Lapaks in it |
|---|---|---|---|
| Bersih-bersih rumah | `cleaning` | `51000000-…-0001` | joko, sari, agus |
| Taman & kebun | `gardening` | `51000000-…-0002` | joko, sari, agus |
| Pindahan & angkut barang | `moving` | `51000000-…-0003` | **none — on purpose** |

All three lapaks cover both `cleaning` and `gardening` because automatic
matching filters by category *before* it measures distance. If each lapak had a
category of its own, an automatic bid would have exactly one candidate and the
ordering above would have nothing to compare. Either category exercises the full
ordering.

`moving` has no lapak deliberately: it is the **`no_match` fixture**. An
automatic bid there must come back `status: no_match` with the 2500 fee
refunded in the same transaction. Without an empty category that path is
untestable without editing seed data mid-run. Do not add a lapak to `moving`.

Note that `is_available` does **not** gate manual bids: Agus is expected to place
offers on manual bids like anyone else.

## Platform config

`GET /market/v1/config` reads these rows from `market.config`. They are rows,
not constants, so QA can change one and reload to prove the UI renders fees from
the API instead of hard-coding them.

| Key | Seeded value | Meaning |
|---|---|---|
| `bid_auto_fee_idr` | `2500` | charged to the customer **before** automatic matching runs; refunded in the same transaction on `no_match` |
| `bid_manual_fee_idr` | `10000` | charged when the customer **awards** a manual bid; posting one is free |
| `order_auto_confirm_seconds` | `60` | window between a lapak marking work done and the sweeper auto-confirming |

**60 seconds is deliberate.** The product-facing value is 1×24h; sprint 1 seeds
60 so QA can watch auto-confirm happen instead of waiting a day. Do not
"correct" it to 86400.

## Catalog

### Products (flow A)

| Id | Lapak | Title | Price |
|---|---|---|---|
| `52000000-…-0001` | joko | Kipas angin rakitan ulang | 450 000 |
| `52000000-…-0002` | sari | Paket alat kebersihan lengkap | 350 000 |
| `52000000-…-0003` | agus | Meja kayu jati custom | 1 500 000 |

All three sit under Budi's 5 000 000, so the buy-and-pay criterion works
whichever one QA picks. Ordering four of the 1 500 000 table exceeds the balance,
which is how QA reaches the `402` insufficient-balance path without editing seed
data.

Three rather than the required two: the `q` filter on `/market/v1/products`
cannot be shown to filter anything against a single-row table.

### Gigs and tiers (flow B)

**Joko — Servis kulkas & freezer** (`53000000-…-0001`). This is the upsell
fixture: buy Konsultasi, chat, add Perbaikan besar to the *same* order, pay
again — one order id, two payment rows.

| Tier id | Name | Price |
|---|---|---|
| `54000000-…-0001` | Konsultasi | **10 000** |
| `54000000-…-0002` | Perbaikan ringan | **20 000** |
| `54000000-…-0003` | Perbaikan besar | **150 000** |

Those three prices are fixed by the contract. Do not round them.

**Sari — Bersih rumah harian** (`53000000-…-0002`), tiers Paket 2 jam 75 000 and
Paket seharian 250 000. It exists so `/market/v1/gigs` and its `q` filter have
more than one row to work with; nothing asserts against it.

Tiers carry no sort column — the API orders them by `price_idr` ascending, so
price *is* the order.

## Id blocks

Fixed UUIDs throughout, so QA scripts and FE mocks can reference known ids.

| Block | Contents |
|---|---|
| `00000000-…-0003` / `-0004` | `core.roles` — `customer`, `lapak` (continues the core role block) |
| `10000000-…-0011` … `-0014` | `core.users` — budi, joko, sari, agus (continues the core user block, from -0011 to leave core room) |
| `50000000-…` | `market.lapak_profiles` |
| `51000000-…` | `market.bid_categories` |
| `52000000-…` | `market.products` |
| `53000000-…` | `market.gigs` |
| `54000000-…` | `market.gig_tiers` |
| `55000000-…` | `market.ledger_entries` |

Blocks `20000000` (companies), `30000000` and `40000000` (clients) belong to the
core seeders — do not reuse them.

## What is NOT seeded

No orders, payments, bids, offers, chat threads or messages. Every one of those
is produced by exercising the API, which is the point: `goal.md` criterion 7
asks QA to read real rows created by real calls, and a pre-seeded order would let
a broken write path look like a working one.

## Related

- Why no `market` table has a `company_id`:
  [`docs/architecture/market-tenancy-deviation.md`](architecture/market-tenancy-deviation.md)
- Schema: `internal/database/migrations/core/000015_market_schema.up.sql`
