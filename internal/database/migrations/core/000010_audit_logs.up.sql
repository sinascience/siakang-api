-- =====================================================
-- CORE SCHEMA - AUDIT LOGS (generic, polymorphic, append-only)
-- =====================================================
-- Migration 010: Per-entity audit trail reusable by all features.
--   - core.audit_logs : one row = one event that happened to an entity
--
-- Design notes:
--   - Polymorphic (reff_type, reff_id) follows the pattern used by
--     core.approval_requests — schema-qualified so entries can span
--     multiple schemas (finance, rental, ...) without name collisions.
--   - Append-only: no updated_at/by or deleted_at/by columns. Rows are
--     immutable by convention; service layer never issues UPDATE/DELETE.
--   - Actor fields (actor_name, actor_role) are snapshotted at write
--     time so history remains readable if the user is renamed/deleted.
--     Same pattern as core.approval_request_levels.role_name.
--   - No FKs to other modules per project convention — validated in
--     service layer.
-- =====================================================

CREATE TABLE IF NOT EXISTS core.audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Scope
    company_id UUID NOT NULL,
    branch_id  UUID,

    -- Polymorphic pointer to target entity
    reff_type VARCHAR(100) NOT NULL,
    reff_id   UUID NOT NULL,
    reff_no   VARCHAR(50),

    -- Event
    action  VARCHAR(40) NOT NULL,
    summary TEXT,

    -- Field-level diff (populated on action = 'updated')
    changes JSONB,

    -- Extra context (approval level, reject reason, attachment name, ...)
    metadata JSONB,

    -- Actor snapshot
    actor_id   UUID,
    actor_name VARCHAR(150),
    actor_role VARCHAR(100),

    -- Request fingerprint
    ip_address INET,
    user_agent TEXT,
    request_id VARCHAR(64),

    -- Timestamps
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_audit_logs_action
        CHECK (action ~ '^[a-z][a-z0-9_]*$'),
    CONSTRAINT chk_audit_logs_reff_type
        CHECK (reff_type ~ '^[a-z][a-z0-9_.]*$')
);

-- Primary use case: "show audit history on entity detail page"
CREATE INDEX idx_audit_logs_reff
    ON core.audit_logs (reff_type, reff_id, occurred_at DESC);

-- "All actions by this user" — admin/user activity view
CREATE INDEX idx_audit_logs_actor
    ON core.audit_logs (company_id, actor_id, occurred_at DESC)
    WHERE actor_id IS NOT NULL;

-- Company-wide audit dashboard + date range
CREATE INDEX idx_audit_logs_company_time
    ON core.audit_logs (company_id, occurred_at DESC);

-- Branch-scoped audit listing
CREATE INDEX idx_audit_logs_branch
    ON core.audit_logs (branch_id, occurred_at DESC)
    WHERE branch_id IS NOT NULL;

-- Filter by action type (e.g. all rejects across a company)
CREATE INDEX idx_audit_logs_action
    ON core.audit_logs (company_id, action, occurred_at DESC);

-- ─── COMMENTS ────────────────────────────────────────────────────

COMMENT ON TABLE core.audit_logs IS
    'Append-only per-entity audit trail. One row = one event. Immutable by convention.';

COMMENT ON COLUMN core.audit_logs.reff_type IS
    'Polymorphic discriminator, schema-qualified: finance.cash_transactions, finance.journal_entries, rental.rentals, ...';
COMMENT ON COLUMN core.audit_logs.reff_no IS
    'Snapshot of the entity document number (e.g. CI/JKT-202604/0001) so detail UIs and global dashboards can render without joining the source table.';
COMMENT ON COLUMN core.audit_logs.action IS
    'Event kind: created, updated, submitted, approved, rejected, cancelled, posted, voided, deleted, restored, attachment_added, attachment_removed, ... — snake_case, extensible (validated by regex, not CHECK IN, so new modules can add actions without migration).';
COMMENT ON COLUMN core.audit_logs.summary IS
    'Human-readable one-line description rendered by the FE as-is.';
COMMENT ON COLUMN core.audit_logs.changes IS
    'Field-level diff, only populated when action = updated. Format: {"field_name": {"old": ..., "new": ...}}. Sensitive fields are masked by the service layer.';
COMMENT ON COLUMN core.audit_logs.metadata IS
    'Free-form context for the event: approval level, comment, reject reason, filename, correlation ids, etc.';
COMMENT ON COLUMN core.audit_logs.actor_name IS
    'Snapshot of the user display name at the time of the event — preserves history if the user is later renamed or deleted.';
COMMENT ON COLUMN core.audit_logs.actor_role IS
    'Snapshot of the role the actor used to perform the action (users may change roles later).';
COMMENT ON COLUMN core.audit_logs.occurred_at IS
    'When the event happened per business logic. Separated from created_at so backfills/replays can carry their original timestamp.';
COMMENT ON COLUMN core.audit_logs.created_at IS
    'When the row was written to the database. Append-only — never updated.';
