-- =====================================================
-- CORE SCHEMA - TRANSLATION OVERRIDES (global, system-wide)
-- =====================================================
-- Migration 011: Admin-editable overrides for FE i18n strings.
--   - core.translation_overrides : one row = one i18n key override
--
-- Design notes:
--   - Global (no company_id) — overrides apply system-wide. Only
--     super_admin can create/update/delete; the list endpoint is
--     public so the FE can bootstrap translations before login.
--   - UNIQUE on translation_key lets the service use PUT-by-key
--     semantics without exposing the surrogate id.
--   - created_by / updated_by reference core.users for audit; ON DELETE
--     SET NULL so removing a user does not cascade-delete overrides.
-- =====================================================

CREATE TABLE IF NOT EXISTS core.translation_overrides (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    translation_key VARCHAR(128) NOT NULL UNIQUE,
    value           TEXT NOT NULL,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by      UUID REFERENCES core.users(id) ON DELETE SET NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by      UUID REFERENCES core.users(id) ON DELETE SET NULL,

    CONSTRAINT chk_translation_overrides_value_not_empty
        CHECK (LENGTH(TRIM(value)) > 0),
    CONSTRAINT chk_translation_overrides_key_format
        CHECK (translation_key ~ '^[a-z0-9-]+$')
);

CREATE INDEX idx_translation_overrides_updated_at
    ON core.translation_overrides (updated_at DESC);

COMMENT ON TABLE core.translation_overrides IS
    'Admin-editable i18n string overrides. Global (not company-scoped). Public list endpoint used by FE bootstrap; mutations restricted to super_admin.';
COMMENT ON COLUMN core.translation_overrides.translation_key IS
    'Stable i18n key. Format: ^[a-z0-9-]+$ (lowercase, digits, hyphen).';
COMMENT ON COLUMN core.translation_overrides.value IS
    'Override text rendered by the FE in place of the built-in default.';
COMMENT ON COLUMN core.translation_overrides.notes IS
    'Optional admin-facing note — not shown to end users.';
