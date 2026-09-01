# Company Management API Contract

**Version:** v1
**Base URL:** `/core/v1/companies`
**Module:** Core Company Management
**Last Updated:** 2026-04-17

---

## Overview

API untuk mengelola companies dengan **hierarchical structure** (holding → subsidiary). Termasuk company CRUD, hierarchy traversal, dan user membership management.

**Key Features:**
- Hierarchical company structure (parent-child)
- Company types: `holding`, `subsidiary`
- Owner management (super_admin can transfer ownership)
- User membership (add/remove/sync users to companies)
- Scoped access — non-super admin hanya lihat company sendiri + descendants
- Soft delete

---

## Company Object

```json
{
  "id": "20000000-0000-0000-0000-000000000001",
  "parent_id": null,
  "name": "PT Tuai Indonesia",
  "type": "holding",
  "logo_url": null,
  "owner_id": "10000000-0000-0000-0000-000000000001",
  "owner_name": "Super Admin",
  "sort": 0,
  "is_active": true,
  "created_by": "10000000-0000-0000-0000-000000000001",
  "created_at": "2026-01-01T08:00:00Z",
  "updated_at": "2026-01-01T08:00:00Z"
}
```

| Field | Type | Keterangan |
|-------|------|------------|
| `id` | UUID | ID company |
| `parent_id` | UUID / null | Parent company (`null` = root/holding) |
| `name` | string | Nama company |
| `type` | string | `holding` atau `subsidiary` |
| `logo_url` | string / null | URL logo |
| `owner_id` | UUID | User pemilik company |
| `owner_name` | string / null | Nama owner (join dari users) |
| `sort` | int | Urutan display |
| `is_active` | bool | Status aktif |
| `created_by` | UUID / null | User yang membuat |
| `created_at` | timestamp | Waktu dibuat |
| `updated_at` | timestamp | Waktu terakhir diubah |

---

## Company Endpoints

### 1. List All Companies

```
GET /core/v1/companies
```

**Auth:** Bearer token

**Behavior:**
- **Super admin:** Melihat semua companies
- **Non-super admin:** Hanya melihat company aktif + descendants-nya

**Query Parameters:**

| Param | Type | Default | Keterangan |
|-------|------|---------|------------|
| `page` | int | 1 | Halaman |
| `limit` | int | 10 | Max 100 |
| `search` | string | - | Cari di name |
| `parent_id` | UUID | - | Filter by parent company |
| `type` | string | - | Filter: `holding` / `subsidiary` |
| `is_active` | bool | - | Filter status aktif |

**Response (200):**
```json
{
  "data": [
    {
      "id": "20000000-0000-0000-0000-000000000001",
      "parent_id": null,
      "name": "PT Tuai Indonesia",
      "type": "holding",
      "logo_url": null,
      "owner_id": "10000000-0000-0000-0000-000000000001",
      "owner_name": "Super Admin",
      "sort": 0,
      "is_active": true,
      "created_by": "10000000-0000-0000-0000-000000000001",
      "created_at": "2026-01-01T08:00:00Z",
      "updated_at": "2026-01-01T08:00:00Z"
    }
  ],
  "message": "Companies retrieved successfully",
  "meta": {
    "pagination": {
      "page": 1,
      "limit": 10,
      "total": 5,
      "total_pages": 1
    }
  }
}
```

---

### 2. Get Company by ID

```
GET /core/v1/companies/:id
```

**Auth:** Bearer token

**Response (200):** Single company object.

**Errors:**

| Status | Message |
|--------|---------|
| 404 | Company not found |

---

### 3. Create Company

```
POST /core/v1/companies
```

**Auth:** Bearer token

**Request:**
```json
{
  "parent_id": "20000000-0000-0000-0000-000000000001",
  "name": "Tuai Bali",
  "type": "subsidiary",
  "logo_url": "https://example.com/logo.png",
  "owner_id": "10000000-0000-0000-0000-000000000002"
}
```

