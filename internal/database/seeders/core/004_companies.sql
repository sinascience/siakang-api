-- =====================================================
-- CORE SEEDER - COMPANIES
-- =====================================================
-- Seeder 004: Companies with parent-child structure
--
-- Every company belongs to a client (NOT NULL client_id, migration
-- 000012). client_id is sourced from seeder 003_clients:
--   - Owner companies  → client 40000000-...-001 (slug "owner")
--   - Client companies → client 40000000-...-002 (slug "client")
--
-- Owner (User 1):
--   PT Alpha Indonesia (Holding)
--     ├── Alpha Jakarta (Subsidiary)
--     └── Alpha Surabaya (Subsidiary)
--   PT Beta Nusantara (Holding)
--     ├── Beta Bandung (Subsidiary)
--     └── Beta Semarang (Subsidiary)
--   PT Gamma Teknologi (Holding)
--     ├── Gamma Yogyakarta (Subsidiary)
--     └── Gamma Bali (Subsidiary)
--
-- Client (User 2):
--   PT Delta Digital (Holding)
--     ├── Delta Medan (Subsidiary)
--     └── Delta Makassar (Subsidiary)
--   PT Epsilon Solusi (Holding)
--     ├── Epsilon Malang (Subsidiary)
--     └── Epsilon Palembang (Subsidiary)
--   PT Zeta Global (Holding)
--     ├── Zeta Denpasar (Subsidiary)
--     └── Zeta Manado (Subsidiary)
-- =====================================================

-- =====================================================
-- OWNER COMPANIES  (client_id = 40000000-...-001)
-- =====================================================
INSERT INTO core.companies (id, parent_id, name, type, owner_id, client_id, sort, is_active) VALUES
-- Holding 1: PT Alpha Indonesia
(
    '20000000-0000-0000-0000-000000000001',
    NULL,
    'PT Alpha Indonesia',
    'holding',
    '10000000-0000-0000-0000-000000000001',
    '40000000-0000-0000-0000-000000000001',
    1,
    TRUE
),
-- Holding 2: PT Beta Nusantara
(
    '20000000-0000-0000-0000-000000000002',
    NULL,
    'PT Beta Nusantara',
    'holding',
    '10000000-0000-0000-0000-000000000001',
    '40000000-0000-0000-0000-000000000001',
    2,
    TRUE
),
-- Holding 3: PT Gamma Teknologi
(
    '20000000-0000-0000-0000-000000000003',
    NULL,
    'PT Gamma Teknologi',
    'holding',
    '10000000-0000-0000-0000-000000000001',
    '40000000-0000-0000-0000-000000000001',
    3,
    TRUE
)
ON CONFLICT DO NOTHING;

-- Subsidiaries of Owner
INSERT INTO core.companies (id, parent_id, name, type, owner_id, client_id, sort, is_active) VALUES
-- Alpha subsidiaries
(
    '20000000-0000-0000-0000-000000000011',
    '20000000-0000-0000-0000-000000000001',
    'Alpha Jakarta',
    'subsidiary',
    '10000000-0000-0000-0000-000000000001',
    '40000000-0000-0000-0000-000000000001',
    1,
    TRUE
),
(
    '20000000-0000-0000-0000-000000000012',
    '20000000-0000-0000-0000-000000000001',
    'Alpha Surabaya',
    'subsidiary',
    '10000000-0000-0000-0000-000000000001',
    '40000000-0000-0000-0000-000000000001',
    2,
    TRUE
),
-- Beta subsidiaries
(
    '20000000-0000-0000-0000-000000000021',
    '20000000-0000-0000-0000-000000000002',
    'Beta Bandung',
    'subsidiary',
    '10000000-0000-0000-0000-000000000001',
    '40000000-0000-0000-0000-000000000001',
    1,
    TRUE
),
(
    '20000000-0000-0000-0000-000000000022',
    '20000000-0000-0000-0000-000000000002',
    'Beta Semarang',
    'subsidiary',
    '10000000-0000-0000-0000-000000000001',
    '40000000-0000-0000-0000-000000000001',
    2,
    TRUE
),
-- Gamma subsidiaries
(
    '20000000-0000-0000-0000-000000000031',
    '20000000-0000-0000-0000-000000000003',
    'Gamma Yogyakarta',
    'subsidiary',
    '10000000-0000-0000-0000-000000000001',
    '40000000-0000-0000-0000-000000000001',
    1,
    TRUE
),
(
    '20000000-0000-0000-0000-000000000032',
    '20000000-0000-0000-0000-000000000003',
    'Gamma Bali',
    'subsidiary',
    '10000000-0000-0000-0000-000000000001',
    '40000000-0000-0000-0000-000000000001',
    2,
    TRUE
)
ON CONFLICT DO NOTHING;

