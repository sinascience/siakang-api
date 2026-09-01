-- =====================================================
-- CORE SCHEMA - TRANSLATION OVERRIDES (per-client rescope)
-- =====================================================
-- Migration 013: Re-scope core.translation_overrides from global to
-- per-client. The table was introduced in migration 011 with a global
-- UNIQUE(translation_key); this migration:
--   1. Drops the global unique
--   2. Adds client_id FK
--   3. Replaces unique with (client_id, translation_key)
--
-- IMPORTANT — destructive step:
--   The migration TRUNCATEs core.translation_overrides. The table has
--   no production data at the time this migration is authored (it was
--   created in 011 and not yet wired to the FE). If you are running
--   this against a DB with existing overrides, export them first and
--   re-import them with the target client_id after migration.
-- =====================================================

TRUNCATE TABLE core.translation_overrides;

ALTER TABLE core.translation_overrides
    DROP CONSTRAINT IF EXISTS translation_overrides_translation_key_key;

ALTER TABLE core.translation_overrides
    ADD COLUMN client_id UUID NOT NULL
        REFERENCES core.clients(id) ON DELETE CASCADE;

ALTER TABLE core.translation_overrides
    ADD CONSTRAINT uq_translation_overrides_client_key
        UNIQUE (client_id, translation_key);

CREATE INDEX idx_translation_overrides_client
    ON core.translation_overrides (client_id);

COMMENT ON COLUMN core.translation_overrides.client_id IS
    'Client that owns this override. Overrides are scoped per-client; two clients can have different values for the same translation_key. CASCADE on client delete cleans up orphans.';
