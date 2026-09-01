-- =====================================================
-- MARKET SEEDER - BID CATEGORIES AND LAPAK COVERAGE
-- =====================================================
-- Seeder 007: the admin-predeclared categories (no admin UI in sprint 1)
-- and which lapak works in which.
--
-- WHY ALL THREE LAPAKS COVER BOTH cleaning AND gardening:
-- automatic matching filters candidates by category first, then measures
-- distance. If each lapak had one category, an automatic bid would have a
-- single candidate and criterion 4 - "nearest wins over better-rated, and
-- unavailable never wins" - would have nothing to compare. All three in
-- both categories means either category exercises the full ordering:
--
--   Agus 0.063 km / 5.0 / UNAVAILABLE  -> excluded
--   Joko 1.240 km / 4.8 / available    -> WINNER
--   Sari 7.109 km / 4.9 / available    -> loses on distance
--
-- `moving` has NO lapak on purpose: it is the no_match fixture. An
-- automatic bid there must come back status no_match with the 2500 fee
-- refunded in the same transaction, which is otherwise untestable without
-- editing seed data.
-- =====================================================

INSERT INTO market.bid_categories (id, name, slug) VALUES
(
    '51000000-0000-0000-0000-000000000001',
    'Bersih-bersih rumah',
    'cleaning'
),
(
    '51000000-0000-0000-0000-000000000002',
    'Taman & kebun',
    'gardening'
),
-- Deliberately empty - see the header. Do not add a lapak to this one.
(
    '51000000-0000-0000-0000-000000000003',
    'Pindahan & angkut barang',
    'moving'
)
ON CONFLICT DO NOTHING;

INSERT INTO market.lapak_categories (lapak_id, category_id) VALUES
-- Joko
('50000000-0000-0000-0000-000000000001', '51000000-0000-0000-0000-000000000001'),
('50000000-0000-0000-0000-000000000001', '51000000-0000-0000-0000-000000000002'),
-- Sari
('50000000-0000-0000-0000-000000000002', '51000000-0000-0000-0000-000000000001'),
('50000000-0000-0000-0000-000000000002', '51000000-0000-0000-0000-000000000002'),
-- Agus (unavailable, but categorised - availability is checked at match
-- time, not by leaving him out of the category)
('50000000-0000-0000-0000-000000000003', '51000000-0000-0000-0000-000000000001'),
('50000000-0000-0000-0000-000000000003', '51000000-0000-0000-0000-000000000002')
ON CONFLICT DO NOTHING;
