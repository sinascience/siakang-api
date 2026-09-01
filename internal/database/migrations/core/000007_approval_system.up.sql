-- =====================================================
-- CORE SCHEMA - APPROVAL SYSTEM (generic, polymorphic)
-- =====================================================
-- Migration 007: Approval system reusable by all features
--   - approval_configs        : per (company, branch, feature_key)
--   - approval_config_levels  : ordered levels with role + chosen approvers
--   - approval_requests       : runtime instance per document (polymorphic)
--   - approval_request_levels : per-level audit trail with snapshot
-- =====================================================

-- ─── 1. APPROVAL CONFIGS ─────────────────────────────────────────
CREATE TABLE IF NOT EXISTS core.approval_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Scope
    company_id UUID NOT NULL,
    branch_id  UUID NOT NULL,
    feature_key VARCHAR(50) NOT NULL,

    -- Status
    is_active BOOLEAN NOT NULL DEFAULT TRUE,

    -- Audit
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    deleted_by UUID,

    CONSTRAINT chk_approval_configs_feature_key CHECK (feature_key ~ '^[a-z][a-z0-9_]*$')
);

-- One config per (company, branch, feature)
CREATE UNIQUE INDEX uniq_approval_configs_scope
    ON core.approval_configs (company_id, branch_id, feature_key)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_approval_configs_branch
    ON core.approval_configs (branch_id)
    WHERE deleted_at IS NULL;

-- ─── 2. APPROVAL CONFIG LEVELS ───────────────────────────────────
CREATE TABLE IF NOT EXISTS core.approval_config_levels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    approval_config_id UUID NOT NULL REFERENCES core.approval_configs(id) ON DELETE CASCADE,

    -- Level definition
    level SMALLINT NOT NULL,
    role_id UUID NOT NULL,
    approver_user_ids UUID[] NOT NULL DEFAULT '{}',

    -- Audit
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_approval_config_levels_level CHECK (level >= 1),
    CONSTRAINT chk_approval_config_levels_approvers
        CHECK (array_length(approver_user_ids, 1) >= 1),
    CONSTRAINT uniq_approval_config_levels_level UNIQUE (approval_config_id, level)
);

CREATE INDEX idx_approval_config_levels_config
    ON core.approval_config_levels (approval_config_id);

CREATE INDEX idx_approval_config_levels_role
    ON core.approval_config_levels (role_id);

-- ─── 3. APPROVAL REQUESTS ────────────────────────────────────────
CREATE TABLE IF NOT EXISTS core.approval_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Scope
    company_id UUID NOT NULL,
    branch_id  UUID NOT NULL,
    feature_key VARCHAR(50) NOT NULL,

    -- Polymorphic pointer to target document
    reff_type VARCHAR(100) NOT NULL,
    reff_id   UUID NOT NULL,

    -- Snapshot of config used at submission time
    approval_config_id UUID REFERENCES core.approval_configs(id) ON DELETE SET NULL,

    -- Status & progress
    status VARCHAR(20) NOT NULL DEFAULT 'waiting',
    current_level SMALLINT,
    total_levels  SMALLINT NOT NULL,

    -- Submission
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    submitted_by UUID NOT NULL,

    -- Finalization
    finalized_at TIMESTAMPTZ,
    finalized_by UUID,
    reject_reason TEXT,

    -- Audit
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_approval_requests_status
        CHECK (status IN ('waiting', 'approved', 'rejected', 'cancelled')),
    CONSTRAINT chk_approval_requests_waiting_level
        CHECK (status <> 'waiting' OR (current_level BETWEEN 1 AND total_levels)),
    CONSTRAINT chk_approval_requests_finalized
        CHECK (status = 'waiting' OR finalized_at IS NOT NULL)
);

-- A document can only have one active approval request at a time
CREATE UNIQUE INDEX uniq_approval_requests_active_doc
    ON core.approval_requests (reff_type, reff_id)
    WHERE status = 'waiting';

CREATE INDEX idx_approval_requests_doc
    ON core.approval_requests (reff_type, reff_id);

CREATE INDEX idx_approval_requests_inbox
    ON core.approval_requests (company_id, feature_key, status)
    WHERE status = 'waiting';

CREATE INDEX idx_approval_requests_branch
    ON core.approval_requests (branch_id, status)
    WHERE status = 'waiting';

CREATE INDEX idx_approval_requests_submitted_by
    ON core.approval_requests (submitted_by);

-- ─── 4. APPROVAL REQUEST LEVELS (audit trail) ────────────────────
CREATE TABLE IF NOT EXISTS core.approval_request_levels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    approval_request_id UUID NOT NULL REFERENCES core.approval_requests(id) ON DELETE CASCADE,

    -- Level
    level SMALLINT NOT NULL,

    -- Snapshot taken at submission time (so config changes do not break history)
    role_id UUID NOT NULL,
    role_name VARCHAR(100) NOT NULL,
    eligible_approver_ids UUID[] NOT NULL,

    -- Action
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    action_by UUID,
    action_at TIMESTAMPTZ,
    comment TEXT,

    -- Audit
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_approval_request_levels_status
        CHECK (status IN ('pending', 'approved', 'rejected', 'skipped')),
    CONSTRAINT chk_approval_request_levels_action
        CHECK (status = 'pending' OR (action_by IS NOT NULL AND action_at IS NOT NULL)),
    CONSTRAINT uniq_approval_request_levels_level UNIQUE (approval_request_id, level)
);

CREATE INDEX idx_approval_request_levels_request
    ON core.approval_request_levels (approval_request_id, level);

-- GIN index for approver inbox lookup
CREATE INDEX idx_approval_request_levels_inbox
    ON core.approval_request_levels USING GIN (eligible_approver_ids)
    WHERE status = 'pending';
