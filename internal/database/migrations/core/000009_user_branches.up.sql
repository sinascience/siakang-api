-- =====================================================
-- CORE SCHEMA - USER BRANCHES
-- =====================================================
-- Migration 009: Per-user branch access (scopes what branches
-- a user can operate under within their member companies).
-- =====================================================

CREATE TABLE IF NOT EXISTS core.user_branches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Relations
    user_id   UUID NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,
    branch_id UUID NOT NULL REFERENCES core.branches(id) ON DELETE CASCADE,

    -- Audit
    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    deleted_by UUID
);

-- One active assignment per (user, branch)
CREATE UNIQUE INDEX user_branches_user_branch_active_key
    ON core.user_branches(user_id, branch_id) WHERE deleted_at IS NULL;

-- Performance indexes
CREATE INDEX idx_user_branches_user_id   ON core.user_branches(user_id)   WHERE deleted_at IS NULL;
CREATE INDEX idx_user_branches_branch_id ON core.user_branches(branch_id) WHERE deleted_at IS NULL;

COMMENT ON TABLE core.user_branches IS 'Per-user branch access; narrows a member user''s scope within their companies';
