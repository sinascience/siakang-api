# The `market` schema is not company-scoped

**Status:** in force since 2026-09-02.
**Ruling:** product, 2026-09-02
(`siakang-pipeline/inbox/2026-09-02-product-to-be-master-001.md`), restated at
the top of the frozen contract `siakang-pipeline/contract/api-v1.yaml`.
**Applies to:** every table in the `market` schema and every route under
`/market/v1/*`.

## What the repository normally does

`CLAUDE.md` states the rule for this codebase, and the `core` schema follows it
everywhere:

> Include `company_id UUID NOT NULL` for multi-tenant tables. […] Always filter
> by `company_id` in repository queries. Use `CompanyContext()` middleware for
> tenant-isolated endpoints.

Middleware chain: `JWTAuth()` → `CompanyContext()` → `RequirePermission()`. A
user picks a company with `POST /core/v1/auth/switch-company`, gets a new JWT
carrying `company_id`, and every repository query filters by it.

## What the `market` schema does instead

**No table in the `market` schema has a `company_id`. None. Not orders, not
wallets, not the ledger.**

`/market/v1/*` runs **`JWTAuth()` only** — no `CompanyContext()`, no
`RequirePermission()`. Marketplace users hold no core RBAC permissions
(`permissions` is `[]` for them and is never consulted), and the JWT's
`company_id` is ignored by every marketplace endpoint. FE never calls
`switch-company` in any marketplace flow.

## Why

Tuai's `core` is a B2B product: a company is the tenant, and every row belongs
to one. SIAKANG is a consumer marketplace running beside it, and the same shape
does not fit.

- **A marketplace is single-tenant by nature.** The catalog is platform-wide on
  purpose — the whole product is customers finding *any* lapak, not lapaks
  inside their own company's walled garden. A `company_id` on `products` would
  be a column with one value in it.
- **It would put a company switch in front of every success criterion.** Budi is
  a person who wants a freezer fixed. Company-scoping the marketplace would
  require him to own a company, and to switch into it, before he could browse.
  That is not a small tax on the UX; it is a step in front of all seven of
  `goal.md`'s criteria.
- **The isolation that actually matters here is ownership, not tenancy.** The
  real question a marketplace asks is "is this *your* order / *your* wallet /
  *your* bid", which `company_id` cannot answer — two customers of the same
  platform must not read each other's orders regardless of any company.

## What replaces it

**Ownership-based authorization, enforced in repository `WHERE` clauses**, the
same place `company_id` filtering would have gone. The user id comes from the
JWT and constrains every read and write:

```go
// A customer reads their own orders, a lapak reads orders placed against them.
// The predicate is the authorization — there is no separate check to forget.
WHERE o.customer_user_id = $1
WHERE o.lapak_id = (SELECT id FROM market.lapak_profiles WHERE user_id = $1)
```

Persona (`customer` / `lapak`) comes from **seeded global role assignments** —
`core.user_roles` rows with `company_id IS NULL` — which is exactly what
`GetUserRoles(userID, nil)` already selects. Persona-specific routes check the
persona; they never check a permission.

The rule to keep, stated the way `CLAUDE.md` states its own:

> **Every `market` repository method takes the caller's user id and filters by
> it.** A query that reads or writes a `market` row without constraining it to
> the caller is a bug, in the same way a missing `company_id` filter is a bug in
> `core`.

## Boundaries

- **`core` is untouched.** No core table gained or lost a column for SIAKANG.
  The marketplace personas are ordinary `core.users` rows with global role
  assignments and no company — that is why supporting them needed no core code.
- **The two schemas do not join in `core`'s direction.** `GET /core/v1/auth/me`
  performs no join into `market` (product ruling); marketplace identity lives at
  `GET /market/v1/me`, which returns the caller's lapak profile or `null`. Core
  stays self-contained, and the schemas stay separable.
- **This deviation does not travel.** It licenses the `market` schema and
  nothing else. A new module under `/core/v1` follows `CLAUDE.md` as written.

## If the marketplace ever needs tenants

It would mean SIAKANG had become multi-tenant SaaS — several marketplaces on one
deployment — not that this decision was wrong for a single marketplace. That
change is a migration adding `company_id` to `market` tables plus
`CompanyContext()` on the route group, and it should be made deliberately, with
its own ruling, rather than by adding the column to one table because it looked
inconsistent.

## Related

- Schema and its inline design notes:
  `internal/database/migrations/core/000015_market_schema.up.sql`
- Seeded actors and their personas: [`docs/seed-actors.md`](../seed-actors.md)
