-- =====================================================
-- MARKET SEEDER - WALLETS AND OPENING LEDGER
-- =====================================================
-- Seeder 006: one wallet per actor, plus the ledger row that explains
-- Budi's opening balance.
--
-- Balance and ledger always move together - that is the rule the whole
-- payments simulation rests on - so an opening balance that appeared with
-- no ledger row behind it would be the first thing to break it. Budi's
-- 5 000 000 gets a `topup` entry; the lapaks start at 0 with no entry
-- because nothing has moved.
--
-- 5 000 000 IDR is deliberately more than one QA run needs: every flow in
-- goal.md can be exercised back to back without a top-up endpoint, which
-- sprint 1 does not have.
-- =====================================================

INSERT INTO market.wallets (user_id, balance_idr) VALUES
-- Budi (customer) - funded.
('10000000-0000-0000-0000-000000000011', 5000000),
-- The lapaks start empty; they are credited on order completion (payout).
('10000000-0000-0000-0000-000000000012', 0),
('10000000-0000-0000-0000-000000000013', 0),
('10000000-0000-0000-0000-000000000014', 0)
ON CONFLICT DO NOTHING;

-- Budi's opening balance, as a real ledger row rather than a bare number.
INSERT INTO market.ledger_entries (id, user_id, type, amount_idr, balance_after_idr, note) VALUES
(
    '55000000-0000-0000-0000-000000000001',
    '10000000-0000-0000-0000-000000000011',
    'topup',
    5000000,
    5000000,
    'Seeded opening balance for sprint-1 QA.'
)
ON CONFLICT DO NOTHING;
