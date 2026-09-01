-- =====================================================
-- CORE SEEDER - USERS
-- =====================================================
-- Seeder 002: Default users
-- =====================================================

INSERT INTO core.users (id, email, username, password_hash, full_name, phone, is_active, is_email_verified, email_verified_at) VALUES
-- User 1: Owner
(
    '10000000-0000-0000-0000-000000000001',
    'owner@gmail.com',
    'owner',
    '$2a$10$z6Fy41kbyo2qjJY48mk63ebgxd4AUg3uX0qijSnWq5ligAF.YCW6W', -- Bismillah1407*
    'Owner',
    '+6281234567001',
    TRUE,
    TRUE,
    NOW()
),
-- User 2: Client
(
    '10000000-0000-0000-0000-000000000002',
    'client@gmail.com',
    'client',
    '$2a$10$AVzgTS9Jthz/olHx7F.64OXPKT1gR67fvS0gIVusBiQQ5LcxMMc8u', -- Client2026*
    'Client',
    '+6281234567002',
    TRUE,
    TRUE,
    NOW()
)
ON CONFLICT DO NOTHING;
