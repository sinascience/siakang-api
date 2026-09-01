-- =====================================================
-- MARKET SEEDER - GIGS AND TIERS (flow B)
-- =====================================================
-- Seeder 009: the freezer-repair gig with its three tiers, plus a second
-- gig so the gigs list and its `q` filter have something to filter.
--
-- Joko's three tiers ARE goal.md criterion 3: Budi buys Konsultasi
-- (10 000), they chat, Budi adds Perbaikan besar (150 000) to the SAME
-- order and pays again - one order id, two payment rows. The prices
-- 10000 / 20000 / 150000 are fixed by the contract; do not round them.
--
-- Tiers carry no sort column: the API orders them by price ascending.
-- =====================================================

INSERT INTO market.gigs (id, lapak_id, title, description, image_url) VALUES
-- Joko's gig - the criterion-3 fixture.
(
    '53000000-0000-0000-0000-000000000001',
    '50000000-0000-0000-0000-000000000001',
    'Servis kulkas & freezer',
    'Diagnosa dan perbaikan kulkas atau freezer di tempat. Mulai dari konsultasi sampai ganti kompresor.',
    'https://picsum.photos/seed/siakang-freezer/640/480'
),
-- Sari's gig - exists so /market/v1/gigs returns more than one row.
(
    '53000000-0000-0000-0000-000000000002',
    '50000000-0000-0000-0000-000000000002',
    'Bersih rumah harian',
    'Sapu, pel, cuci piring, dan rapikan kamar. Durasi menyesuaikan paket.',
    'https://picsum.photos/seed/siakang-rumah/640/480'
)
ON CONFLICT DO NOTHING;

INSERT INTO market.gig_tiers (id, gig_id, name, description, price_idr) VALUES
-- Joko's freezer gig: 10000 / 20000 / 150000, exactly as the contract says.
(
    '54000000-0000-0000-0000-000000000001',
    '53000000-0000-0000-0000-000000000001',
    'Konsultasi',
    'Kunjungan dan diagnosa kerusakan. Biaya ini tidak termasuk perbaikan.',
    10000
),
(
    '54000000-0000-0000-0000-000000000002',
    '53000000-0000-0000-0000-000000000001',
    'Perbaikan ringan',
    'Ganti karet pintu, thermostat, atau isi ulang freon.',
    20000
),
(
    '54000000-0000-0000-0000-000000000003',
    '53000000-0000-0000-0000-000000000001',
    'Perbaikan besar',
    'Ganti kompresor atau evaporator, termasuk suku cadang dan garansi 6 bulan.',
    150000
),
-- Sari's cleaning gig.
(
    '54000000-0000-0000-0000-000000000004',
    '53000000-0000-0000-0000-000000000002',
    'Paket 2 jam',
    'Satu ruangan utama plus dapur.',
    75000
),
(
    '54000000-0000-0000-0000-000000000005',
    '53000000-0000-0000-0000-000000000002',
    'Paket seharian',
    'Seluruh rumah, termasuk kamar mandi dan halaman depan.',
    250000
)
ON CONFLICT DO NOTHING;
