-- =====================================================
-- CORE SCHEMA - CLIENTS (registration-level tenant)
-- =====================================================
-- Migration 012: Clients table + companies.client_id link.
--   - core.clients        : one row per "client organization"
--   - core.companies      : now carries client_id NOT NULL
--
-- Design notes:
--   - A "client" is the tenant that owns one or more companies. One
--     signup = one client. Under a client, the registrant can create
--     multiple companies (holding + subsidiaries).
--   - slug is DNS-safe and globally unique among non-deleted rows. It
--     is used by the unauthenticated translation-bootstrap endpoint
--     (?slug=acme) and is reserved for future subdomain routing
--     (acme.tuai.id).
--   - companies.client_id starts nullable so existing rows can be
--     backfilled in-place; the NOT NULL constraint is applied after
--     backfill completes in the same migration.
--   - Slug regex enforces POSIX ERE (Postgres ~): 3–63 chars, lower
--     alnum + hyphen, must start and end with alnum.
-- =====================================================

CREATE TABLE IF NOT EXISTS core.clients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    slug VARCHAR(63) NOT NULL,
    name VARCHAR(255) NOT NULL,

    -- The user who registered. ON DELETE SET NULL so deleting the
    -- registrant does not cascade; a separate ownership-transfer flow
    -- is required for production use.
    owner_user_id UUID REFERENCES core.users(id) ON DELETE SET NULL,

    is_active BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    deleted_by UUID,

    CONSTRAINT chk_clients_slug_format
        CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$')
);

-- Slug unique among non-deleted rows (mirrors core.users.email pattern
-- so a soft-deleted client's slug can eventually be reused).
CREATE UNIQUE INDEX clients_slug_active_key
    ON core.clients (slug) WHERE deleted_at IS NULL;

CREATE INDEX idx_clients_owner_user_id
    ON core.clients (owner_user_id) WHERE deleted_at IS NULL;

CREATE INDEX idx_clients_is_active
    ON core.clients (is_active) WHERE deleted_at IS NULL;

-- Add client_id to companies (nullable during backfill)
ALTER TABLE core.companies
    ADD COLUMN client_id UUID REFERENCES core.clients(id) ON DELETE RESTRICT;

-- =====================================================
-- BACKFILL: one client per distinct company owner
-- =====================================================
-- For each distinct owner_id in core.companies:
--   1. Derive a slug from the user's username (sanitized + length
--      clamp + collision suffix), fallback to 'client-{hex}'.
--   2. Insert a client row owned by that user.
--   3. Link all of that user's companies to the new client.
-- After the loop, client_id is SET NOT NULL.
--
-- If core.companies is empty (fresh DB), the loop is a no-op.
-- =====================================================
DO $$
DECLARE
    v_user RECORD;
    v_slug TEXT;
    v_base TEXT;
    v_client_id UUID;
    v_suffix INT;
BEGIN
    FOR v_user IN
        SELECT DISTINCT owner_id AS user_id
        FROM core.companies
    LOOP
        -- Base slug: lowercase username with non-[a-z0-9] replaced by '-'.
        SELECT lower(regexp_replace(coalesce(username, ''), '[^a-z0-9]+', '-', 'g'))
          INTO v_base
          FROM core.users
         WHERE id = v_user.user_id;

        -- Normalise: collapse runs of '-', strip leading/trailing '-'.
        v_base := regexp_replace(coalesce(v_base, ''), '-+', '-', 'g');
        v_base := regexp_replace(v_base, '^-+|-+$', '', 'g');

        -- Fallback if too short / empty to pass the 3-char check.
        IF length(v_base) < 3 THEN
            v_base := 'client-' || substr(replace(gen_random_uuid()::text, '-', ''), 1, 8);
        END IF;

        -- Clamp to max slug length.
        IF length(v_base) > 63 THEN
            v_base := substr(v_base, 1, 63);
        END IF;

        -- Resolve collisions by appending -1, -2, ... (length-aware).
        v_slug := v_base;
        v_suffix := 1;
        WHILE EXISTS (
            SELECT 1 FROM core.clients
            WHERE slug = v_slug AND deleted_at IS NULL
        ) LOOP
            v_slug := substr(
                v_base,
                1,
                greatest(3, 63 - length('-' || v_suffix::text))
            ) || '-' || v_suffix;
            v_suffix := v_suffix + 1;
        END LOOP;

        INSERT INTO core.clients (
            id, slug, name, owner_user_id, created_by, updated_by
        )
        SELECT gen_random_uuid(),
               v_slug,
               coalesce(nullif(u.full_name, ''), u.username, v_slug),
               u.id,
               u.id,
               u.id
          FROM core.users u
         WHERE u.id = v_user.user_id
        RETURNING id INTO v_client_id;

        UPDATE core.companies
           SET client_id = v_client_id
         WHERE owner_id = v_user.user_id;
    END LOOP;
END $$;

-- Enforce NOT NULL now that every existing row is linked.
ALTER TABLE core.companies
    ALTER COLUMN client_id SET NOT NULL;

CREATE INDEX idx_companies_client_id
    ON core.companies (client_id) WHERE deleted_at IS NULL;

-- =====================================================
-- COMMENTS
-- =====================================================
COMMENT ON TABLE core.clients IS
    'Registration-level tenant. One row per signup. A client owns one or more companies and is the scope for per-tenant customisation (translations, future billing/branding).';
COMMENT ON COLUMN core.clients.slug IS
    'DNS-safe public identifier (3–63 chars, ^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$). Used by unauthenticated bootstrap endpoints and reserved for future subdomain routing.';
COMMENT ON COLUMN core.clients.owner_user_id IS
    'User who registered the client. FK is ON DELETE SET NULL — deleting the owner does not cascade; a separate ownership-transfer flow is required.';
COMMENT ON COLUMN core.companies.client_id IS
    'FK to the client that owns this company. Every company belongs to exactly one client. Multi-company clients are the expected shape.';
