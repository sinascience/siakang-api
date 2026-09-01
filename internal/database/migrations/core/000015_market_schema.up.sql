-- =====================================================
-- MARKET SCHEMA - SIAKANG MARKETPLACE
-- =====================================================
-- Migration 015: the whole `market` schema in one pair.
--
-- SIAKANG connects offline manual workers ("lapak") with customers.
-- Three flows all end in one tracked order:
--   A  product      - pay the full price upfront
--   B  gig + tiers  - pay a tier, chat, add a second tier to the SAME
--                     order, pay again (one order id, TWO payment rows)
--   C  bid          - auto (nearest available lapak, fee charged before
--                     matching) or manual (offers, fee charged on award)
--
-- Design notes
-- ------------
-- TENANCY - this schema deliberately breaks the repository's usual rule.
--   CLAUDE.md says multi-tenant tables carry company_id and every query
--   filters by it. NO table here has a company_id. A marketplace is
--   single-tenant by nature; company-scoping it would force every customer
--   to own a company just to browse. Authorization is ownership-based,
--   enforced in repository WHERE clauses. Product ruling 2026-09-02; see
--   docs/architecture/market-tenancy-deviation.md.
--
-- MONEY - every amount is an integer number of rupiah in a BIGINT. IDR has
--   no minor unit, so there is nothing for a NUMERIC scale to hold, and a
--   float would be a money bug waiting to happen. Column names carry the
--   unit (*_idr) so a unitless amount cannot sneak in.
--
-- DERIVED, NOT STORED - anything the API can compute from rows it already
--   has is left out on purpose, because a stored copy is a copy that can
--   drift:
--     order.total_idr / paid_idr / outstanding_idr  -> SUM over order_items
--     order.chat_thread_id                          -> chat_threads.order_id
--     bid.order_id                                  -> orders.bid_id
--     bid.offer_count / accepted_offer_id           -> bid_offers
--     bid.off_platform_risk                         -> mode manual AND status open
--     chat_thread.customer / lapak                  -> the thread's order
--   The one money value that IS stored is bids.fee_paid_idr, a read
--   snapshot; ledger_entries remains its source of truth.
--
-- APPEND-ONLY - ledger_entries and payments have no updated_at and no soft
--   delete. A ledger you can edit is not a ledger.
-- =====================================================

CREATE SCHEMA IF NOT EXISTS market;

-- =====================================================
-- PLATFORM CONFIG
-- =====================================================
-- Key/value rather than a one-row table: QA changes a fee to prove the UI
-- renders it from the API instead of hard-coding 2500, and adding a knob
-- later is an INSERT, not a migration. Every value in sprint 1 is an
-- integer, so `value` is BIGINT - no casting in the read path.
CREATE TABLE IF NOT EXISTS market.config (
    key         VARCHAR(64) PRIMARY KEY,
    value       BIGINT NOT NULL,
    description TEXT NOT NULL DEFAULT '',

    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_config_key_format CHECK (key ~ '^[a-z][a-z0-9_]*$')
);

-- =====================================================
-- LAPAK PROFILES
-- =====================================================
-- A lapak is an ordinary core.users row plus this profile. The persona
-- itself is a seeded global role assignment (core.user_roles with
-- company_id NULL), so core needs no change; core `me` performs no join
-- into this schema - GET /market/v1/me does.
--
-- lat/lng are the matching origin for automatic bids: haversine computed
-- in SQL over these columns, no maps API in sprint 1. rating is seeded and
-- read-only (there is no ratings write path) and only breaks distance ties.
CREATE TABLE IF NOT EXISTS market.lapak_profiles (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,

    name         VARCHAR(255) NOT NULL,
    description  TEXT NOT NULL DEFAULT '',

    lat          DOUBLE PRECISION NOT NULL,
    lng          DOUBLE PRECISION NOT NULL,

    rating       DOUBLE PRECISION NOT NULL DEFAULT 0,
    is_available BOOLEAN NOT NULL DEFAULT TRUE,

    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by   UUID,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by   UUID,
    deleted_at   TIMESTAMPTZ,
    deleted_by   UUID,

    CONSTRAINT chk_lapak_profiles_rating CHECK (rating >= 0 AND rating <= 5),
    CONSTRAINT chk_lapak_profiles_lat    CHECK (lat >= -90  AND lat <= 90),
    CONSTRAINT chk_lapak_profiles_lng    CHECK (lng >= -180 AND lng <= 180)
);

