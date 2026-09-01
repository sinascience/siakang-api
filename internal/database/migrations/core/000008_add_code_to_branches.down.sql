DROP INDEX IF EXISTS core.branches_company_code_key;

ALTER TABLE core.branches
    DROP COLUMN IF EXISTS code;
