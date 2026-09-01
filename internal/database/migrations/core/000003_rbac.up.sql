-- =====================================================
-- CORE2 SCHEMA - RBAC (SIMPLIFIED)
-- =====================================================
-- Migration 003: Roles with JSON-based Permissions
-- =====================================================

-- Roles table with JSONB permissions
CREATE TABLE IF NOT EXISTS core.roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Identity
    code VARCHAR(50) NOT NULL,  -- e.g., 'super_admin', 'admin', 'member'
    name VARCHAR(100) NOT NULL, -- e.g., 'Super Administrator'
    description TEXT,

    -- Permissions as JSONB (level-based)
    -- Format: {"resource": "level"}
    -- Levels: "viewer", "editor", "admin" (mapped to actions in backend config)
    -- Example: {"dashboard": "viewer", "user-management": "admin", "jurnal-umum": "editor"}
    permissions JSONB DEFAULT '{}',

    -- Scope
    is_system BOOLEAN DEFAULT FALSE,  -- System roles cannot be modified
    company_id UUID,                   -- NULL = global role, UUID = company-specific role

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

-- Unique constraint: code must be unique per scope (global or within company)
CREATE UNIQUE INDEX roles_code_global_key ON core.roles(code)
    WHERE company_id IS NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX roles_code_company_key ON core.roles(company_id, code)
    WHERE company_id IS NOT NULL AND deleted_at IS NULL;

-- Performance indexes
CREATE INDEX idx_roles_code ON core.roles(code) WHERE deleted_at IS NULL;
CREATE INDEX idx_roles_company_id ON core.roles(company_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_roles_is_system ON core.roles(is_system) WHERE deleted_at IS NULL;
CREATE INDEX idx_roles_is_active ON core.roles(is_active) WHERE deleted_at IS NULL;
CREATE INDEX idx_roles_permissions ON core.roles USING GIN(permissions);


-- User roles junction table
CREATE TABLE IF NOT EXISTS core.user_roles (
    user_id UUID NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES core.roles(id) ON DELETE CASCADE,

    -- Scope: which company this role assignment applies to
    -- NULL = global assignment (user has this role everywhere)
    -- UUID = role only applies within specific company
    company_id UUID,

    -- Audit
    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID,

    PRIMARY KEY (user_id, role_id, company_id)
);

-- Handle NULL company_id in primary key (use COALESCE for unique constraint)
-- Drop the primary key and create a new one without company_id
ALTER TABLE core.user_roles DROP CONSTRAINT user_roles_pkey;
ALTER TABLE core.user_roles ADD PRIMARY KEY (user_id, role_id);

-- Allow NULL on company_id (was implicitly NOT NULL when part of PK)
ALTER TABLE core.user_roles ALTER COLUMN company_id DROP NOT NULL;

-- Partial unique index to prevent duplicate role assignments per company
CREATE UNIQUE INDEX user_roles_user_role_company_key
    ON core.user_roles(user_id, role_id, company_id)
    WHERE company_id IS NOT NULL;
CREATE UNIQUE INDEX user_roles_user_role_global_key
    ON core.user_roles(user_id, role_id)
    WHERE company_id IS NULL;

-- Performance indexes
CREATE INDEX idx_user_roles_user_id ON core.user_roles(user_id);
CREATE INDEX idx_user_roles_role_id ON core.user_roles(role_id);
CREATE INDEX idx_user_roles_company_id ON core.user_roles(company_id) WHERE company_id IS NOT NULL;


-- =====================================================
-- COMMENTS
-- =====================================================
COMMENT ON TABLE core.roles IS 'Role definitions with JSON-based permissions';
COMMENT ON COLUMN core.roles.code IS 'Unique identifier for the role (snake_case)';
COMMENT ON COLUMN core.roles.permissions IS 'JSONB object mapping resources to permission levels (viewer/editor/admin)';
COMMENT ON COLUMN core.roles.is_system IS 'System roles cannot be modified or deleted';
COMMENT ON COLUMN core.roles.company_id IS 'NULL for global roles, UUID for company-specific roles';

COMMENT ON TABLE core.user_roles IS 'Junction table linking users to roles with optional company scope';
COMMENT ON COLUMN core.user_roles.company_id IS 'NULL = global assignment, UUID = company-scoped assignment';


-- =====================================================
-- HELPER FUNCTIONS
-- =====================================================

-- Function to check if a role has a specific resource permission
CREATE OR REPLACE FUNCTION core.has_permission(
    p_permissions JSONB,
    p_resource VARCHAR,
    p_level VARCHAR DEFAULT NULL
) RETURNS BOOLEAN AS $$
BEGIN
    IF p_level IS NULL THEN
        -- Check if resource exists (any level)
        RETURN p_permissions ? p_resource;
    ELSE
        -- Check if resource has specific level
        RETURN p_permissions->>p_resource = p_level;
    END IF;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

COMMENT ON FUNCTION core.has_permission IS 'Check if permissions JSONB contains a resource with optional level check';

-- Example usage:
-- SELECT core.has_permission(permissions, 'dashboard') FROM core.roles WHERE code = 'admin';
-- SELECT core.has_permission(permissions, 'dashboard', 'viewer') FROM core.roles WHERE code = 'admin';
