# Clients API Contract

**Version:** v1
**Base URL:** `/core/v1/admin/clients`
**Module:** Core - Clients (registration-level tenant)
**Last Updated:** 2026-04-18
**Status:** ✅ Implemented

---

## Overview

API admin untuk **clients** — entitas tenant tingkat registrasi. Dibaca sebelum implement fitur per-client lainnya (contoh: translation overrides, roadmap: branding, billing).

**Arsitektur:**

```
users (per-orang login)
 │
 ▼ (one user registers → one client)
clients ────────────── slug (unique, DNS-safe, public)
 │
 ├─► companies (company_id → client_id FK, NOT NULL)
 │       │
 │       ▼
 │    branches
 │
 └─► translation_overrides (per-client i18n strings)
```

- **1 signup = 1 client** (otomatis di-create di endpoint `POST /core/v1/auth/signup`)
- 1 client punya **1..N companies** (`companies.client_id` FK)
- User yang diinvite ke company X otomatis berada di client pemilik X
- Slug client dipakai oleh endpoint publik (tanpa auth) seperti translation bootstrap

**Base Auth:** Bearer token + role `super_admin` untuk semua endpoint di file ini.

**Tidak ada endpoint publik untuk Clients** — slug-only lookup dilakukan implisit di endpoint lain (contoh `GET /core/v1/translation-overrides?slug=...`), tidak mengembalikan daftar client.

---

## Response Format

### Standard Response

```json
{
  "data": { ... },
  "message": "..."
}
```

### HTTP Status Codes

| Status | Deskripsi |
|---|---|
| 200 OK | Success |
| 400 Bad Request | Payload invalid / slug format salah |
| 401 Unauthorized | Belum login |
| 403 Forbidden | Bukan super_admin |
| 404 Not Found | Client tidak ada |
| 409 Conflict | Slug sudah dipakai |
| 500 Internal Server Error | Server error |

---

## Data Model

### Client Object

```json
{
  "id": "a1111111-1111-1111-1111-111111111111",
  "slug": "acme",
  "name": "Acme Corporation",
  "owner_user_id": "10000000-0000-0000-0000-000000000001",
  "is_active": true,
  "created_at": "2026-04-18T03:00:00Z",
  "updated_at": "2026-04-18T03:00:00Z"
}
```

| Field | Type | Keterangan |
|---|---|---|
| `id` | UUID | Primary key |
| `slug` | string | Identifier publik DNS-safe. Format: `^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$` (3-63 chars). Unique di antara row non-deleted. Dipakai untuk bootstrap publik & subdomain routing (roadmap). |
| `name` | string | Display name client. 1-255 chars. |
| `owner_user_id` | UUID \| null | User yang registrasi. `null` kalau user dihapus (ON DELETE SET NULL). |
| `is_active` | boolean | Kalau `false`, roadmap: FE disable akses. Saat ini belum ada enforcement di backend. |
| `created_at` / `updated_at` | datetime (ISO-8601 UTC) | Audit timestamps |

### Slug Format

Regex `^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`:

- ✅ `acme`, `wiz-hub`, `client-001`, `my-company-2026`
- ❌ `Acme` (uppercase), `-acme` (leading hyphen), `acme-` (trailing hyphen), `ab` (too short)

Validasi di service layer (fail-fast 400) **dan** DB CHECK constraint.

---

## Endpoints

### 1. List Clients

```
GET /core/v1/admin/clients
```

**Auth:** Bearer token + role `super_admin`

**Response (200):**

```json
{
  "data": {
    "items": [
      {
        "id": "a1111111-...",
        "slug": "acme",
        "name": "Acme Corporation",
        "owner_user_id": "10000000-...",
        "is_active": true,
        "created_at": "2026-04-18T03:00:00Z",
        "updated_at": "2026-04-18T03:00:00Z"
      },
      {
        "id": "b2222222-...",
        "slug": "globex",
        "name": "Globex Inc",
        "owner_user_id": "10000000-...",
        "is_active": true,
        "created_at": "2026-04-17T10:00:00Z",
        "updated_at": "2026-04-17T10:00:00Z"
      }
    ],
    "total": 2
  },
  "message": "Clients retrieved successfully"
}
```

**Sorting:** fix — server selalu order `created_at DESC` (client terbaru di atas).

**Pagination:** Belum ada di v1.

**Errors:**

| Status | Message |
|---|---|
| 401 | Unauthorized |
| 403 | Insufficient permissions |

---

### 2. Get Client by ID

```
GET /core/v1/admin/clients/:id
```

**Auth:** Bearer token + role `super_admin`

**Response (200):**

```json
{
  "data": {
    "id": "a1111111-...",
    "slug": "acme",
    "name": "Acme Corporation",
    "owner_user_id": "10000000-...",
    "is_active": true,
    "created_at": "2026-04-18T03:00:00Z",
    "updated_at": "2026-04-18T03:00:00Z"
  },
  "message": "Client retrieved successfully"
}
```

**Errors:**

| Status | Message | Kapan |
|---|---|---|
| 401 | Unauthorized | Token tidak ada |
| 403 | Insufficient permissions | Bukan super_admin |
| 404 | Client not found | ID tidak ada / soft-deleted |

---

### 3. Update Client

```
PUT /core/v1/admin/clients/:id
```

**Auth:** Bearer token + role `super_admin`

