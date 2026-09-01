-- =====================================================
-- MARKET SEEDER - PRODUCTS (flow A)
-- =====================================================
-- Seeder 008: three priced products, one per lapak.
--
-- All three are under Budi's 5 000 000 balance, so criterion 2 (order a
-- product, pay in full, wallet debited) works whichever one QA picks.
--
-- Three rather than the required two: the /market/v1/products `q` filter
-- cannot be shown to filter anything against a single-row table.
-- =====================================================

INSERT INTO market.products (id, lapak_id, title, description, price_idr, image_url) VALUES
-- Joko (Servis Elektronik) - 450 000
(
    '52000000-0000-0000-0000-000000000001',
    '50000000-0000-0000-0000-000000000001',
    'Kipas angin rakitan ulang',
    'Kipas angin bekas yang direkondisi total: motor baru, kabel baru, garansi servis 3 bulan.',
    450000,
    'https://picsum.photos/seed/siakang-kipas/640/480'
),
-- Sari (Bersih Kilat) - 350 000
(
    '52000000-0000-0000-0000-000000000002',
    '50000000-0000-0000-0000-000000000002',
    'Paket alat kebersihan lengkap',
    'Sapu, pel, ember peras, cairan pembersih lantai, dan lap microfiber. Cukup untuk rumah 2 lantai.',
    350000,
    'https://picsum.photos/seed/siakang-bersih/640/480'
),
-- Agus (Tukang Kebun) - 1 500 000. The priciest, still well under Budi's
-- balance; buy two and the wallet is the binding constraint, which is how
-- QA reaches the 402 path without editing seed data.
(
    '52000000-0000-0000-0000-000000000003',
    '50000000-0000-0000-0000-000000000003',
    'Meja kayu jati custom',
    'Meja kayu jati solid, ukuran dan finishing sesuai permintaan. Pengerjaan 2-3 minggu.',
    1500000,
    'https://picsum.photos/seed/siakang-meja/640/480'
)
ON CONFLICT DO NOTHING;