-- One profile per user among non-deleted rows (mirrors core's partial
-- unique index pattern, so a soft-deleted profile can be re-created).
CREATE UNIQUE INDEX lapak_profiles_user_active_key
    ON market.lapak_profiles (user_id) WHERE deleted_at IS NULL;

-- Automatic matching scans available lapaks only.
CREATE INDEX idx_lapak_profiles_is_available
    ON market.lapak_profiles (is_available) WHERE deleted_at IS NULL;

-- =====================================================
-- WALLET + LEDGER (simulated payments)
-- =====================================================
-- There is no payment gateway in sprint 1. Every charge, fee, refund and
-- payout writes an append-only ledger row AND moves the balance, in one
-- database transaction. user_id is the primary key: a user has exactly one
-- wallet, so a surrogate id would only add a second way to name it.
CREATE TABLE IF NOT EXISTS market.wallets (
    user_id     UUID PRIMARY KEY REFERENCES core.users(id) ON DELETE CASCADE,
    balance_idr BIGINT NOT NULL DEFAULT 0,

    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- The wallet cannot go negative. Insufficient balance is a 402 at the
    -- API; this is the backstop that makes an overdraft impossible even if
    -- a service forgets to check.
    CONSTRAINT chk_wallets_balance_non_negative CHECK (balance_idr >= 0)
);

CREATE TABLE IF NOT EXISTS market.ledger_entries (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,

    type              VARCHAR(20) NOT NULL,

    -- SIGNED. Negative = money left this wallet, positive = money arrived.
    amount_idr        BIGINT NOT NULL,
    balance_after_idr BIGINT NOT NULL,

    -- What the movement was about. Nullable because a topup is about
    -- neither. FKs are attached at the end of this migration, once orders
    -- and bids exist; ON DELETE SET NULL so a ledger row outlives its
    -- subject.
    order_id          UUID,
    bid_id            UUID,

    note              TEXT NOT NULL DEFAULT '',

    -- Append-only: no updated_at, no deleted_at, by design.
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_ledger_entries_type
        CHECK (type IN ('topup', 'order_payment', 'platform_fee', 'payout', 'refund')),
    CONSTRAINT chk_ledger_entries_amount_non_zero CHECK (amount_idr <> 0),
    CONSTRAINT chk_ledger_entries_balance_non_negative CHECK (balance_after_idr >= 0)
);

-- The ledger endpoint is "my entries, newest first".
CREATE INDEX idx_ledger_entries_user_created
    ON market.ledger_entries (user_id, created_at DESC);
CREATE INDEX idx_ledger_entries_order_id ON market.ledger_entries (order_id);
CREATE INDEX idx_ledger_entries_bid_id   ON market.ledger_entries (bid_id);

-- =====================================================
-- CATALOG - products (flow A) and gigs + tiers (flow B)
-- =====================================================
CREATE TABLE IF NOT EXISTS market.products (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lapak_id    UUID NOT NULL REFERENCES market.lapak_profiles(id) ON DELETE CASCADE,

    title       VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price_idr   BIGINT NOT NULL,
    image_url   VARCHAR(500) NOT NULL DEFAULT '',

    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by  UUID,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by  UUID,
    deleted_at  TIMESTAMPTZ,
    deleted_by  UUID,

    CONSTRAINT chk_products_price_positive CHECK (price_idr > 0)
);

CREATE INDEX idx_products_lapak_id
    ON market.products (lapak_id) WHERE deleted_at IS NULL;
-- ponytail: the `q` filter is a plain ILIKE, which at seed scale is a
-- sequential scan over a handful of rows. Add pg_trgm + a GIN index when
-- the catalog is big enough for it to show up in a query plan.

CREATE TABLE IF NOT EXISTS market.gigs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lapak_id    UUID NOT NULL REFERENCES market.lapak_profiles(id) ON DELETE CASCADE,

    title       VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    image_url   VARCHAR(500) NOT NULL DEFAULT '',

    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by  UUID,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by  UUID,
    deleted_at  TIMESTAMPTZ,
    deleted_by  UUID
);

CREATE INDEX idx_gigs_lapak_id
    ON market.gigs (lapak_id) WHERE deleted_at IS NULL;

