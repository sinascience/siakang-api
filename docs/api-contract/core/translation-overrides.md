# Translation Overrides API Contract

**Version:** v1
**Base URL:** `/core/v1`
**Module:** Core - Translation Overrides (per-client)
**Last Updated:** 2026-04-18
**Status:** ✅ Implemented

---

## Overview

API untuk mengelola **i18n string overrides** yang menggantikan teks default FE. Scope per-**client** (tenant registrasi), bukan per-company — satu client punya peta terjemahan sendiri yang berlaku di semua company miliknya.

**Arsitektur client (baca dulu sebelum implement):**

- 1 registrasi (signup) = 1 **client** (`core.clients`)
- 1 client bisa punya banyak **company** (`core.companies.client_id`)
- Semua user yang member di company-company milik 1 client → pakai terjemahan yang sama
- Setiap client punya **slug** DNS-safe unik (contoh `acme`, `tuai-demo`) yang dipakai FE untuk bootstrap tanpa login

**Key Features:**

- **Per-client scope** — unique key adalah `(client_id, translation_key)`. Dua client berbeda boleh punya value beda untuk key yang sama.
- **Public bootstrap lewat slug** — `GET /core/v1/translation-overrides?slug=acme` tidak perlu auth, dirancang untuk FE memuat terjemahan sebelum user login.
- **Scope-guarded self-service** — user dari client X bisa kelola overrides milik client X (tidak perlu super_admin). Cross-client access ditolak 403. super_admin bypass ke semua client.
- **Nested admin routes** — endpoint admin hidup di bawah `/admin/clients/:id/...` sehingga tenant boundary eksplisit di URL.
- **Immutable key** — setelah create, `translation_key` tidak bisa di-rename; pakai DELETE + POST.
- **Reset = DELETE** — menghapus row berarti FE jatuh ke string default built-in.

**Base Auth:**

- Public endpoint: **tidak perlu auth**
- Admin endpoints: Bearer token + **dua layer**:
  1. **Client scope match** — `claims.client_id` JWT harus sama dengan `:id` URL (super_admin bypass)
  2. **Permission** pada resource `translation_overrides` sesuai action (super_admin bypass)

| Endpoint | Permission |
|---|---|
| `GET  .../translation-overrides` | `translation_overrides:read` |
| `GET  .../translation-overrides/:key` | `translation_overrides:read` |
| `POST .../translation-overrides` | `translation_overrides:create` |
| `PUT  .../translation-overrides/:key` | `translation_overrides:update` |
| `DELETE .../translation-overrides/:key` | `translation_overrides:delete` |

Role `administrator` (default di signup) sudah mendapat level `admin` untuk resource ini — berarti user yang registrasi punya full CRUD di client miliknya sendiri out-of-the-box. Tanpa CompanyContext (overrides tidak per-company).

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
| 200 OK | Success (GET, PUT, DELETE) |
| 201 Created | Berhasil create (POST) |
| 400 Bad Request | Payload invalid / format `translation_key` salah / `value` empty / `slug` query missing |
| 401 Unauthorized | Belum login (admin endpoints) |
| 403 Forbidden | Bukan super_admin (admin endpoints) |
| 404 Not Found | Key atau client tidak ada |
| 409 Conflict | `translation_key` sudah dipakai untuk client ini (POST) |
| 500 Internal Server Error | Server error |

---

## Data Model

### TranslationOverride Object (admin view)

```json
{
  "id": "c0ffee00-1234-5678-9abc-def012345678",
  "client_id": "a1111111-1111-1111-1111-111111111111",
  "translation_key": "button-save",
  "value": "Simpan",
  "notes": "Changed from 'Save' for consistency",
  "created_at": "2026-04-18T03:21:00Z",
  "created_by": "10000000-0000-0000-0000-000000000001",
  "updated_at": "2026-04-18T03:21:00Z",
  "updated_by": "10000000-0000-0000-0000-000000000001"
}
```

