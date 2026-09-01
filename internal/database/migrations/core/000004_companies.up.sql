-- =====================================================
-- CORE2 SCHEMA - COMPANIES (HIERARCHICAL)
-- =====================================================
-- Migration 004: Companies with Parent-Child Structure
-- =====================================================

-- Company type enum
CREATE TYPE core.company_type AS ENUM (
    'holding',      -- Top-level parent company
    'subsidiary'    -- Child company (owned by holding)
);

-- Companies table with hierarchical structure
CREATE TABLE IF NOT EXISTS core.companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Hierarchy
    parent_id UUID REFERENCES core.companies(id) ON DELETE RESTRICT,

    -- Identity
    name VARCHAR(255) NOT NULL,
    type core.company_type DEFAULT 'subsidiary',

    -- Details
    logo_url VARCHAR(500),

    -- Ownership
    owner_id UUID NOT NULL REFERENCES core.users(id),

    -- Display
    sort INT DEFAULT 0,

    -- Status
    is_active BOOLEAN DEFAULT TRUE,

    -- Audit
    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    deleted_by UUID
);

-- Performance indexes
CREATE INDEX idx_companies_parent_id ON core.companies(parent_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_companies_type ON core.companies(type) WHERE deleted_at IS NULL;
CREATE INDEX idx_companies_owner_id ON core.companies(owner_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_companies_is_active ON core.companies(is_active) WHERE deleted_at IS NULL;
CREATE INDEX idx_companies_sort ON core.companies(sort) WHERE deleted_at IS NULL;


-- Company users junction table (user membership in companies)
CREATE TABLE IF NOT EXISTS core.company_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Relations
    company_id UUID NOT NULL REFERENCES core.companies(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,
    role_id UUID REFERENCES core.roles(id) ON DELETE SET NULL,

    -- Status
    is_primary BOOLEAN DEFAULT FALSE,  -- User's primary/default company
    is_active BOOLEAN DEFAULT TRUE,

    -- Metadata
    invited_by UUID REFERENCES core.users(id),
    joined_at TIMESTAMPTZ DEFAULT NOW(),

    -- Audit
    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    deleted_by UUID
);

-- Unique constraint: one active membership per user per company
CREATE UNIQUE INDEX company_users_company_user_active_key
    ON core.company_users(company_id, user_id) WHERE deleted_at IS NULL;

-- Ensure only one primary company per user
CREATE UNIQUE INDEX company_users_user_primary_key
    ON core.company_users(user_id) WHERE is_primary = TRUE AND deleted_at IS NULL;

-- Performance indexes
CREATE INDEX idx_company_users_company_id ON core.company_users(company_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_company_users_user_id ON core.company_users(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_company_users_role_id ON core.company_users(role_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_company_users_is_primary ON core.company_users(is_primary) WHERE deleted_at IS NULL;
CREATE INDEX idx_company_users_is_active ON core.company_users(is_active) WHERE deleted_at IS NULL;


-- =====================================================
-- ADD FOREIGN KEY TO ROLES (after companies exists)
-- =====================================================
ALTER TABLE core.roles
    ADD CONSTRAINT fk_roles_company_id
    FOREIGN KEY (company_id) REFERENCES core.companies(id) ON DELETE CASCADE;

ALTER TABLE core.user_roles
    ADD CONSTRAINT fk_user_roles_company_id
    FOREIGN KEY (company_id) REFERENCES core.companies(id) ON DELETE CASCADE;


-- =====================================================
-- COMMENTS
-- =====================================================
COMMENT ON TABLE core.companies IS 'Hierarchical company structure';
COMMENT ON COLUMN core.companies.parent_id IS 'Reference to parent company (NULL for root/holding)';
COMMENT ON COLUMN core.companies.type IS 'Company type: holding, subsidiary';

COMMENT ON TABLE core.company_users IS 'User membership in companies (many-to-many)';
COMMENT ON COLUMN core.company_users.is_primary IS 'Marks user''s default company (only one per user)';
COMMENT ON COLUMN core.company_users.role_id IS 'User''s role within this specific company';


-- =====================================================
-- HELPER FUNCTIONS
-- =====================================================

-- Get all ancestors of a company (using recursive CTE)
CREATE OR REPLACE FUNCTION core.get_company_ancestors(p_company_id UUID)
RETURNS TABLE (
    id UUID,
    name VARCHAR,
    level INT
) AS $$
BEGIN
    RETURN QUERY
    WITH RECURSIVE ancestors AS (
        SELECT c.id, c.name, c.parent_id, 0 AS lvl
        FROM core.companies c
        WHERE c.id = (SELECT parent_id FROM core.companies WHERE id = p_company_id)
          AND c.deleted_at IS NULL
        UNION ALL
        SELECT c.id, c.name, c.parent_id, a.lvl + 1
        FROM core.companies c
        JOIN ancestors a ON c.id = a.parent_id
        WHERE c.deleted_at IS NULL
    )
    SELECT ancestors.id, ancestors.name, ancestors.lvl
    FROM ancestors
    ORDER BY ancestors.lvl DESC;
END;
$$ LANGUAGE plpgsql STABLE;

-- Get all descendants of a company (using recursive CTE)
CREATE OR REPLACE FUNCTION core.get_company_descendants(p_company_id UUID)
RETURNS TABLE (
    id UUID,
    name VARCHAR,
    level INT
) AS $$
BEGIN
    RETURN QUERY
    WITH RECURSIVE descendants AS (
        SELECT c.id, c.name, c.parent_id, 1 AS lvl
        FROM core.companies c
        WHERE c.parent_id = p_company_id
          AND c.deleted_at IS NULL
        UNION ALL
        SELECT c.id, c.name, c.parent_id, d.lvl + 1
        FROM core.companies c
        JOIN descendants d ON c.parent_id = d.id
        WHERE c.deleted_at IS NULL
    )
    SELECT descendants.id, descendants.name, descendants.lvl
    FROM descendants
    ORDER BY descendants.lvl, descendants.name;
END;
$$ LANGUAGE plpgsql STABLE;

-- Get user's companies with roles
CREATE OR REPLACE FUNCTION core.get_user_companies(p_user_id UUID)
RETURNS TABLE (
    company_id UUID,
    company_name VARCHAR,
    role_code VARCHAR,
    role_name VARCHAR,
    is_primary BOOLEAN
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        c.id,
        c.name,
        r.code,
        r.name,
        cu.is_primary
    FROM core.company_users cu
    JOIN core.companies c ON c.id = cu.company_id
    LEFT JOIN core.roles r ON r.id = cu.role_id
    WHERE cu.user_id = p_user_id
      AND cu.is_active = TRUE
      AND cu.deleted_at IS NULL
      AND c.deleted_at IS NULL
    ORDER BY cu.is_primary DESC, c.name;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION core.get_company_ancestors IS 'Get all parent companies up to root';
COMMENT ON FUNCTION core.get_company_descendants IS 'Get all child companies recursively';
COMMENT ON FUNCTION core.get_user_companies IS 'Get all companies a user belongs to with their roles';
