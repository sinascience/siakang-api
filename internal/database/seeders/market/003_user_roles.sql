-- =====================================================
-- MARKET SEEDER - PERSONA ROLE ASSIGNMENTS
-- =====================================================
-- Seeder 003: assign the personas GLOBALLY (company_id NULL).
--
-- company_id NULL is load-bearing, not a placeholder: it is what makes
-- GetUserRoles(userID, nil) return the persona for a user who belongs to no
-- company, which is every marketplace user.
-- =====================================================

INSERT INTO core.user_roles (user_id, role_id, company_id) VALUES
-- Budi -> customer
(
    '10000000-0000-0000-0000-000000000011',
    '00000000-0000-0000-0000-000000000003',
    NULL
),
-- Joko -> lapak
(
    '10000000-0000-0000-0000-000000000012',
    '00000000-0000-0000-0000-000000000004',
    NULL
),
-- Sari -> lapak
(
    '10000000-0000-0000-0000-000000000013',
    '00000000-0000-0000-0000-000000000004',
    NULL
),
-- Agus -> lapak
(
    '10000000-0000-0000-0000-000000000014',
    '00000000-0000-0000-0000-000000000004',
    NULL
)
ON CONFLICT DO NOTHING;
