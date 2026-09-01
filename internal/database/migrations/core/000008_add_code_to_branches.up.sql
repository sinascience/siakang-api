-- =====================================================
-- CORE SCHEMA - ADD CODE TO BRANCHES
-- =====================================================
-- Migration 008: Add code column to branches
-- =====================================================

ALTER TABLE core.branches
    ADD COLUMN code VARCHAR(50);

-- Ensure branch code is unique per company (soft-delete aware)
CREATE UNIQUE INDEX branches_company_code_key
    ON core.branches(company_id, code) WHERE deleted_at IS NULL AND code IS NOT NULL;

COMMENT ON COLUMN core.branches.code IS 'Short code/identifier for the branch, unique per company';