| Field | Type | Keterangan |
|---|---|---|
| `id` | UUID | Primary key (surrogate) |
| `client_id` | UUID | Scope tenant — overrides hanya berlaku untuk client ini |
| `translation_key` | string | Key i18n. Format: `^[a-z0-9-]+$`, max 128 karakter |
| `value` | string | Teks override yang dirender FE. Tidak boleh empty/whitespace-only |
| `notes` | string \| null | Catatan internal admin — tidak terlihat end user |
| `created_at` / `updated_at` | datetime (ISO-8601 UTC) | Audit timestamps |
| `created_by` / `updated_by` | UUID \| null | Snapshot user id, null kalau user dihapus |

**Unique constraint:** `(client_id, translation_key)` — kombinasi unik, bukan key saja.

### Key Format

`translation_key` harus cocok regex `^[a-z0-9-]+$`:

- ✅ `button-save`, `page-title-dashboard`, `error-404`
- ❌ `Button.Save`, `button_save`, `button save`

Validasi di service layer (fail-fast 400) **dan** DB CHECK constraint.

---

## Endpoints

### 1. Public: Get Translations by Slug (FE bootstrap)

```
GET /core/v1/translation-overrides?slug={client_slug}
```

**Auth:** Tidak perlu (public)

**Query Parameters:**

| Param | Type | Required | Keterangan |
|---|---|---|---|
| `slug` | string | ✅ | Slug client (contoh: `acme`). Case-sensitive. Lihat **Clients API** untuk cara FE mendapatkan slug. |

**Response (200) — saat ada overrides:**

```json
{
  "data": {
    "client_id": "a1111111-1111-1111-1111-111111111111",
    "slug": "acme",
    "translations": {
      "button-save": "Simpan",
      "button-cancel": "Batal",
      "page-title-dashboard": "Beranda",
      "error-404": "Halaman tidak ditemukan"
    }
  },
  "message": "Translations retrieved successfully"
}
```

**Response (200) — saat client ada tapi belum ada overrides:**

```json
{
  "data": {
    "client_id": "a1111111-...",
    "slug": "acme",
    "translations": {}
  },
  "message": "Translations retrieved successfully"
}
```

**Errors:**

| Status | Message | Kapan |
|---|---|---|
| 400 | slug query parameter is required | `?slug=` tidak ada atau kosong |
| 404 | Client not found | Slug tidak match ke client manapun |
| 500 | Internal server error | Server error |

**Catatan:**

- Payload flat `{ key: value }` sengaja agar langsung dikonsumsi i18n library (i18next, vue-i18n, react-intl) tanpa transformasi di FE.
- Field admin (`id`, `notes`, `created_by`, dst.) **tidak ada** di response publik.
- Aman di-cache di CDN short-TTL (beberapa menit).

---

### 2. List Translation Overrides for a Client (admin)

```
GET /core/v1/admin/clients/:id/translation-overrides
```

**Auth:** Bearer token + `claims.client_id == :id` (super_admin bypass)

**Path Parameters:**

| Param | Type | Keterangan |
|---|---|---|
| `client_id` | UUID | Client yang overrides-nya ingin dilihat |

**Response (200):**

```json
{
  "data": {
    "items": [
      {
        "id": "c0ffee00-...",
        "client_id": "a1111111-...",
        "translation_key": "button-save",
        "value": "Simpan",
        "notes": null,
        "created_at": "2026-04-18T03:21:00Z",
        "created_by": "10000000-...",
        "updated_at": "2026-04-18T04:10:00Z",
        "updated_by": "10000000-..."
      }
    ],
    "total": 1
  },
  "message": "Translation overrides retrieved successfully"
}
```

**Sorting:** fix — server selalu order `updated_at DESC`.

**Pagination:** Belum ada di v1 (jumlah override per client kecil).

**Errors:**

| Status | Message | Kapan |
|---|---|---|
| 401 | Unauthorized | Token tidak ada |
| 403 | You cannot access this client's resources | `:id` di URL bukan client si caller (dan bukan super_admin) |
| 404 | Client not found | `client_id` tidak ada |

---

### 3. Get Translation Override by Key (admin)

```
GET /core/v1/admin/clients/:id/translation-overrides/:key
```

**Auth:** Bearer token + `claims.client_id == :id` (super_admin bypass)

**Path Parameters:**

