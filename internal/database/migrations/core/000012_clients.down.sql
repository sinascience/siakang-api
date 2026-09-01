-- =====================================================
-- CORE SCHEMA - CLIENTS (rollback)
-- =====================================================

ALTER TABLE core.companies
    DROP COLUMN IF EXISTS client_id;

DROP TABLE IF EXISTS core.clients CASCADE;
