# Branch Management API Contract

**Version:** v1
**Base URL:** `/core/v1/branches`
**Module:** Core Branch Management
**Last Updated:** 2026-04-15

---

## Overview

API untuk mengelola branch (cabang) dalam sebuah company. Setiap branch terikat pada company yang aktif di JWT (via `CompanyContext` middleware).

**Key Features:**
- Branch CRUD scoped per company
- Kode branch unik per company (auto-generate bila tidak diisi, prefix `CB`)
- Default branch per company (hanya 1)
- Default branch tidak bisa dihapus
- Soft delete

---

## Branch Object

```json
{
  "id": "30000000-0000-0000-0000-000000000001",
  "company_id": "20000000-0000-0000-0000-000000000001",
  "code": "CB0001",
  "name": "Tuai Pusat",
  "logo_url": null,
  "sort": 0,
  "is_default": true,
  "is_active": true,
  "created_by": "10000000-0000-0000-0000-000000000001",
  "created_at": "2026-01-01T08:00:00Z",
  "updated_at": "2026-01-01T08:00:00Z"
}
```

| Field | Type | Keterangan |
|-------|------|------------|
| `id` | UUID | ID branch |
| `company_id` | UUID | Company pemilik branch |
| `code` | string | Kode unik branch per company (auto-generate `CB0001`, `CB0002`, ... bila tidak diisi) |
| `name` | string | Nama branch |
| `logo_url` | string / null | URL logo branch |
| `sort` | int | Urutan display |
| `is_default` | bool | Branch default company (hanya 1 per company) |
| `is_active` | bool | Status aktif |
| `created_by` | UUID / null | User yang membuat |
| `created_at` | timestamp | Waktu dibuat |
| `updated_at` | timestamp | Waktu terakhir diubah |

---

## Endpoints

### 1. List All Branches

```
GET /core/v1/branches
```

**Auth:** Bearer token + CompanyContext
**Middleware:** `JWTAuth()`, `CompanyContext()`

Mengembalikan branches milik company yang aktif di JWT.

**Query Parameters:**

| Param | Type | Default | Keterangan |
|-------|------|---------|------------|
| `page` | int | 1 | Halaman (min 1) |
| `limit` | int | 10 | Items per page (min 1, max 100) |
| `search` | string | - | Cari di name (max 255 char) |
| `is_active` | bool | - | Filter status aktif |

**Response (200):**
```json
{
  "data": [
    {
      "id": "30000000-0000-0000-0000-000000000001",
      "company_id": "20000000-0000-0000-0000-000000000001",
      "code": "CB0001",
      "name": "Tuai Pusat",
      "logo_url": null,
      "sort": 0,
      "is_default": true,
      "is_active": true,
      "created_by": "10000000-0000-0000-0000-000000000001",
      "created_at": "2026-01-01T08:00:00Z",
      "updated_at": "2026-01-01T08:00:00Z"
    },
    {
      "id": "30000000-0000-0000-0000-000000000002",
      "company_id": "20000000-0000-0000-0000-000000000001",
      "code": "CB0002",
      "name": "Tuai Cabang Sudirman",
      "logo_url": null,
      "sort": 1,
      "is_default": false,
      "is_active": true,
      "created_by": "10000000-0000-0000-0000-000000000001",
      "created_at": "2026-01-01T08:00:00Z",
      "updated_at": "2026-01-01T08:00:00Z"
    }
  ],
  "message": "Branches retrieved successfully",
  "meta": {
    "pagination": {
      "page": 1,
      "limit": 10,
      "total": 2,
      "total_pages": 1
    }
  }
}
```

> Sorting default: `is_default DESC`, `sort ASC`, `name ASC` (default branch selalu muncul pertama).

**Errors:**

| Status | Message |
|--------|---------|
| 400 | Invalid query parameters |
| 401 | Unauthorized |

---

### 2. Get Branch by ID

```
GET /core/v1/branches/:id
```

**Auth:** Bearer token + CompanyContext

**Response (200):** Single branch object.
```json
{
  "data": { "...": "branch object" },
  "message": "Branch retrieved successfully"
}
```

**Errors:**

| Status | Message |
|--------|---------|
| 400 | Branch ID is required |
| 404 | Branch not found |

---

### 3. Create Branch

```
POST /core/v1/branches
```

**Auth:** Bearer token + CompanyContext
**Permission:** `branches:create`