| Param | Type | Keterangan |
|---|---|---|
| `client_id` | UUID | Scope tenant |
| `key` | string | `translation_key` — case-sensitive exact match |

**Response (200):**

```json
{
  "data": {
    "id": "c0ffee00-...",
    "client_id": "a1111111-...",
    "translation_key": "button-save",
    "value": "Simpan",
    "notes": null,
    "created_at": "2026-04-18T03:21:00Z",
    "created_by": "10000000-...",
    "updated_at": "2026-04-18T04:10:00Z",
    "updated_by": "10000000-..."
  },
  "message": "Translation override retrieved successfully"
}
```

**Errors:**

| Status | Message | Kapan |
|---|---|---|
| 401 | Unauthorized | Token tidak ada |
| 403 | You cannot access this client's resources | `:id` di URL bukan client si caller (dan bukan super_admin) |
| 404 | Translation override not found | Key tidak ada untuk client ini |

---

### 4. Create Translation Override (admin)

```
POST /core/v1/admin/clients/:id/translation-overrides
```

**Auth:** Bearer token + `claims.client_id == :id` (super_admin bypass)

**Path Parameters:**

| Param | Type | Keterangan |
|---|---|---|
| `client_id` | UUID | Client yang akan menerima override baru |

**Request Body:**

```json
{
  "translation_key": "button-save",
  "value": "Simpan",
  "notes": "Changed from 'Save' for consistency"
}
```

**Validation:**

| Field | Rule |
|---|---|
| `translation_key` | required, min 1, max 128, match `^[a-z0-9-]+$` |
| `value` | required, min 1, bukan whitespace-only |
| `notes` | optional |

**Response (201):**

```json
{
  "data": {
    "id": "c0ffee00-...",
    "client_id": "a1111111-...",
    "translation_key": "button-save",
    "value": "Simpan",
    "notes": "Changed from 'Save' for consistency",
    "created_at": "2026-04-18T03:21:00Z",
    "created_by": "10000000-...",
    "updated_at": "2026-04-18T03:21:00Z",
    "updated_by": "10000000-..."
  },
  "message": "Translation override created successfully"
}
```

**Errors:**

| Status | Message | Kapan |
|---|---|---|
| 400 | Invalid request payload | Required field missing / JSON malformed |
| 400 | Invalid translation_key | Format key tidak match |
| 400 | Invalid value | Value empty / whitespace-only |
| 401 | Unauthorized | Token tidak ada |
| 403 | You cannot access this client's resources | `:id` di URL bukan client si caller (dan bukan super_admin) |
| 404 | Client not found | `client_id` tidak ada |
| 409 | Translation key already exists for this client | Kombinasi `(client_id, translation_key)` sudah ada — pakai PUT untuk update |

---

### 5. Update Translation Override (admin)

```
PUT /core/v1/admin/clients/:id/translation-overrides/:key
```

**Auth:** Bearer token + `claims.client_id == :id` (super_admin bypass)

**Path Parameters:** `client_id`, `key`

**Request Body:**

```json
{
  "value": "Simpan Dokumen",
  "notes": "Updated phrasing per product team"
}
```

**Validation:**

| Field | Rule |
|---|---|
| `value` | required, min 1, bukan whitespace-only |
| `notes` | optional — kirim `null` atau omit untuk clear |

**Response (200):**

```json
{
  "data": {
    "id": "c0ffee00-...",
    "client_id": "a1111111-...",
    "translation_key": "button-save",
    "value": "Simpan Dokumen",
    "notes": "Updated phrasing per product team",
    "created_at": "2026-04-18T03:21:00Z",
    "created_by": "10000000-...",
    "updated_at": "2026-04-18T04:45:00Z",
    "updated_by": "10000000-..."
  },
  "message": "Translation override updated successfully"
}
```

**Errors:** 400 (invalid body/key/value), 401, 403, 404.

---

### 6. Delete Translation Override (admin)

```
DELETE /core/v1/admin/clients/:id/translation-overrides/:key
```

**Auth:** Bearer token + `claims.client_id == :id` (super_admin bypass)

**Effect:** Row di-hard delete. FE jatuh ke string default built-in di bootstrap berikutnya.

**Response (200):**

