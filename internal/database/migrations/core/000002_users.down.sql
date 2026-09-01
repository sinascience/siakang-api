-- Drop refresh_tokens first (has FK to users)
DROP TABLE IF EXISTS core.refresh_tokens;

-- Drop users
DROP TABLE IF EXISTS core.users;
