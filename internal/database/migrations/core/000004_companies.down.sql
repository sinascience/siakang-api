-- Drop helper functions
DROP FUNCTION IF EXISTS core.get_user_companies(UUID);
DROP FUNCTION IF EXISTS core.get_company_descendants(UUID);
DROP FUNCTION IF EXISTS core.get_company_ancestors(UUID);

-- Drop foreign keys from roles tables
ALTER TABLE core.user_roles DROP CONSTRAINT IF EXISTS fk_user_roles_company_id;
ALTER TABLE core.roles DROP CONSTRAINT IF EXISTS fk_roles_company_id;

-- Drop company_users first (has FK to companies)
DROP TABLE IF EXISTS core.company_users;

-- Drop companies
DROP TABLE IF EXISTS core.companies;

-- Drop enum type
DROP TYPE IF EXISTS core.company_type;
