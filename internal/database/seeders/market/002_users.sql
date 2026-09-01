-- =====================================================
-- MARKET SEEDER - PERSONA USERS
-- =====================================================
-- Seeder 002: the four SIAKANG actors as ordinary core.users rows.
--
-- Password for EVERY seeded account: siakang123
-- (bcrypt cost 10, the repo's bcrypt.DefaultCost - same format as
-- core seeder 002_users.sql).
--
-- These users deliberately have NO client row and NO company. Marketplace
-- flows never call /core/v1/auth/switch-company, and `me` omits company and
-- client for a user who has none. Do not "fix" that by giving them one.
--
-- Ids continue the core user block (10000000-...-0001 owner,
-- -0002 client), starting at -0011 to leave the core sequence room.
--
-- Full detail (coordinates, balances, expected match ordering):
-- docs/seed-actors.md
-- =====================================================

INSERT INTO core.users (id, email, username, password_hash, full_name, phone, is_active, is_email_verified, email_verified_at) VALUES
-- Budi - the customer. The actor for every goal.md criterion.
(
    '10000000-0000-0000-0000-000000000011',
    'budi@siakang.test',
    'budi',
    '$2a$10$3j9hQIZsnPh08l7/fKIuqOWBIE1WlL8l.XN/Y1znBXzPH2s3LhN3y', -- siakang123
    'Budi Santoso',
    '+6281234500011',
    TRUE,
    TRUE,
    NOW()
),
-- Siti - the SECOND customer. She exists only to be the non-participant in
-- ownership assertions ("someone else's order" must 404), so those tests have
-- a stable actor instead of an ad-hoc signup. Contract amendment v1.0.4.
(
    '10000000-0000-0000-0000-000000000015',
    'siti@siakang.test',
    'siti',
    '$2a$10$3j9hQIZsnPh08l7/fKIuqOWBIE1WlL8l.XN/Y1znBXzPH2s3LhN3y', -- siakang123
    'Siti Rahayu',
    '+6281234500015',
    TRUE,
    TRUE,
    NOW()
),
-- Joko - lapak, NEAREST available to Budi. The expected automatic match
-- and the counterparty for the gig/upsell and chat criteria.
(
    '10000000-0000-0000-0000-000000000012',
    'joko@siakang.test',
    'joko',
    '$2a$10$3j9hQIZsnPh08l7/fKIuqOWBIE1WlL8l.XN/Y1znBXzPH2s3LhN3y', -- siakang123
    'Joko Prasetyo',
    '+6281234500012',
    TRUE,
    TRUE,
    NOW()
),
-- Sari - lapak, farther away but rated HIGHER than Joko. Exists to lose
-- the automatic match, proving matching is nearest-first.
(
    '10000000-0000-0000-0000-000000000013',
    'sari@siakang.test',
    'sari',
    '$2a$10$3j9hQIZsnPh08l7/fKIuqOWBIE1WlL8l.XN/Y1znBXzPH2s3LhN3y', -- siakang123
    'Sari Wulandari',
    '+6281234500013',
    TRUE,
    TRUE,
    NOW()
),
-- Agus - lapak, NEAREST and best-rated but UNAVAILABLE. Exists to lose the
-- automatic match on availability alone. Still offers on manual bids.
(
    '10000000-0000-0000-0000-000000000014',
    'agus@siakang.test',
    'agus',
    '$2a$10$3j9hQIZsnPh08l7/fKIuqOWBIE1WlL8l.XN/Y1znBXzPH2s3LhN3y', -- siakang123
    'Agus Setiawan',
    '+6281234500014',
    TRUE,
    TRUE,
    NOW()
)
ON CONFLICT DO NOTHING;
