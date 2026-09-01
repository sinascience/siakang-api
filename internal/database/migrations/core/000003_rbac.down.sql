-- Drop helper functions
DROP FUNCTION IF EXISTS core.has_permission(JSONB, VARCHAR, VARCHAR);

-- Drop user_roles first (has FK to roles and users)
DROP TABLE IF EXISTS core.user_roles;

-- Drop roles
DROP TABLE IF EXISTS core.roles;
