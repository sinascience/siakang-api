-- =====================================================
-- CORE SCHEMA - BRANCHES
-- =====================================================
-- Migration 006: Branches linked to companies
-- =====================================================

CREATE TABLE IF NOT EXISTS core.branches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Company relation (no FK constraint)
    company_id UUID NOT NULL,

    -- Identity
    name VARCHAR(255) NOT NULL,
    logo_url VARCHAR(500),

    -- Display
    sort INT DEFAULT 0,

    -- Status
    is_default BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,

    -- Audit
    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    deleted_by UUID
);

-- Ensure only one default branch per company
CREATE UNIQUE INDEX branches_company_default_key
    ON core.branches(company_id) WHERE is_default = TRUE AND deleted_at IS NULL;

-- Performance indexes
CREATE INDEX idx_branches_company_id ON core.branches(company_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_branches_is_default ON core.branches(is_default) WHERE deleted_at IS NULL;
CREATE INDEX idx_branches_is_active ON core.branches(is_active) WHERE deleted_at IS NULL;
CREATE INDEX idx_branches_sort ON core.branches(sort) WHERE deleted_at IS NULL;

-- Comments
COMMENT ON TABLE core.branches IS 'Branch offices belonging to a company';
COMMENT ON COLUMN core.branches.company_id IS 'Company this branch belongs to (no FK constraint)';
COMMENT ON COLUMN core.branches.is_default IS 'Default branch for the company (only one per company)';