| Field | Type | Required | Validasi |
|-------|------|----------|----------|
| `parent_id` | UUID | Tidak | Parent company ID |
| `name` | string | Ya | 2-255 karakter |
| `type` | string | Ya | `holding` / `subsidiary` |
| `logo_url` | string | Tidak | Format URL valid, max 500 karakter |
| `owner_id` | UUID | Tidak | Hanya super_admin yang bisa assign owner lain. Default: creator. |

> Owner otomatis ditambahkan sebagai member company. Jika owner belum punya primary company, company ini akan jadi primary-nya.

**Response (201):** Company object yang baru dibuat.

**Errors:**

| Status | Message |
|--------|---------|
| 400 | Invalid request payload |
| 400 | Parent company not found |
| 403 | Only super_admin can assign a different owner |

---

### 4. Update Company

```
PUT /core/v1/companies/:id
```

**Auth:** Bearer token
**Permission:** `companies:update`

**Request:** (semua field opsional)
```json
{
  "name": "Tuai Bali Updated",
  "type": "subsidiary",
  "logo_url": "https://example.com/new-logo.png",
  "owner_id": "10000000-0000-0000-0000-000000000003",
  "sort": 1,
  "is_active": false
}
```

| Field | Type | Keterangan |
|-------|------|------------|
| `name` | string | 2-255 karakter |
| `type` | string | `holding` / `subsidiary` |
| `logo_url` | string | Format URL valid, max 500 karakter |
| `owner_id` | UUID | Transfer ownership (hanya super_admin) |
| `sort` | int | Urutan display (min 0) |
| `is_active` | bool | Toggle aktif/nonaktif |

> **Owner transfer:** Hanya super_admin. New owner otomatis ditambahkan sebagai member jika belum.

**Response (200):** Updated company object.

**Errors:**

| Status | Message |
|--------|---------|
| 403 | Only super_admin can transfer ownership |
| 404 | Company not found |

---

### 5. Delete Company

```
DELETE /core/v1/companies/:id
```

**Auth:** Bearer token
**Permission:** `companies:delete`

**Response (200):**
```json
{
  "data": null,
  "message": "Company deleted successfully"
}
```

**Errors:**

| Status | Message |
|--------|---------|
| 404 | Company not found |

---

### 6. List Deleted Companies (Trash)

```
GET /core/v1/companies/trash
```

**Auth:** Bearer token
**Permission:** `companies:delete`

**Query Parameters:**

| Param | Type | Default | Keterangan |
|-------|------|---------|------------|
| `page` | int | 1 | Halaman |
| `limit` | int | 10 | Max 100 |
| `search` | string | - | Cari di name |

**Response (200):**
```json
{
  "data": [
    {
      "id": "20000000-0000-0000-0000-000000000001",
      "parent_id": null,
      "name": "PT Tuai Indonesia",
      "type": "holding",
      "logo_url": null,
      "owner_id": "10000000-0000-0000-0000-000000000001",
      "owner_name": "Super Admin",
      "sort": 0,
      "is_active": true,
      "created_by": "10000000-0000-0000-0000-000000000001",
      "created_at": "2026-01-01T08:00:00Z",
      "updated_at": "2026-01-01T08:00:00Z"
    }
  ],
  "message": "Deleted companies retrieved successfully",
  "meta": {
    "pagination": {
      "page": 1,
      "limit": 10,
      "total": 1,
      "total_pages": 1
    }
  }
}
```

---

### 7. Restore Deleted Company

```
PATCH /core/v1/companies/:id/restore
```

**Auth:** Bearer token
**Permission:** `companies:delete`

**Response (200):**
```json
{
  "data": {
    "id": "20000000-0000-0000-0000-000000000001",
    "parent_id": null,
    "name": "PT Tuai Indonesia",
    "type": "holding",
    "logo_url": null,
    "owner_id": "10000000-0000-0000-0000-000000000001",
    "owner_name": "Super Admin",
    "sort": 0,
    "is_active": true,
    "created_by": "10000000-0000-0000-0000-000000000001",
    "created_at": "2026-01-01T08:00:00Z",
    "updated_at": "2026-04-17T10:00:00Z"
  },
  "message": "Company restored successfully"
}
```

