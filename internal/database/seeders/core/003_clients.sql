-- =====================================================
-- CORE SEEDER - CLIENTS
-- =====================================================
-- Seeder 003: Registration-level tenant rows (core.clients).
--
-- A "client" is the tenant that owns one or more companies (see
-- migration 000012_clients). Every company carries a NOT NULL client_id,
-- so clients MUST be seeded before companies (004).
--
-- One client per seeded user:
--   - Owner  (User 1) → client "owner"
--   - Client (User 2) → client "client"
--
-- slug must satisfy ^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$ (3–63 chars).
-- =====================================================

INSERT INTO core.clients (id, slug, name, owner_user_id, is_active, created_by, updated_by) VALUES
-- Client of Owner (User 1)
(
    '40000000-0000-0000-0000-000000000001',
    'owner',
    'Owner Organization',
    '10000000-0000-0000-0000-000000000001',
    TRUE,
    '10000000-0000-0000-0000-000000000001',
    '10000000-0000-0000-0000-000000000001'
),
-- Client of Client (User 2)
(
    '40000000-0000-0000-0000-000000000002',
    'client',
    'Client Organization',
    '10000000-0000-0000-0000-000000000002',
    TRUE,
    '10000000-0000-0000-0000-000000000002',
    '10000000-0000-0000-0000-000000000002'
)
ON CONFLICT DO NOTHING;
