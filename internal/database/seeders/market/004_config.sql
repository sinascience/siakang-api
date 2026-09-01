-- =====================================================
-- MARKET SEEDER - PLATFORM CONFIG
-- =====================================================
-- Seeder 004: the three knobs GET /market/v1/config exposes.
--
-- These are ROWS, not constants, on purpose: QA changes a fee and reloads
-- to prove the UI renders it from the API instead of hard-coding 2500.
-- =====================================================

INSERT INTO market.config (key, value, description) VALUES
-- Charged to the customer BEFORE automatic matching starts, and refunded in
-- the same transaction when matching finds nobody (no_match).
(
    'bid_auto_fee_idr',
    2500,
    'Platform fee charged to the customer before automatic bid matching runs. Refunded in the same transaction on no_match.'
),
-- Charged when the customer awards a manual bid on-platform. Posting a
-- manual bid is free; this is what the platform earns for the match.
(
    'bid_manual_fee_idr',
    10000,
    'Platform fee charged to the customer when a manual bid is awarded on-platform. Posting a manual bid is free.'
),
-- 60 SECONDS IS DELIBERATE. The product-facing value is 1x24h; sprint 1
-- seeds 60 so QA can watch auto-confirm happen instead of waiting a day.
-- Do not "correct" this to 86400.
(
    'order_auto_confirm_seconds',
    60,
    'Seconds between a lapak marking work done and the sweeper auto-confirming the order. Seeded at 60 so sprint-1 QA can observe it; the production value is 86400.'
)
ON CONFLICT DO NOTHING;