**Errors:**

| Status | Message |
|--------|---------|
| 404 | Company not found |

---

## Hierarchy Endpoints

### 8. Get Children

```
GET /core/v1/companies/:id/children
```

**Auth:** Bearer token

Mengembalikan **direct children** (1 level ke bawah) dari company.

**Response (200):**
```json
{
  "data": [
    {
      "id": "20000000-0000-0000-0000-000000000011",
      "parent_id": "20000000-0000-0000-0000-000000000001",
      "name": "Tuai Jakarta",
      "type": "subsidiary",
      "logo_url": null,
      "owner_id": "10000000-0000-0000-0000-000000000002",
      "owner_name": null,
      "sort": 0,
      "is_active": true,
      "created_by": null,
      "created_at": "2026-01-01T08:00:00Z",
      "updated_at": "2026-01-01T08:00:00Z"
    }
  ],
  "message": "Children retrieved successfully"
}
```

**Errors:**

| Status | Message |
|--------|---------|
| 404 | Company not found |

---

### 9. Get Ancestors

```
GET /core/v1/companies/:id/ancestors
```

**Auth:** Bearer token

Mengembalikan **semua ancestor** (parent chain sampai root) dari company.

**Response (200):** Array of company objects, diurutkan dari root ke parent terdekat.

**Errors:**

| Status | Message |
|--------|---------|
| 404 | Company not found |

---

## Company User Management

### 10. List Company Users

```
GET /core/v1/companies/:id/users
```

**Auth:** Bearer token

**Query Parameters:**

| Param | Type | Default | Keterangan |
|-------|------|---------|------------|
| `page` | int | 1 | Halaman |
| `limit` | int | 10 | Max 100 |
| `search` | string | - | Cari di email, username, full_name |
| `is_active` | bool | - | Filter status aktif |

**Response (200):**
```json
{
  "data": [
    {
      "id": "cu-uuid-...",
      "company_id": "20000000-0000-0000-0000-000000000001",
      "user_id": "10000000-0000-0000-0000-000000000001",
      "role_id": "00000000-0000-0000-0000-000000000001",
      "role_name": "Super Administrator",
      "role_code": "super_admin",
      "user_email": "admin@tuai.com",
      "user_username": "admin",
      "user_full_name": "Super Admin",
      "is_primary": true,
      "is_active": true,
      "joined_at": "2026-01-01T08:00:00Z",
      "created_at": "2026-01-01T08:00:00Z"
    }
  ],
  "message": "Users retrieved successfully",
  "meta": {
    "pagination": {
      "page": 1,
      "limit": 10,
      "total": 3,
      "total_pages": 1
    }
  }
}
```

**Errors:**

| Status | Message |
|--------|---------|
| 404 | Company not found |

---

### 11. Add User to Company

```
POST /core/v1/companies/:id/users
```

**Auth:** Bearer token
**Permission:** `company_users:create`

**Request:**
```json
{
  "user_id": "10000000-0000-0000-0000-000000000005",
  "role_id": "00000000-0000-0000-0000-000000000003"
}
```

| Field | Type | Required | Keterangan |
|-------|------|----------|------------|
| `user_id` | UUID | Ya | User yang akan ditambahkan |
| `role_id` | UUID | Tidak | Role dalam company |

**Response (201):** Company user object.

**Errors:**

| Status | Message |
|--------|---------|
| 404 | Company not found |
| 409 | User is already a member |

---

### 12. Update Company User

```
PUT /core/v1/companies/:id/users/:user_id
```

**Auth:** Bearer token
**Permission:** `company_users:update`

**Request:** (semua field opsional)
```json
{
  "role_id": "00000000-0000-0000-0000-000000000002",
  "is_active": true,
  "is_primary": true
}
```