-- =====================================================
-- CLIENT COMPANIES  (client_id = 40000000-...-002)
-- =====================================================
INSERT INTO core.companies (id, parent_id, name, type, owner_id, client_id, sort, is_active) VALUES
-- Holding 1: PT Delta Digital
(
    '20000000-0000-0000-0000-000000000004',
    NULL,
    'PT Delta Digital',
    'holding',
    '10000000-0000-0000-0000-000000000002',
    '40000000-0000-0000-0000-000000000002',
    1,
    TRUE
),
-- Holding 2: PT Epsilon Solusi
(
    '20000000-0000-0000-0000-000000000005',
    NULL,
    'PT Epsilon Solusi',
    'holding',
    '10000000-0000-0000-0000-000000000002',
    '40000000-0000-0000-0000-000000000002',
    2,
    TRUE
),
-- Holding 3: PT Zeta Global
(
    '20000000-0000-0000-0000-000000000006',
    NULL,
    'PT Zeta Global',
    'holding',
    '10000000-0000-0000-0000-000000000002',
    '40000000-0000-0000-0000-000000000002',
    3,
    TRUE
)
ON CONFLICT DO NOTHING;

-- Subsidiaries of Client
INSERT INTO core.companies (id, parent_id, name, type, owner_id, client_id, sort, is_active) VALUES
-- Delta subsidiaries
(
    '20000000-0000-0000-0000-000000000041',
    '20000000-0000-0000-0000-000000000004',
    'Delta Medan',
    'subsidiary',
    '10000000-0000-0000-0000-000000000002',
    '40000000-0000-0000-0000-000000000002',
    1,
    TRUE
),
(
    '20000000-0000-0000-0000-000000000042',
    '20000000-0000-0000-0000-000000000004',
    'Delta Makassar',
    'subsidiary',
    '10000000-0000-0000-0000-000000000002',
    '40000000-0000-0000-0000-000000000002',
    2,
    TRUE
),
-- Epsilon subsidiaries
(
    '20000000-0000-0000-0000-000000000051',
    '20000000-0000-0000-0000-000000000005',
    'Epsilon Malang',
    'subsidiary',
    '10000000-0000-0000-0000-000000000002',
    '40000000-0000-0000-0000-000000000002',
    1,
    TRUE
),
(
    '20000000-0000-0000-0000-000000000052',
    '20000000-0000-0000-0000-000000000005',
    'Epsilon Palembang',
    'subsidiary',
    '10000000-0000-0000-0000-000000000002',
    '40000000-0000-0000-0000-000000000002',
    2,
    TRUE
),
-- Zeta subsidiaries
(
    '20000000-0000-0000-0000-000000000061',
    '20000000-0000-0000-0000-000000000006',
    'Zeta Denpasar',
    'subsidiary',
    '10000000-0000-0000-0000-000000000002',
    '40000000-0000-0000-0000-000000000002',
    1,
    TRUE
),
(
    '20000000-0000-0000-0000-000000000062',
    '20000000-0000-0000-0000-000000000006',
    'Zeta Manado',
    'subsidiary',
    '10000000-0000-0000-0000-000000000002',
    '40000000-0000-0000-0000-000000000002',
    2,
    TRUE
)
ON CONFLICT DO NOTHING;
