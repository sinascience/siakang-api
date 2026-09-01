-- =====================================================
-- CORE SEEDER - USER ROLES
-- =====================================================
-- Seeder 006: Assign administrator role to users
-- Both users get the administrator role globally
-- =====================================================

INSERT INTO core.user_roles (user_id, role_id, company_id) VALUES
-- Owner - super_admin role (global)
(
    '10000000-0000-0000-0000-000000000001',
    '00000000-0000-0000-0000-000000000001',
    NULL
),
-- Client - administrator role (global)
(
    '10000000-0000-0000-0000-000000000002',
    '00000000-0000-0000-0000-000000000002',
    NULL
)
ON CONFLICT DO NOTHING;