```json
{
  "data": null,
  "message": "Translation override deleted successfully"
}
```

**Errors:** 401, 403, 404.

---

## Example Scenarios

### A. FE bootstrap

FE tahu `slug` user (dari subdomain, config build, atau user preference). Sebelum render pertama:

```js
const slug = getClientSlug(); // "acme"
const res = await fetch(`/core/v1/translation-overrides?slug=${slug}`);
const { data } = await res.json();
i18n.addResourceBundle('id', 'overrides', data.translations, true, true);
```

### B. Super admin manage overrides untuk client tertentu

1. `GET /core/v1/admin/clients` → pilih client (lihat [clients.md](clients.md))
2. `GET /core/v1/admin/clients/{id}/translation-overrides` → lihat semua overrides
3. `POST /core/v1/admin/clients/{id}/translation-overrides` → tambah
4. `PUT /core/v1/admin/clients/{id}/translation-overrides/{key}` → edit
5. `DELETE /core/v1/admin/clients/{id}/translation-overrides/{key}` → reset

### C. Dua client, key sama, value beda

Client `acme` dan `globex` masing-masing POST `translation_key=button-save`. Tidak konflik — unique index di `(client_id, translation_key)` membolehkan.

- `acme` → "Simpan"
- `globex` → "Save Now"

Masing-masing FE bootstrap pakai slug-nya sendiri.

### D. Duplikat key dalam satu client

POST dengan `translation_key` yang sudah ada di client yang sama → **409 Conflict**. FE arahkan user untuk pakai "Edit" (PUT).

---

## Business Rules

| Rule | Keterangan |
|---|---|
| Per-client scope | Overrides selalu di-scope per client. Endpoint admin membutuhkan `client_id` di URL. |
| Public via slug | Bootstrap publik pakai `slug` karena FE belum login. Slug → client_id di-resolve di server. |
| Super admin only | Semua CRUD admin diproteksi role `super_admin`. Tidak ada self-service untuk client owner di v1. |
| Immutable key | Setelah create, `translation_key` tidak bisa di-rename (rename = DELETE + POST). |
| Value non-empty | `value` wajib ada isinya (DB CHECK + service layer). |
| Key format strict | `^[a-z0-9-]+$` — lowercase + digit + hyphen saja. |
| Hard delete | DELETE menghapus row permanen. Tidak ada soft-delete di v1. |
| Slug resolution | Public endpoint resolve slug → client_id. Unknown slug = 404 (tidak ada fallback). |
| Cascade pada client delete | FK `ON DELETE CASCADE` — kalau client dihapus, semua overrides-nya ikut. |

---

## Notes for FE Implementers

- **Slug discovery** — FE harus tahu slug client-nya sebelum bootstrap. Source umum: subdomain (`acme.tuai.id` → `acme`), build-time env var, atau user preference di localStorage.
- **Bootstrap strategy** — panggil public endpoint **sebelum** render pertama. Kalau gagal, fallback ke default string; jangan blocking.
- **Cache** — response publik boleh di-cache HTTP short-TTL.
- **Admin panel flow** — pilih client dari [admin/clients](clients.md) list dulu, baru navigate ke manage translations untuk client tersebut.
- **Client-side validation** — gunakan regex `^[a-z0-9-]+$` untuk `translation_key` supaya error 400 tidak perlu round-trip.
- **Conflict handling** — kalau POST balik 409, tampilkan tombol "Edit existing" yang navigate ke form edit key tersebut.

---

## Related

- [Clients Contract](clients.md) — tenant boundary; wajib baca untuk paham scope
- [Auth Contract](auth.md) — untuk dapat JWT dengan role `super_admin`
- Schema migration (client table): [`internal/database/migrations/core/000012_clients.up.sql`](../../../internal/database/migrations/core/000012_clients.up.sql)
- Schema migration (reshape): [`internal/database/migrations/core/000013_translation_overrides_per_client.up.sql`](../../../internal/database/migrations/core/000013_translation_overrides_per_client.up.sql)
- Module source: [`internal/modules/core/translation_overrides/`](../../../internal/modules/core/translation_overrides/)

---

**Last Updated:** 2026-04-18