-- Tiers are the upsell ladder: consultation 10000 / small fix 20000 / big
-- fix 150000. There is no sort column - the contract orders tiers by price
-- ascending, so price IS the order.
CREATE TABLE IF NOT EXISTS market.gig_tiers (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gig_id      UUID NOT NULL REFERENCES market.gigs(id) ON DELETE CASCADE,

    name        VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price_idr   BIGINT NOT NULL,

    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by  UUID,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by  UUID,
    deleted_at  TIMESTAMPTZ,
    deleted_by  UUID,

    CONSTRAINT chk_gig_tiers_price_positive CHECK (price_idr > 0)
);

CREATE INDEX idx_gig_tiers_gig_price
    ON market.gig_tiers (gig_id, price_idr) WHERE deleted_at IS NULL;

-- =====================================================
-- BIDS (flow C) - categories, bids, offers
-- =====================================================
CREATE TABLE IF NOT EXISTS market.bid_categories (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(100) NOT NULL,
    slug       VARCHAR(50) NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT chk_bid_categories_slug_format
        CHECK (slug ~ '^[a-z0-9][a-z0-9-]*$')
);

CREATE UNIQUE INDEX bid_categories_slug_active_key
    ON market.bid_categories (slug) WHERE deleted_at IS NULL;

-- Which categories a lapak works in. Automatic matching filters candidates
-- by category BEFORE it measures distance, so this link is what makes
-- "nearest available lapak in this category" answerable at all.
-- Many-to-many because a manual worker really does do more than one kind of
-- job, and a single category column would force the seed fixture to choose
-- between being plausible and being testable.
CREATE TABLE IF NOT EXISTS market.lapak_categories (
    lapak_id    UUID NOT NULL REFERENCES market.lapak_profiles(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES market.bid_categories(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (lapak_id, category_id)
);

CREATE INDEX idx_lapak_categories_category_id
    ON market.lapak_categories (category_id);

-- Two modes in one table, because they produce the same thing - a tracked
-- order - and differ only in how the counterparty is chosen and when the
-- platform fee lands.
--
--   auto:   proposed -> customer_confirmed -> accepted, or no_match.
--           Fee charged AT CREATION and refunded in the same transaction on
--           no_match (a fee for a match that never happened is a money bug,
--           not a corner case).
--   manual: open -> awarded. Free to post, fee charged AT AWARD.
--   either: -> cancelled
CREATE TABLE IF NOT EXISTS market.bids (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    mode                VARCHAR(10) NOT NULL,
    status              VARCHAR(20) NOT NULL,

    category_id         UUID NOT NULL REFERENCES market.bid_categories(id) ON DELETE RESTRICT,
    customer_user_id    UUID NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,

    title               VARCHAR(200) NOT NULL DEFAULT '',
    description         TEXT NOT NULL DEFAULT '',
    budget_idr          BIGINT NOT NULL,

    -- Matching origin. Required for auto, meaningless for manual.
    lat                 DOUBLE PRECISION,
    lng                 DOUBLE PRECISION,

    -- Platform fee charged so far. Read snapshot; ledger_entries carrying
    -- this bid_id is the source of truth.
    fee_paid_idr        BIGINT NOT NULL DEFAULT 0,

    matched_lapak_id    UUID REFERENCES market.lapak_profiles(id) ON DELETE SET NULL,
    matched_distance_km DOUBLE PRECISION,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_bids_mode CHECK (mode IN ('auto', 'manual')),
    CONSTRAINT chk_bids_status
        CHECK (status IN ('proposed', 'customer_confirmed', 'accepted',
                          'no_match', 'open', 'awarded', 'cancelled')),

    -- Each mode owns its own half of the status vocabulary. Without this a
    -- manual bid could sit in 'proposed' and no reader would notice.
    CONSTRAINT chk_bids_status_matches_mode CHECK (
        (mode = 'auto'   AND status IN ('proposed', 'customer_confirmed', 'accepted', 'no_match', 'cancelled'))
     OR (mode = 'manual' AND status IN ('open', 'awarded', 'cancelled'))
    ),

    -- An automatic bid with no origin cannot be matched.
    CONSTRAINT chk_bids_auto_has_origin
        CHECK (mode <> 'auto' OR (lat IS NOT NULL AND lng IS NOT NULL)),

    CONSTRAINT chk_bids_budget_positive CHECK (budget_idr > 0),
    CONSTRAINT chk_bids_fee_non_negative CHECK (fee_paid_idr >= 0)
);

CREATE INDEX idx_bids_customer_created
    ON market.bids (customer_user_id, created_at DESC);
CREATE INDEX idx_bids_category_id   ON market.bids (category_id);
CREATE INDEX idx_bids_matched_lapak ON market.bids (matched_lapak_id);
-- Lapaks browse open manual bids; that list is the whole of their flow-C
-- inbox, so it gets its own partial index.
CREATE INDEX idx_bids_open_manual
    ON market.bids (created_at DESC) WHERE mode = 'manual' AND status = 'open';

CREATE TABLE IF NOT EXISTS market.bid_offers (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bid_id     UUID NOT NULL REFERENCES market.bids(id) ON DELETE CASCADE,
    lapak_id   UUID NOT NULL REFERENCES market.lapak_profiles(id) ON DELETE CASCADE,

    amount_idr BIGINT NOT NULL,
    message    TEXT NOT NULL DEFAULT '',
    status     VARCHAR(20) NOT NULL DEFAULT 'pending',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_bid_offers_status CHECK (status IN ('pending', 'awarded', 'rejected')),
    CONSTRAINT chk_bid_offers_amount_positive CHECK (amount_idr > 0),

    -- One offer per lapak per bid. Posting again REPLACES amount and
    -- message (an UPSERT on this constraint); it does not stack.
    CONSTRAINT bid_offers_bid_lapak_key UNIQUE (bid_id, lapak_id)
);

-- Offers are listed cheapest first.
CREATE INDEX idx_bid_offers_bid_amount ON market.bid_offers (bid_id, amount_idr);
CREATE INDEX idx_bid_offers_lapak_id   ON market.bid_offers (lapak_id);

-- A bid is awarded to exactly one offer, enforced here rather than in a
-- service that could be called twice.
CREATE UNIQUE INDEX bid_offers_one_awarded_per_bid
    ON market.bid_offers (bid_id) WHERE status = 'awarded';

-- =====================================================
-- ORDERS - one model for all three flows
-- =====================================================
-- total_idr / paid_idr / outstanding_idr are NOT columns: they are SUMs
-- over order_items, and storing them would be three chances to drift from
-- the items that define them.
--
-- bid_id lives here rather than order_id living on bids, so the pair needs
-- one FK instead of two pointing at each other. Bid.order_id in the API is
-- the reverse lookup.
CREATE TABLE IF NOT EXISTS market.orders (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    source              VARCHAR(20) NOT NULL,
    status              VARCHAR(30) NOT NULL DEFAULT 'pending_payment',

    customer_user_id    UUID NOT NULL REFERENCES core.users(id) ON DELETE RESTRICT,
    lapak_id            UUID NOT NULL REFERENCES market.lapak_profiles(id) ON DELETE RESTRICT,

    bid_id              UUID REFERENCES market.bids(id) ON DELETE SET NULL,

    -- Informational only in sprint 1 - no delivery integration exists.
    delivery_status     VARCHAR(20) NOT NULL DEFAULT 'none',

    -- Set by /complete to now() + config.order_auto_confirm_seconds. The
    -- sweeper completes the order when it passes; the customer confirming
    -- first does exactly the same thing.
    confirm_deadline_at TIMESTAMPTZ,
    auto_confirmed      BOOLEAN NOT NULL DEFAULT FALSE,
    completed_at        TIMESTAMPTZ,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_orders_source CHECK (source IN ('product', 'gig', 'bid_auto', 'bid_manual')),
    CONSTRAINT chk_orders_status
        CHECK (status IN ('pending_payment', 'paid', 'awaiting_confirmation', 'completed', 'cancelled')),
    CONSTRAINT chk_orders_delivery_status
        CHECK (delivery_status IN ('none', 'preparing', 'shipped', 'delivered')),

    -- A bid-sourced order came from a bid, and no other order did.
    CONSTRAINT chk_orders_bid_source
        CHECK ((source IN ('bid_auto', 'bid_manual')) = (bid_id IS NOT NULL)),

    CONSTRAINT chk_orders_completed_at
        CHECK (status <> 'completed' OR completed_at IS NOT NULL),
    -- auto_confirmed is a claim about HOW the order completed, so it cannot
    -- be true on an order that has not completed.
    CONSTRAINT chk_orders_auto_confirmed
        CHECK (auto_confirmed = FALSE OR status = 'completed')
);

-- One order per bid.
CREATE UNIQUE INDEX orders_bid_id_key
    ON market.orders (bid_id) WHERE bid_id IS NOT NULL;

CREATE INDEX idx_orders_customer_created ON market.orders (customer_user_id, created_at DESC);
CREATE INDEX idx_orders_lapak_created    ON market.orders (lapak_id, created_at DESC);
CREATE INDEX idx_orders_status           ON market.orders (status);

-- The auto-confirm sweeper's only query: overdue orders awaiting
-- confirmation. Partial, so it stays small however many orders exist.
CREATE INDEX idx_orders_confirm_deadline
    ON market.orders (confirm_deadline_at)
    WHERE status = 'awaiting_confirmation';

-- One row per charge. An upsold flow-B order has TWO rows against the same
-- order id - that is the point of goal.md criterion 3, not a side effect.
-- Append-only, like the ledger.
CREATE TABLE IF NOT EXISTS market.payments (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id   UUID NOT NULL REFERENCES market.orders(id) ON DELETE CASCADE,

    amount_idr BIGINT NOT NULL,
    paid_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_payments_amount_positive CHECK (amount_idr > 0)
);

CREATE INDEX idx_payments_order_paid_at ON market.payments (order_id, paid_at);

-- `name` and `unit_price_idr` are SNAPSHOTS taken at order time, so editing
-- the catalog later never rewrites order history.
--
-- payment_id is how an item knows which charge covered it; the contract's
-- Payment.order_item_ids is this same relation read from the payment side.
-- It replaces what would otherwise be a UUID[] column or a join table, and
-- unlike both it has real referential integrity.
--
-- product_id and gig_tier_id are both NULL on a bid-produced order: its
-- single item is priced from the bid and no catalog row backs it.
CREATE TABLE IF NOT EXISTS market.order_items (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id       UUID NOT NULL REFERENCES market.orders(id) ON DELETE CASCADE,

    product_id     UUID REFERENCES market.products(id) ON DELETE SET NULL,
    gig_tier_id    UUID REFERENCES market.gig_tiers(id) ON DELETE SET NULL,

    name           VARCHAR(255) NOT NULL,
    unit_price_idr BIGINT NOT NULL,
    quantity       INT NOT NULL DEFAULT 1,

    -- Generated, because a subtotal that disagrees with its own price and
    -- quantity is not a number anyone should have to reconcile.
    subtotal_idr   BIGINT GENERATED ALWAYS AS (unit_price_idr * quantity) STORED,

    status         VARCHAR(10) NOT NULL DEFAULT 'unpaid',
    payment_id     UUID REFERENCES market.payments(id) ON DELETE SET NULL,

    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_order_items_status CHECK (status IN ('unpaid', 'paid')),
    CONSTRAINT chk_order_items_price_positive CHECK (unit_price_idr > 0),
    CONSTRAINT chk_order_items_quantity_positive CHECK (quantity >= 1),

    -- An item is a product OR a gig tier OR (on a bid order) neither.
    CONSTRAINT chk_order_items_one_source
        CHECK (product_id IS NULL OR gig_tier_id IS NULL),

    -- "paid" and "has a payment" are the same fact, so they cannot disagree.
    CONSTRAINT chk_order_items_paid_has_payment
        CHECK ((status = 'paid') = (payment_id IS NOT NULL))
);

CREATE INDEX idx_order_items_order_id    ON market.order_items (order_id);
CREATE INDEX idx_order_items_payment_id  ON market.order_items (payment_id);
CREATE INDEX idx_order_items_product_id  ON market.order_items (product_id);
CREATE INDEX idx_order_items_gig_tier_id ON market.order_items (gig_tier_id);
-- What /pay reads: this order's unpaid items.
CREATE INDEX idx_order_items_unpaid
    ON market.order_items (order_id) WHERE status = 'unpaid';

-- =====================================================
-- CHAT
-- =====================================================
-- Threads are created server-side only: on first payment of a gig order,
-- and when a bid produces a tracked order. There is no client-side create
-- endpoint, and exactly one thread exists per order - hence the UNIQUE on
-- order_id, and no customer/lapak columns, which the order already knows.
CREATE TABLE IF NOT EXISTS market.chat_threads (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id   UUID NOT NULL UNIQUE REFERENCES market.orders(id) ON DELETE CASCADE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS market.chat_messages (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    thread_id      UUID NOT NULL REFERENCES market.chat_threads(id) ON DELETE CASCADE,
    sender_user_id UUID NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,

    body           TEXT NOT NULL,

    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_chat_messages_body_not_blank CHECK (LENGTH(TRIM(body)) > 0)
);

-- Both the history page and the SSE tail read a thread in time order.
CREATE INDEX idx_chat_messages_thread_created
    ON market.chat_messages (thread_id, created_at);

-- =====================================================
-- LATE FOREIGN KEYS
-- =====================================================
-- ledger_entries is defined before orders and bids exist (nothing
-- references it, but it references both), so its FKs are attached here
-- rather than reordering the file around a table that is read far more
-- often than it is joined.
ALTER TABLE market.ledger_entries
    ADD CONSTRAINT fk_ledger_entries_order
        FOREIGN KEY (order_id) REFERENCES market.orders(id) ON DELETE SET NULL,
    ADD CONSTRAINT fk_ledger_entries_bid
        FOREIGN KEY (bid_id) REFERENCES market.bids(id) ON DELETE SET NULL;

-- =====================================================
-- COMMENTS
-- =====================================================
COMMENT ON SCHEMA market IS
    'SIAKANG marketplace. Deliberately NOT company-scoped: no table here has a company_id, /market/v1/* runs JWTAuth() only, and authorization is ownership-based in repository WHERE clauses. Product ruling 2026-09-02; see docs/architecture/market-tenancy-deviation.md.';

COMMENT ON TABLE market.config IS
    'Platform settings as key/value rows: bid_auto_fee_idr, bid_manual_fee_idr, order_auto_confirm_seconds. Read-only in sprint 1 (no admin UI); QA edits the rows to prove the UI renders fees from the API rather than hard-coding them.';
COMMENT ON TABLE market.lapak_profiles IS
    'Worker profile attached to a core.users row. lat/lng are the haversine matching origin; rating is seeded, read-only, and only breaks distance ties.';
COMMENT ON COLUMN market.lapak_profiles.is_available IS
    'Automatic matching considers available lapaks only. Placing offers on manual bids is NOT gated by this flag.';
COMMENT ON TABLE market.wallets IS
    'Simulated wallet. The balance moves only alongside a ledger_entries row, in the same transaction.';
COMMENT ON TABLE market.ledger_entries IS
    'Append-only money journal. amount_idr is signed: negative means money left this wallet. No updates, no soft delete.';
COMMENT ON TABLE market.gig_tiers IS
    'Gig price ladder. Tiers are ordered by price ascending - price is the sort, there is no sort column.';
COMMENT ON TABLE market.lapak_categories IS
    'Which bid categories a lapak works in. Automatic matching filters candidates by category before measuring distance.';
COMMENT ON TABLE market.bids IS
    'Flow C. auto: fee charged before matching, refunded on no_match. manual: free to post, fee charged on award. Both end in a tracked order.';
COMMENT ON COLUMN market.bids.fee_paid_idr IS
    'Platform fee charged so far on this bid. Read snapshot; ledger_entries carrying this bid_id is the source of truth.';
COMMENT ON TABLE market.bid_offers IS
    'One offer per lapak per bid - posting again replaces amount and message (UPSERT on bid_offers_bid_lapak_key).';
COMMENT ON TABLE market.orders IS
    'One order model for all three flows. total_idr, paid_idr and outstanding_idr are computed from order_items and deliberately not stored.';
COMMENT ON COLUMN market.orders.confirm_deadline_at IS
    'Set by /complete to now() + config.order_auto_confirm_seconds. Cleared when the order completes.';
COMMENT ON COLUMN market.orders.auto_confirmed IS
    'TRUE when the sweeper completed the order rather than the customer, so the UI can say so honestly.';
COMMENT ON TABLE market.payments IS
    'One row per charge. An upsold flow-B order has two rows against a single order id. Append-only.';
COMMENT ON COLUMN market.order_items.name IS
    'Snapshot of the catalog name at order time, so later catalog edits never rewrite order history.';
COMMENT ON COLUMN market.order_items.payment_id IS
    'The charge that covered this item. Payment.order_item_ids in the API is this relation read from the payment side.';
COMMENT ON TABLE market.chat_threads IS
    'Exactly one thread per order, created server-side on first gig payment or when a bid produces an order. Customer and lapak come from the order.';
