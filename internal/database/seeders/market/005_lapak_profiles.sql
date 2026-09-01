-- =====================================================
-- MARKET SEEDER - LAPAK PROFILES
-- =====================================================
-- Seeder 005: the three workers.
--
-- THE GEOGRAPHY IS A TEST FIXTURE, NOT DECORATION.
-- Origin is Budi's seeded coordinate, -7.9666 / 112.6326 (Malang).
-- Haversine distance from it, and what each row proves:
--
--   Agus  0.063 km  rating 5.0  is_available FALSE  <- nearest AND best
--                                                      rated, and must
--                                                      still lose: proves
--                                                      availability is
--                                                      honoured
--   Joko  1.240 km  rating 4.8  is_available TRUE   <- EXPECTED WINNER of
--                                                      automatic matching
--   Sari  7.109 km  rating 4.9  is_available TRUE   <- rated higher than
--                                                      Joko and must still
--                                                      lose: proves rating
--                                                      only breaks ties
--
-- Change a coordinate here and you change what goal.md criterion 4 asserts.
-- The full ordering and the maths are in docs/seed-actors.md.
-- =====================================================

INSERT INTO market.lapak_profiles (id, user_id, name, description, lat, lng, rating, is_available) VALUES
-- Joko - the expected automatic match. Owns the 3-tier freezer gig.
(
    '50000000-0000-0000-0000-000000000001',
    '10000000-0000-0000-0000-000000000012',
    'Servis Elektronik Pak Joko',
    'Servis kulkas, freezer, mesin cuci, dan alat elektronik rumah tangga. Bisa panggilan ke rumah.',
    -7.9750,
    112.6400,
    4.8,
    TRUE
),
-- Sari - farther, rated higher. Loses the automatic match on distance.
(
    '50000000-0000-0000-0000-000000000002',
    '10000000-0000-0000-0000-000000000013',
    'Bersih Kilat Sari',
    'Jasa bersih-bersih rumah, kos, dan kantor. Tim berpengalaman, alat lengkap.',
    -8.0100,
    112.6800,
    4.9,
    TRUE
),
-- Agus - nearest and best rated, but UNAVAILABLE. is_available FALSE is the
-- point of this row; flipping it to TRUE breaks criterion 4.
(
    '50000000-0000-0000-0000-000000000003',
    '10000000-0000-0000-0000-000000000014',
    'Tukang Kebun Agus',
    'Perawatan taman, potong rumput, dan pertukangan kayu ringan.',
    -7.9670,
    112.6330,
    5.0,
    FALSE
)
ON CONFLICT DO NOTHING;
