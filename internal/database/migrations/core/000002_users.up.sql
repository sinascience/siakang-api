-- =====================================================
-- CORE2 SCHEMA - USERS & AUTHENTICATION
-- =====================================================
-- Migration 002: Users and Refresh Tokens
-- =====================================================

-- Users table (simplified, all-in-one)
CREATE TABLE IF NOT EXISTS core.users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Identity
    email VARCHAR(255) NOT NULL,
    username VARCHAR(100) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(255),
    phone VARCHAR(20),
    avatar_url VARCHAR(500),

    -- Status
    is_active BOOLEAN DEFAULT TRUE,
    is_email_verified BOOLEAN DEFAULT FALSE,
    email_verified_at TIMESTAMPTZ,

    -- Security
    failed_login_count INT DEFAULT 0,
    locked_until TIMESTAMPTZ,
    last_login_at TIMESTAMPTZ,

    -- Audit
    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    deleted_by UUID
);

-- Partial unique indexes (allow re-registration after soft delete)
CREATE UNIQUE INDEX users_email_active_key ON core.users(email) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX users_username_active_key ON core.users(username) WHERE deleted_at IS NULL;

-- Performance indexes
CREATE INDEX idx_users_email ON core.users(email);
CREATE INDEX idx_users_username ON core.users(username);
CREATE INDEX idx_users_is_active ON core.users(is_active) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_deleted_at ON core.users(deleted_at) WHERE deleted_at IS NOT NULL;


-- Refresh tokens table (for JWT session management)
CREATE TABLE IF NOT EXISTS core.refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,

    -- Token
    token_hash VARCHAR(255) NOT NULL,

    -- Device info (flexible JSON structure)
    device_info JSONB DEFAULT '{}',
    -- Example: {"name": "Chrome on Windows", "type": "browser", "os": "Windows 11"}

    ip_address VARCHAR(45),  -- Supports IPv6

    -- Lifecycle
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,

    -- Audit
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Unique constraint on token_hash
CREATE UNIQUE INDEX refresh_tokens_token_hash_key ON core.refresh_tokens(token_hash);

-- Performance indexes
CREATE INDEX idx_refresh_tokens_user_id ON core.refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_expires_at ON core.refresh_tokens(expires_at) WHERE revoked_at IS NULL;
CREATE INDEX idx_refresh_tokens_revoked_at ON core.refresh_tokens(revoked_at) WHERE revoked_at IS NOT NULL;


-- =====================================================
-- COMMENTS
-- =====================================================
COMMENT ON TABLE core.users IS 'User accounts with authentication fields';
COMMENT ON COLUMN core.users.email_verified_at IS 'Timestamp when email was verified (NULL = not verified)';
COMMENT ON COLUMN core.users.failed_login_count IS 'Counter for failed login attempts, reset on successful login';
COMMENT ON COLUMN core.users.locked_until IS 'Account locked until this timestamp after max failed attempts';

COMMENT ON TABLE core.refresh_tokens IS 'JWT refresh tokens for session management';
COMMENT ON COLUMN core.refresh_tokens.device_info IS 'JSON object containing device details: name, type, os, browser';
COMMENT ON COLUMN core.refresh_tokens.revoked_at IS 'Set when token is explicitly revoked (logout)';
