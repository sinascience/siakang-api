-- =====================================================
-- MARKET SCHEMA - SIAKANG MARKETPLACE (rollback)
-- =====================================================
-- The up migration creates the `market` schema and nothing outside it: no
-- core table gained a column, so dropping the schema is a complete reverse.
-- CASCADE takes the tables, indexes, constraints and comments with it.
--
-- The FKs that point INTO core (users) are owned by market tables, so they
-- go too; core is left exactly as migration 014 left it.
--
-- Seeded marketplace personas live in core.users / core.roles /
-- core.user_roles and are seed data, not schema - `make db-reset` re-seeds
-- them. This migration deliberately does not delete them: a rollback should
-- undo structure, not silently remove rows another seeder owns.
-- =====================================================

DROP SCHEMA IF EXISTS market CASCADE;