| Field | Type | Keterangan |
|-------|------|------------|
| `role_id` | UUID | Ganti role user dalam company |
| `is_active` | bool | Toggle membership aktif (owner tidak bisa di-deactivate) |
| `is_primary` | bool | Set sebagai primary company user (auto clear primary lama) |

**Response (200):** Updated company user object.

**Errors:**

| Status | Message |
|--------|---------|
| 403 | Cannot deactivate company owner |
| 404 | Membership not found |

---

### 13. Remove User from Company

```
DELETE /core/v1/companies/:id/users/:user_id
```

**Auth:** Bearer token
**Permission:** `company_users:delete`

**Response (200):**
```json
{
  "data": null,
  "message": "User removed successfully"
}
```

**Errors:**

| Status | Message |
|--------|---------|
| 403 | Cannot remove company owner |
| 404 | Company not found |
| 404 | Membership not found |

---

## User-Company Endpoints

Endpoint ini berada di bawah `/core/v1/users`, digunakan untuk manage company memberships dari sisi user (misalnya checkbox tree di halaman Users).

### 14. Get User's Companies

```
GET /core/v1/users/:id/companies
```

**Auth:** Bearer token

Mengembalikan list company IDs yang user ikuti, beserta flag ownership.

**Response (200):**
```json
{
  "data": [
    {
      "company_id": "20000000-0000-0000-0000-000000000001",
      "is_owner": true
    },
    {
      "company_id": "20000000-0000-0000-0000-000000000011",
      "is_owner": false
    }
  ],
  "message": "User companies retrieved successfully"
}
```

> `is_owner: true` → checkbox harus di-disable di frontend (tidak bisa di-uncheck).

---

### 15. Sync User Companies (Batch)

```
PUT /core/v1/users/:id/companies
```

**Auth:** Bearer token
**Permission:** `company_users:update`

**Request:**
```json
{
  "company_ids": [
    "20000000-0000-0000-0000-000000000001",
    "20000000-0000-0000-0000-000000000011",
    "20000000-0000-0000-0000-000000000012"
  ],
  "role_id": "00000000-0000-0000-0000-000000000003"
}
```

| Field | Type | Required | Keterangan |
|-------|------|----------|------------|
| `company_ids` | UUID[] | Ya | List company IDs yang diinginkan |
| `role_id` | UUID | Tidak | Role untuk membership baru / update existing |

**Behavior:**
- Company yang belum ada membership → **ditambahkan**
- Company yang ada di list → role di-update jika `role_id` diberikan
- Company yang **tidak** ada di list → **dihapus** (kecuali user adalah owner)

**Response (200):**
```json
{
  "data": null,
  "message": "User companies synced successfully"
}
```

---

## Company Types

| Type | Keterangan |
|------|------------|
| `holding` | Top-level parent company |
| `subsidiary` | Child company (di bawah holding) |

---

## Permission Requirements Summary

| Endpoint | Permission |
|----------|-----------|
| `GET /companies` | Login saja (scoped by role) |
| `GET /companies/:id` | Login saja |
| `POST /companies` | Login saja |
| `PUT /companies/:id` | `companies:update` |
| `DELETE /companies/:id` | `companies:delete` |
| `GET /companies/trash` | `companies:delete` |
| `PATCH /companies/:id/restore` | `companies:delete` |
| `GET /companies/:id/children` | Login saja |
| `GET /companies/:id/ancestors` | Login saja |
| `GET /companies/:id/users` | Login saja |
| `POST /companies/:id/users` | `company_users:create` |
| `PUT /companies/:id/users/:user_id` | `company_users:update` |
| `DELETE /companies/:id/users/:user_id` | `company_users:delete` |
| `GET /users/:id/companies` | Login saja |
| `PUT /users/:id/companies` | `company_users:update` |

> Permission format di JWT: `"companies:update"` (bukan `core.companies:update`).
> Cek di frontend: `permissions.includes("companies:update")`

---

**Last Updated:** 2026-04-17
