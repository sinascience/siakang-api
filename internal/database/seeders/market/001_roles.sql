-- =====================================================
-- MARKET SEEDER - PERSONA ROLES
-- =====================================================
-- Seeder 001: the two marketplace personas, as GLOBAL core roles.
--
-- Why core.roles and not a market table: FE routes the app shell off
-- `GET /core/v1/auth/me` -> data.roles, exactly as it already does for the
-- B2B core. Assigning these globally (core.user_roles.company_id IS NULL)
-- is what lets a marketplace user have no company at all - verified against
-- GetUserRoles(userID, nil), which selects exactly `ur.company_id IS NULL`.
-- So no core code changes for personas; only these rows.
--
-- permissions is '{}': /market/v1/* runs JWTAuth() only. It never calls
-- RequirePermission(), and `permissions` is [] for marketplace users.
--
-- Ids continue the core role block (00000000-...-0001 super_admin,
-- -0002 administrator).
-- =====================================================

INSERT INTO core.roles (id, code, name, description, permissions, is_system, company_id, is_active) VALUES
-- Customer: buys products, books gig tiers, posts bids.
(
    '00000000-0000-0000-0000-000000000003',
    'customer',
    'Customer',
    'SIAKANG marketplace customer. Global (company-less) persona: browses the catalog, places and pays orders, posts bids. Carries no core RBAC permissions.',
    '{}'::jsonb,
    TRUE,
    NULL,
    TRUE
),
-- Lapak: the offline manual worker selling on the platform.
(
    '00000000-0000-0000-0000-000000000004',
    'lapak',
    'Lapak',
    'SIAKANG marketplace worker ("lapak"). Global (company-less) persona: owns a market.lapak_profiles row, sells products and gigs, offers on manual bids. Carries no core RBAC permissions.',
    '{}'::jsonb,
    TRUE,
    NULL,
    TRUE
)
ON CONFLICT DO NOTHING;