**Request:**
```json
{
  "code": "CB-JKT-01",
  "name": "Cabang Baru",
  "logo_url": "https://example.com/logo.png",
  "sort": 2,
  "is_default": false
}
```

| Field | Type | Required | Validasi |
|-------|------|----------|----------|
| `code` | string | Tidak | Max 50 karakter. Jika kosong/tidak dikirim → auto-generate `CB0001`, `CB0002`, ... per company |
| `name` | string | Ya | 2-255 karakter |
| `logo_url` | string | Tidak | Format URL valid, max 500 karakter |
| `sort` | int | Tidak | Min 0, default 0 |
| `is_default` | bool | Tidak | Default `false` |

> Jika `is_default: true`, branch default lama akan otomatis di-clear.
> `company_id` diambil otomatis dari JWT (CompanyContext).
> `code` unik per `company_id`. Spasi di awal/akhir akan di-trim.

**Response (201):**
```json
{
  "data": { "...": "branch object" },
  "message": "Branch created successfully"
}
```

**Errors:**

| Status | Message |
|--------|---------|
| 400 | Invalid request payload |
| 401 | Unauthorized |
| 403 | Forbidden (missing `branches:create`) |
| 409 | Branch code already exists |

---

### 4. Update Branch

```
PUT /core/v1/branches/:id
```

**Auth:** Bearer token + CompanyContext
**Permission:** `branches:update`

**Request:** (semua field opsional)
```json
{
  "code": "CB-JKT-02",
  "name": "Cabang Updated",
  "logo_url": "https://example.com/new-logo.png",
  "sort": 1,
  "is_default": true,
  "is_active": false
}
```

| Field | Type | Keterangan |
|-------|------|------------|
| `code` | string | Max 50 karakter, tidak boleh string kosong setelah di-trim |
| `name` | string | 2-255 karakter |
| `logo_url` | string | Format URL valid, max 500 karakter |
| `sort` | int | Min 0 |
| `is_default` | bool | Set sebagai default (auto clear default lama). Tidak bisa di-unset via field ini |
| `is_active` | bool | Toggle aktif/nonaktif |

**Response (200):**
```json
{
  "data": { "...": "branch object" },
  "message": "Branch updated successfully"
}
```

**Errors:**

| Status | Message |
|--------|---------|
| 400 | Invalid request payload (termasuk `code` kosong setelah di-trim) |
| 401 | Unauthorized |
| 403 | Forbidden (missing `branches:update`) |
| 404 | Branch not found |
| 409 | Branch code already exists |

---

### 5. Delete Branch

```
DELETE /core/v1/branches/:id
```

**Auth:** Bearer token + CompanyContext
**Permission:** `branches:delete`

Soft delete — record tetap ada di DB dengan `deleted_at` terisi.

**Response (200):**
```json
{
  "data": null,
  "message": "Branch deleted successfully"
}
```

**Errors:**

| Status | Message |
|--------|---------|
| 401 | Unauthorized |
| 403 | Cannot delete default branch / missing `branches:delete` |
| 404 | Branch not found |

---

## Business Rules

| Rule | Keterangan |
|------|------------|
| 1 default per company | Hanya boleh 1 branch dengan `is_default: true` per company (enforced di DB + service) |
| Auto-clear default | Set `is_default: true` pada branch baru/update otomatis clear default lama |
| Default tidak bisa dihapus | Branch dengan `is_default: true` tidak bisa di-delete (return 403) |
| Code unik per company | `code` harus unik dalam satu `company_id` (unique constraint di DB) |
| Auto-generate code | Bila `code` tidak dikirim saat create, sistem generate `CB0001`, `CB0002`, ... dengan retry saat race condition |
| Scoped per company | Semua operasi otomatis di-scope ke `company_id` dari JWT |

---

## Permission Requirements Summary

| Endpoint | Permission |
|----------|-----------|
| `GET /branches` | Login + CompanyContext |
| `GET /branches/:id` | Login + CompanyContext |
| `POST /branches` | `branches:create` |
| `PUT /branches/:id` | `branches:update` |
| `DELETE /branches/:id` | `branches:delete` |

> Permission format di JWT: `"branches:create"` (bukan `core.branches:create`).
> Cek di frontend: `permissions.includes("branches:create")`

---

**Last Updated:** 2026-04-15
