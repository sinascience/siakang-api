# User Management API Contract

**Version:** v1
**Base URL:** `/core/v1/users`
**Module:** Core User Management
**Last Updated:** 2026-04-17

---

## Overview

API untuk mengelola user accounts. Termasuk admin CRUD dan self-service profile management.

**Key Features:**
- User CRUD dengan soft delete
- Self-service profile (me endpoints)
- Password change
- Company assignment via batch sync (lihat [company.md](company.md#13-sync-user-companies-batch))
- **Branch access** — per-user branch scoping, inline di Create/Update atau via endpoint terpisah (lihat [Branch Assignment](#branch-assignment))
- RBAC — create/update/delete butuh permission `user-management`
- **Tenant visibility** — non-super-admin hanya melihat user dalam company tree-nya (lihat [Tenant Visibility Rules](#tenant-visibility-rules))

---

## User Object

```json
{
  "id": "550e8400-...",
  "email": "admin@tuai.com",
  "username": "admin",
  "full_name": "Admin Tuai",
  "phone": "+6281234567890",
  "avatar_url": null,
  "is_active": true,
  "is_email_verified": false,
  "role_name": "administrator",
  "companies": [
    {
      "company_id": "f24f27c8-5d80-40f6-9af7-13ca7baaae5a",
      "company_name": "PT Lathif OKe",
      "is_owner": true
    },
    {
      "company_id": "a1b2c3d4-...",
      "company_name": "PT Lathif Subsidiary",
      "is_owner": false
    }
  ],
  "branches": [
    {
      "branch_id": "b1c2d3e4-...",
      "branch_code": "CB-01",
      "branch_name": "Cabang Pusat",
      "company_id": "f24f27c8-5d80-40f6-9af7-13ca7baaae5a"
    }
  ],
  "last_login_at": "2026-04-01T10:00:00Z",
  "created_at": "2026-01-01T08:00:00Z",
  "updated_at": "2026-04-01T10:00:00Z"
}
```

| Field | Type | Keterangan |
|-------|------|------------|
| `id` | UUID | ID user |
| `email` | string | Email (unique) |
| `username` | string | Username (unique) |
| `full_name` | string / null | Nama lengkap |
| `phone` | string / null | Nomor telepon |
| `avatar_url` | string / null | URL foto profil |
| `is_active` | bool | Status aktif |
| `is_email_verified` | bool | Email sudah diverifikasi |
| `role_name` | string / null | Role pertama yang ditemukan dari company memberships (untuk display di UI) |
| `companies` | object[] / null | Daftar company yang user ikuti (lihat struktur di bawah) |
| `branches` | object[] / null | Daftar branch yang user punya akses (lihat struktur di bawah). Kosong/absent = user belum dibatasi ke branch manapun. |
| `last_login_at` | timestamp / null | Login terakhir |
| `created_at` | timestamp | Waktu dibuat |
| `updated_at` | timestamp | Waktu terakhir diubah |

### `companies[]` item

| Field | Type | Keterangan |
|-------|------|------------|
| `company_id` | UUID | ID company |
| `company_name` | string | Nama company |
| `is_owner` | bool | `true` jika user adalah `owner_id` dari company tersebut |

### `branches[]` item

| Field | Type | Keterangan |
|-------|------|------------|
| `branch_id` | UUID | ID branch |
| `branch_code` | string | Kode branch (mis. `CB-01`) |
| `branch_name` | string | Nama branch |
| `company_id` | UUID | Company yang memiliki branch tersebut |

> `role_name`, `companies`, dan `branches` di-populate otomatis oleh service (tidak tersimpan di tabel `core.users` — berasal dari join ke `core.company_users`, `core.roles`, dan `core.user_branches` + `core.branches`).

---

## Tenant Visibility Rules

Endpoint list & single-user read/mutate **otomatis di-scope** berdasarkan role caller:

| Role caller | Yang terlihat |
|---|---|
| `super_admin` | Semua user (tidak ada filter) |
| Selain super_admin | (1) User yang punya membership di **company saat ini** (dari JWT `company_id`) atau **descendant-nya** (ditelusuri rekursif via `parent_id`), ATAU (2) user yang `created_by` = caller (fallback untuk user yang baru dibuat dan belum di-attach ke company) |

**Properti penting:**
- Tree walk **satu arah ke bawah**. Admin di subsidiary B (parent = holding A) **tidak** bisa melihat user di holding A, walaupun A adalah parent-nya.
- Endpoint single-user (`GET/PUT/DELETE /users/:id`) yang target-nya di luar visibility scope akan mengembalikan **`404 User not found`**, bukan 403. Ini disengaja supaya eksistensi UUID di tenant lain tidak bocor.
- Non-super-admin yang belum switch ke company (`company_id` kosong di JWT) mendapat **`403 Company context required`** saat mengakses endpoint admin di bawah ini.
- `GET/PUT /users/me` dan `PUT /users/me/password` **tidak** terkena scope — user selalu bisa mengakses dirinya sendiri terlepas dari company memberships.

### Live verifier (anti-stale-JWT)

Selain check scope di atas, setiap request non-super-admin ke endpoint admin user juga melewati **live verifier** yang men-query database untuk memastikan:

1. Company di JWT (`company_id`) **masih ada** di `core.companies` (tidak `deleted_at`).
2. Company tersebut masih `is_active = true`.
3. Caller masih punya membership aktif di `core.company_users` untuk company tersebut (`is_active = true`, `deleted_at IS NULL`).

Kalau salah satu gagal — misal admin baru saja di-remove dari company atau company-nya di-soft-delete — request langsung ditolak dengan **`403 stale_company_context`** terlepas dari apakah JWT-nya masih valid secara kriptografis. Ini menutup celah di mana JWT lama tetap dipakai sampai expired.

Super_admin **tidak** terkena live verifier (mereka bisa hold `company_id` claim untuk tenant manapun secara desain).

---

## Admin Endpoints

### 1. List All Users

```
GET /core/v1/users
```

**Auth:** Bearer token
**Scope:** Di-scope otomatis — lihat [Tenant Visibility Rules](#tenant-visibility-rules).

**Query Parameters:**

| Param | Type | Default | Keterangan |
|-------|------|---------|------------|
| `page` | int | 1 | Halaman |
| `limit` | int | 10 | Max 100 |
| `search` | string | - | Cari di email, username, full_name |
| `is_active` | bool | - | Filter status aktif |

> Tidak ada parameter untuk override tenant scope dari client. Scope diambil dari JWT claims caller di server side.

**Response (200):**
```json
{
  "data": [
    {
      "id": "550e8400-...",
      "email": "admin@tuai.com",
      "username": "admin",
      "full_name": "Admin Tuai",
      "phone": null,
      "avatar_url": null,
      "is_active": true,
      "is_email_verified": false,
      "role_name": "administrator",
      "companies": [
        {
          "company_id": "f24f27c8-5d80-40f6-9af7-13ca7baaae5a",
          "company_name": "PT Lathif OKe",
          "is_owner": true
        }
      ],
      "branches": [
        {
          "branch_id": "b1c2d3e4-...",
          "branch_code": "CB-01",
          "branch_name": "Cabang Pusat",
          "company_id": "f24f27c8-5d80-40f6-9af7-13ca7baaae5a"
        }
      ],
      "last_login_at": "2026-04-01T10:00:00Z",
      "created_at": "2026-01-01T08:00:00Z",
      "updated_at": "2026-01-01T08:00:00Z"
    }
  ],
  "message": "Users retrieved successfully",
  "meta": {
    "pagination": {
      "page": 1,
      "limit": 10,
      "total": 7,
      "total_pages": 1
    }
  }
}
```

**Errors:**

| Status | Message | Error code |
|--------|---------|------------|
| 400 | Invalid query parameters | - |
| 403 | Company context required | - |
| 403 | Company context is no longer valid for this user | `stale_company_context` |

---

### 2. Get User by ID

```
GET /core/v1/users/:id
```

**Auth:** Bearer token
**Scope:** Target harus visible bagi caller — lihat [Tenant Visibility Rules](#tenant-visibility-rules).

**Response (200):** Single user object (lengkap dengan `role_name` dan `companies[]`).

**Errors:**

| Status | Message | Error code |
|--------|---------|------------|
| 400 | User ID is required | - |
| 403 | Company context required | - |
| 403 | Company context is no longer valid for this user | `stale_company_context` |
| 404 | User not found (tidak ada, sudah di-soft-delete, **atau** di luar tenant scope caller) | - |

---

### 3. Create User

```
POST /core/v1/users
```

**Auth:** Bearer token
**Permission:** `user-management:create`

**Request:**
```json
{
  "email": "newuser@tuai.com",
  "username": "newuser",
  "password": "password123",
  "full_name": "New User",
  "phone": "+6281234567890",
  "role_id": "00000000-0000-0000-0000-000000000003",
  "company_ids": ["company-uuid-1", "company-uuid-2"],
  "branch_ids": ["branch-uuid-1", "branch-uuid-2"]
}
```

| Field | Type | Required | Validasi |
|-------|------|----------|----------|
| `email` | string | Ya | Format email valid |
| `username` | string | Ya | 3-100 karakter |
| `password` | string | Ya | Min 8 karakter |
| `full_name` | string | Tidak | Max 255 karakter |
| `phone` | string | Tidak | Max 20 karakter |
| `role_id` | UUID | Tidak | Role untuk company memberships |
| `company_ids` | UUID[] | Tidak | Company yang akan di-assign ke user (lihat catatan di bawah) |
| `branch_ids` | UUID[] | Tidak | Branch yang bisa diakses user. Kosong/omit = tidak ada scoping branch (akses default mengikuti company). Non-super-admin dibatasi ke branch milik company scope-nya. |

> User dibuat dengan `is_active: true`, `is_email_verified: false`.
> Setiap user **harus** punya minimal 1 company. Insert ke `core.users`, `core.company_users`, dan `core.user_branches` dijalankan dalam **satu transaction** — kalau salah satu gagal, semuanya rollback dan tidak ada user "orphan" tanpa membership/akses yang ter-setengah.

**Tenant enforcement untuk non-super-admin:**

- `company_ids` dari request body akan **diabaikan** dan di-override oleh server menjadi `[current_company_id]` (dari JWT). Artinya non-super-admin hanya bisa meng-assign user baru ke company yang dia sedang masuki.
- Ini mencegah admin company A diam-diam membuat user di company B yang bukan miliknya.
- Caller harus sudah switch company (`company_id` tidak boleh kosong di JWT), kalau tidak akan return `403`.
- Hanya `super_admin` yang bebas mengirim `company_ids` ke company manapun.
- `branch_ids` **tetap boleh dikirim** oleh non-super-admin, tetapi setiap branch divalidasi harus milik `current_company_id` atau descendant-nya. Branch yang tidak ada → `400 Invalid branch_ids`; branch di luar scope → `403 Branch not in caller's scope`.

**Validasi `role_id` untuk non-super-admin:**

Server memvalidasi `role_id` (kalau dikirim) terhadap scope caller:

- Role **global** (`company_id IS NULL`) → diizinkan, **kecuali** role dengan code `super_admin`.
- Role **tenant-owned** (`company_id` ada) → harus persis sama dengan `current_company_id` caller.
- Role tidak ada / soft-deleted → ditolak.

Pelanggaran apapun → `403 Role not allowed for caller`. Super_admin bypass validasi ini.

**Response (201):** User object yang baru dibuat.

**Errors:**

| Status | Message | Keterangan |
|--------|---------|------------|
| 400 | Invalid request payload | Binding/validation gagal |
| 400 | `company_ids is required` | Super_admin tidak mengirim `company_ids` (non-super-admin di-handle otomatis oleh override) |
| 400 | `Invalid company_ids` | Salah satu `company_id` tidak ada / soft-deleted di `core.companies`. Transaction sudah di-rollback. |
| 400 | `Invalid branch_ids` | Salah satu `branch_id` tidak ada / soft-deleted di `core.branches`. Transaction sudah di-rollback. |
| 403 | Company context required | Non-super-admin belum switch company |
| 403 | Company context is no longer valid for this user | `stale_company_context` — caller di-remove dari company atau company di-soft-delete/deactivated sejak JWT diterbitkan |
| 403 | Role not allowed for caller | `role_id` di luar scope caller (lihat Validasi `role_id` di atas) |
| 403 | `Branch not in caller's scope` | Salah satu `branch_id` milik company di luar scope caller (non-super-admin). Transaction sudah di-rollback. |
| 409 | Email already exists | - |
| 409 | Username already exists | - |

---

### 4. Update User

```
PUT /core/v1/users/:id
```

**Auth:** Bearer token
**Permission:** `user-management:update`

**Request:** (semua field opsional)
```json
{
  "email": "updated@tuai.com",
  "username": "updateduser",
  "full_name": "Updated Name",
  "phone": "+6289876543210",
  "avatar_url": "https://example.com/avatar.jpg",
  "is_active": false,
  "role_id": "00000000-0000-0000-0000-000000000003",
  "company_ids": ["company-uuid-1", "company-uuid-2"],
  "branch_ids": ["branch-uuid-1"]
}
```

| Field | Type | Keterangan |
|-------|------|------------|
| `email` | string | Email baru (validasi unique) |
| `username` | string | Username baru (validasi unique) |
| `full_name` | string | Nama lengkap |
| `phone` | string | Nomor telepon |
| `avatar_url` | string | URL avatar (format URL valid) |
| `is_active` | bool | Toggle aktif/nonaktif |
| `role_id` | UUID | Role untuk company memberships baru |
| `company_ids` | UUID[] | Sync company memberships (add/remove). Company dimana user adalah owner tidak bisa di-remove. |
| `branch_ids` | UUID[] / omit | **Tri-state**: **omit** = biarkan branch access apa adanya. **`[]`** (array kosong) = cabut semua branch access user (untuk non-super-admin: hanya mencabut branch yang berada dalam scope caller). **Array berisi UUID** = sync full (add yang belum ada, remove yang tidak ada — untuk non-super-admin entry di luar scope tidak disentuh). |

> Password **tidak bisa** diubah dari endpoint ini. Gunakan change password.
> Jika `company_ids` diberikan, akan di-sync (add yang belum ada, remove yang tidak ada di list — kecuali owned companies).
> `branch_ids` ter-sync independen dari `company_ids` — payload boleh hanya mengirim salah satunya.

**Tenant enforcement untuk non-super-admin:**

- `company_ids` dari request body akan di-**drop** (dipaksa `nil`) oleh server. Non-super-admin **tidak bisa** mengubah membership user melalui endpoint ini. Alasannya: `SyncUserCompanies` mengganti seluruh set membership, jadi kalau diizinkan, admin company A bisa mengeluarkan (evict) user dari company B yang bukan miliknya.
- Non-super-admin juga tidak bisa update user yang di luar tenant scope-nya — akan return `404 User not found` (lihat [Tenant Visibility Rules](#tenant-visibility-rules)).
- `role_id` divalidasi sama seperti pada Create — lihat "Validasi `role_id`" di section Create.
- `branch_ids` **tetap diproses** untuk non-super-admin, tetapi: (1) setiap entry harus milik company scope caller atau descendant-nya (kalau tidak → `403 Branch not in caller's scope`), dan (2) branch access existing yang berada di **luar** scope caller tidak akan disentuh oleh sync — jadi admin company A tidak bisa mencabut akses branch company B dari user yang kebetulan terlihat lintas tenant. Super_admin tidak terkena batasan ini.
- Untuk mengelola company memberships per user, gunakan endpoint `PUT /core/v1/users/:user_id/companies` di [company.md](company.md) yang punya permission check sendiri (`company_users:update`).
- Untuk mengelola branch access per user secara terpisah, gunakan endpoint `PUT /core/v1/users/:user_id/branches` — lihat [Branch Assignment](#branch-assignment).

> Kalau super_admin mengirim `company_ids` dan salah satunya tidak ada di `core.companies`, request **fail** dengan `400 Invalid company_ids` (sebelumnya silently di-skip — lihat [company.md](company.md#sync-user-companies-batch) untuk semantic sync). Errors dari sync sekarang dipropagasi ke caller.

**Response (200):** Updated user object.

**Errors:**

| Status | Message | Keterangan |
|--------|---------|------------|
| 400 | User ID is required / Invalid request payload | - |
| 400 | `Invalid company_ids` | Salah satu `company_id` (super_admin only) tidak ada di `core.companies` |
| 400 | `Invalid branch_ids` | Salah satu `branch_id` tidak ada / soft-deleted di `core.branches` |
| 403 | Company context required | - |
| 403 | Company context is no longer valid for this user | `stale_company_context` |
| 403 | Role not allowed for caller | `role_id` di luar scope caller |
| 403 | `Branch not in caller's scope` | Salah satu `branch_id` milik company di luar scope caller (non-super-admin) |
| 404 | User not found (tidak ada, sudah dihapus, **atau** di luar tenant scope caller) | - |
| 409 | Email already exists | - |
| 409 | Username already exists | - |

---

### 5. Delete User

```
DELETE /core/v1/users/:id
```

**Auth:** Bearer token
**Permission:** `user-management:delete`
**Scope:** Target harus visible bagi caller — lihat [Tenant Visibility Rules](#tenant-visibility-rules).

**Response (200):**
```json
{
  "data": null,
  "message": "User deleted successfully"
}
```

**Errors:**

| Status | Message | Error code |
|--------|---------|------------|
| 400 | User ID is required | - |
| 403 | Company context required | - |
| 403 | Company context is no longer valid for this user | `stale_company_context` |
| 404 | User not found (tidak ada, sudah dihapus, **atau** di luar tenant scope caller) | - |

---

## Self-Service Endpoints

Endpoint untuk user mengelola profil sendiri (tidak butuh permission khusus, hanya butuh login).

### 6. Get Current User (Me)

```
GET /core/v1/users/me
```

**Auth:** Bearer token

**Response (200):** User object dari user yang sedang login (berdasarkan `user_id` di JWT).

---

### 7. Update Profile (Me)

```
PUT /core/v1/users/me
```

**Auth:** Bearer token

**Request:**
```json
{
  "full_name": "John Doe Updated",
  "phone": "+6281234567890",
  "avatar_url": "https://example.com/new-avatar.jpg"
}
```

| Field | Type | Keterangan |
|-------|------|------------|
| `full_name` | string | Max 255 karakter |
| `phone` | string | Max 20 karakter |
| `avatar_url` | string | Format URL valid, max 500 karakter |

> **Tidak bisa** mengubah email, username, atau is_active dari endpoint ini.

**Response (200):** Updated user object.

---

### 8. Change Password

```
PUT /core/v1/users/me/password
```

**Auth:** Bearer token

**Request:**
```json
{
  "current_password": "oldpassword123",
  "new_password": "newpassword456"
}
```

| Field | Type | Required | Validasi |
|-------|------|----------|----------|
| `current_password` | string | Ya | Password saat ini |
| `new_password` | string | Ya | Min 8 karakter |

**Response (200):**
```json
{
  "data": null,
  "message": "Password changed successfully"
}
```

**Errors:**

| Status | Message |
|--------|---------|
| 400 | Invalid current password |
| 404 | User not found |

---

## Company Assignment

Untuk assign user ke company (checkbox tree di halaman Users), gunakan endpoint berikut yang didokumentasikan di [company.md](company.md):

| Endpoint | Keterangan |
|----------|------------|
| `GET /core/v1/users/:user_id/companies` | Ambil list company IDs user |
| `PUT /core/v1/users/:user_id/companies` | Batch sync company memberships |

**Permission:** `company_users:update`

### Contoh Flow dari Frontend

**Create User:**
1. **Isi form** → nama, email, username, password, phone, role, centang company tree, centang branch (opsional)
2. **Save** → panggil `POST /users` dengan `role_id` + `company_ids` + `branch_ids` (semua dalam 1 request)

**Edit User:**
1. **Load form** → panggil `GET /users/:id` + `GET /users/:id/companies` + `GET /users/:id/branches`
2. **Company dengan `is_owner: true`** → disable checkbox (tidak bisa di-uncheck)
3. **Save** → panggil `PUT /users/:id` dengan `company_ids` + `branch_ids` (sync otomatis dalam 1 request)
   - Alternatif: panggil terpisah `PUT /users/:id` + `PUT /users/:id/companies` + `PUT /users/:id/branches`

---

## Branch Assignment

Scoping per-branch untuk user. `core.user_branches` adalah junction table antara `core.users` dan `core.branches`. Absent / list kosong = **tidak ada scoping branch khusus** untuk user tersebut (backend tidak otomatis memfilter resource berdasarkan `user_branches`; ini kebijakan caller/feature yang pakai). Yang backend jamin: API CRUD untuk mengelola list-nya + validasi scope lintas-tenant.

### 9. Get User's Branch Access

```
GET /core/v1/users/:id/branches
```

**Auth:** Bearer token

**Response (200):**
```json
{
  "data": ["b1c2d3e4-...", "c2d3e4f5-..."],
  "message": "User branches retrieved successfully"
}
```

> Returns array of branch UUIDs (plain strings, tidak nested object). Untuk detail code/name, baca dari `branches[]` di `GET /users/:id`.

**Errors:**

| Status | Message |
|--------|---------|
| 400 | User ID is required |
| 401 | Unauthorized |

---

### 10. Sync User's Branch Access

```
PUT /core/v1/users/:id/branches
```

**Auth:** Bearer token
**Permission:** `company_users:update` (re-use permission yang sama dengan sync companies — keduanya adalah "akses user ke resource parent")

**Request:**
```json
{
  "branch_ids": ["b1c2d3e4-...", "c2d3e4f5-..."]
}
```

| Field | Type | Required | Validasi |
|-------|------|----------|----------|
| `branch_ids` | UUID[] | **Ya** | `[]` (kosong) artinya cabut semua akses (subject to scope); setiap elemen harus UUID valid |

**Semantic sync:**

- Branch yang ada di `branch_ids` tapi belum ter-assign → ditambahkan.
- Branch yang sudah ter-assign tapi tidak ada di `branch_ids` → di-soft-delete.
- Untuk **non-super-admin**: assignment yang di luar company scope caller **tidak akan di-remove** — ini mencegah admin company A mencabut akses branch company B dari user yang terlihat lintas tenant.

**Scope enforcement:**

- Non-super-admin: setiap `branch_id` divalidasi milik company saat ini atau descendant-nya. Branch di luar scope → `403 Branch not in caller's scope`.
- Super_admin: bebas assign branch manapun.

**Response (200):**
```json
{
  "data": null,
  "message": "User branches synced successfully"
}
```

**Errors:**

| Status | Message | Error code |
|--------|---------|------------|
| 400 | User ID is required / Invalid request payload | - |
| 400 | `Invalid branch_ids` | Salah satu `branch_id` tidak ada / soft-deleted |
| 401 | Unauthorized | - |
| 403 | Company context required | Non-super-admin belum switch company |
| 403 | `Branch not in caller's scope` | Salah satu `branch_id` di luar scope caller |

---

## Permission Requirements Summary

| Endpoint | Permission | Tenant Scope | Catatan |
|----------|-----------|---|---|
| `GET /users` | Login saja | Di-scope | Live verifier aktif |
| `GET /users/:id` | Login saja | Di-scope | Live verifier aktif |
| `POST /users` | `user-management:create` | `company_ids` dipaksa ke current company; `branch_ids` divalidasi ke scope | Atomic via transaction; `role_id` + `branch_ids` divalidasi |
| `PUT /users/:id` | `user-management:update` | Di-scope; `company_ids` di-drop; `branch_ids` divalidasi ke scope | Sync errors dipropagasi; `role_id` + `branch_ids` divalidasi |
| `DELETE /users/:id` | `user-management:delete` | Di-scope | Live verifier aktif |
| `GET /users/me` | Login saja | **Tidak** di-scope | - |
| `PUT /users/me` | Login saja | **Tidak** di-scope | - |
| `PUT /users/me/password` | Login saja | **Tidak** di-scope | - |
| `GET /users/:id/companies` | Login saja | Lihat [company.md](company.md) | - |
| `PUT /users/:id/companies` | `company_users:update` | Lihat [company.md](company.md) | - |
| `GET /users/:id/branches` | Login saja | - | Return UUID list |
| `PUT /users/:id/branches` | `company_users:update` | Di-scope via `branch_ids` validation | Out-of-scope existing tidak disentuh |

> "Di-scope" = aturan di [Tenant Visibility Rules](#tenant-visibility-rules) di-apply. `super_admin` bypass semua scoping.
>
> "Live verifier aktif" = sebelum scope check, server re-query DB untuk memastikan caller masih member company-nya & company masih ada/aktif. Lihat [Live verifier (anti-stale-JWT)](#live-verifier-anti-stale-jwt). Stale → 403 `stale_company_context`.
>
> "Atomic via transaction" = `POST /users` membungkus insert `core.users`, `core.company_users`, dan `core.user_branches` dalam 1 transaction. Kalau salah satu gagal, semuanya rollback — tidak mungkin ada user "orphan" tanpa membership/akses setengah-setengah.
>
> "Sync errors dipropagasi" = errors dari `SyncUserCompanies` dan `SyncUserBranches` (mis. company/branch tidak ada, branch di luar scope) sekarang return ke caller dengan HTTP code yang sesuai, bukan di-log diam-diam.

> Permission format di JWT: `"user-management:create"` (bukan `core.users:create`).
> Cek di frontend: `permissions.includes("user-management:create")`

---

**Last Updated:** 2026-04-17