**Request Body (semua field optional, update partial):**

```json
{
  "name": "Acme Corp International",
  "slug": "acme-intl",
  "is_active": true
}
```

**Validation:**

| Field | Rule |
|---|---|
| `name` | optional, min 1, max 255 |
| `slug` | optional, min 3, max 63, match `^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$` |
| `is_active` | optional, boolean |

**Behavior:**

- Field yang tidak dikirim tidak berubah (COALESCE di DB)
- Slug di-normalize ke lowercase di service sebelum validasi
- Kalau slug baru sama dengan slug existing client lain (non-deleted) → 409

**Response (200):**

```json
{
  "data": {
    "id": "a1111111-...",
    "slug": "acme-intl",
    "name": "Acme Corp International",
    "owner_user_id": "10000000-...",
    "is_active": true,
    "created_at": "2026-04-18T03:00:00Z",
    "updated_at": "2026-04-18T05:15:00Z"
  },
  "message": "Client updated successfully"
}
```

**Errors:**

| Status | Message | Kapan |
|---|---|---|
| 400 | Invalid request payload | Body invalid |
| 400 | Invalid slug | Slug tidak match regex |
| 401 | Unauthorized | Token tidak ada |
| 403 | Insufficient permissions | Bukan super_admin |
| 404 | Client not found | ID tidak ada |
| 409 | Slug is already taken | Slug konflik dengan client lain |

---

## Not Provided (yet)

| Method | Path | Reason |
|---|---|---|
| `POST /core/v1/admin/clients` | — | Client dibuat otomatis di signup. Super admin provisioning client baru tanpa user → roadmap. |
| `DELETE /core/v1/admin/clients/:id` | — | Menghapus client destructive (cascade ke companies, branches, overrides). Lakukan manual + audit di v1. |
| Public list endpoint | — | Tidak ada — resolve slug dilakukan per-endpoint (contoh translation bootstrap). Tidak ada cara unauthenticated untuk enumerate semua slug. |

---

## Example Scenarios

### A. Super admin melihat daftar semua tenant

```
GET /core/v1/admin/clients
```

Buat admin panel yang menunjukkan semua client yang sudah registrasi + tombol "Manage Translations" yang navigate ke [admin/clients/:id/translation-overrides](translation-overrides.md).

### B. Super admin rename slug client

Sebelum FE dipasang subdomain production, admin perlu fix slug dari default `client-a1b2c3d4` (auto-generated di signup) ke `acme`:

```
PUT /core/v1/admin/clients/{id}
Body: { "slug": "acme" }
```

FE bootstrap berikutnya pakai `?slug=acme`.

### C. Non-admin coba akses

```
GET /core/v1/admin/clients
Authorization: Bearer <token dengan role biasa>
```

Response **403 Forbidden** — `RequireRole(super_admin)` middleware menolak.

---

## Business Rules

| Rule | Keterangan |
|---|---|
| Auto-created | Client dibuat otomatis saat user signup. FE tidak perlu explicit "Create Client" step. |
| 1 registrasi = 1 client | Seorang user punya tepat 1 client (yang di-own via `owner_user_id`). Tidak ada multi-client-per-user di v1. |
| Slug auto-generated | Di signup, slug di-derive dari username (sanitized). Kolisi diselesaikan dengan suffix hex 4-char. Admin bisa rename lewat `PUT /admin/clients/:id`. |
| Owner transfer | Tidak ada endpoint transfer owner di v1 — butuh manual intervention kalau ownership harus pindah. |
| Soft-deletable | Schema punya `deleted_at` — belum ada endpoint delete, roadmap. |
| Companies link | Setiap company di-scope ke 1 client via `companies.client_id NOT NULL`. Dibaca dari JWT company_id claim saat user login. |
| No cross-tenant read | Endpoint translation lookup by slug TIDAK bocor apakah slug valid atau tidak dari timing — tapi 404 response langsung kasih tau slug tidak ada. Untuk v1 ini dianggap OK (slug memang publik). |

---

## Notes for FE Implementers

- **Admin client picker** — bangun halaman `/admin/clients` yang:
  - List semua client
  - Setiap row ada tombol "Manage Translations" → `/admin/clients/:id/translation-overrides`
  - Tombol "Edit" → form rename name/slug, toggle is_active
- **Slug input validation** — pakai regex `^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$` client-side. Lowercase otomatis sebelum submit.
- **Slug conflict UX** — kalau PUT balik 409, tampilkan pesan "Slug sudah dipakai" dan suggest alternative (append `-1`).
- **Current user's client** — belum ada endpoint "get my client". FE sementara bisa resolve lewat JWT (company_id → GET /companies/:id → client_id → GET /admin/clients/:id hanya kalau super_admin). Non-admin FE yang butuh tahu slug untuk bootstrap harus get dari build-time config / subdomain.

---

## Related

- [Translation Overrides Contract](translation-overrides.md) — feature pertama yang per-client
- [Auth Contract](auth.md) — signup flow yang auto-create client
- [Company Contract](company.md) — companies.client_id FK
- Schema migration: [`internal/database/migrations/core/000012_clients.up.sql`](../../../internal/database/migrations/core/000012_clients.up.sql)
- Module source: [`internal/modules/core/client/`](../../../internal/modules/core/client/)

---

**Last Updated:** 2026-04-18
