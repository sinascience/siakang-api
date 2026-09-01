-- =====================================================
-- CORE SCHEMA - TRANSLATION OVERRIDES (revert per-client)
-- =====================================================

ALTER TABLE core.translation_overrides
    DROP CONSTRAINT IF EXISTS uq_translation_overrides_client_key;

DROP INDEX IF EXISTS core.idx_translation_overrides_client;

ALTER TABLE core.translation_overrides
    DROP COLUMN IF EXISTS client_id;

-- Truncate to guarantee no dupes when re-adding the global UNIQUE.
TRUNCATE TABLE core.translation_overrides;

ALTER TABLE core.translation_overrides
    ADD CONSTRAINT translation_overrides_translation_key_key
        UNIQUE (translation_key);
