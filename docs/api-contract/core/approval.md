# Approval System API Contract

**Version:** v1
**Base URL:** `/core/v1/approval-configs`, `/core/v1/approval-requests`
**Module:** Core - Approval (generik, dipakai semua feature finance)
**Last Updated:** 2026-04-18

---

## Overview

Approval system adalah modul generik yang dipakai bersama oleh semua feature yang butuh alur approval multi-level (journal entry, cash in, cash out, transfer, expense, invoice, a1_budget, a2_budget, a3_budget). Dua konsep utama:

1. **Approval Config** — definisi alur approval per `(company, branch, feature_key)`. Admin mendefinisikan level berurutan, setiap level punya 1 role + daftar user yang boleh approve.
2. **Approval Request** — instance runtime per dokumen (polymorphic via `reff_type` + `reff_id`). Dibuat otomatis saat user Submit dokumen, menyimpan snapshot role & approver supaya audit trail tahan terhadap perubahan config di kemudian hari.

**Key Features:**

- Konfigurasi **per cabang per feature** (Opsi B: branch-aware, config wajib disetup tiap cabang)
- Multi-level approval (0..N levels)
- 0 levels = auto-approve
- Snapshot role name & eligible approver saat submit (history-friendly)
- Status denormalized di dokumen target untuk performa list page
- Inbox approver: "yang menunggu action saya"
- Approval actions per level: `approve`, `reject` (skip semua level sisa)
- `cancel` oleh submitter selama masih `waiting`

---

## Feature Keys

`feature_key` adalah identifier logis yang dipakai untuk memisahkan config per jenis dokumen. Nilai yang terdaftar:

| feature_key | Untuk | reff_type |
|---|---|---|
| `journal_entry` | Jurnal umum | `finance.journal_entries` |
| `cash_in` | Kas masuk (receipt) | `finance.cash_transactions` |
| `cash_out` | Kas keluar (disbursement) | `finance.cash_transactions` |
| `transfer` | Transfer antar akun | `finance.fund_transfers` |
| `expense` | Biaya / expense | `finance.expenses` _(planned)_ |
| `invoice` | Invoice penjualan | `finance.invoices` _(planned)_ |
| `a1_budget` | Proyeksi budget operating fund (A1) | `finance.a1_budget_projections` |
| `a2_budget` | Budget per 4 ARCA (formation/aged/apostolic/foundation) | `finance.a2_budget_arcae` |
| `a3_budget` | Operating Fund Curia — two-year comparison budget | `finance.a3_operating_fund_curia` |

> `feature_key` divalidasi di server (`domain.IsKnownFeatureKey`). Nilai di luar list akan ditolak dengan `400`.
> Untuk detail per feature, lihat kontrak modul terkait:
> - [cash-transactions.md](../finance/cash-transactions.md) untuk `cash_in` / `cash_out`
> - [journal-entries.md](../finance/journal-entries.md) untuk `journal_entry`
> - [fund-transfers.md](../finance/fund-transfers.md) untuk `transfer`
> - [a1-budget-projection.md](../finance/a1-budget-projection.md) untuk `a1_budget`
> - [a2-budget-arcae.md](../finance/a2-budget-arcae.md) untuk `a2_budget`
> - [a3-operating-fund-curia.md](../finance/a3-operating-fund-curia.md) untuk `a3_budget`

---

## Approval Config Object

```json
{
  "id": "40000000-0000-0000-0000-000000000001",
  "company_id": "20000000-0000-0000-0000-000000000001",
  "branch_id": "30000000-0000-0000-0000-000000000001",
  "feature_key": "journal_entry",
  "is_active": true,
  "levels": [
    {
      "id": "40000000-0000-0000-0000-000000000101",
      "level": 1,
      "role_id": "50000000-0000-0000-0000-000000000003",
      "approver_user_ids": [
        "10000000-0000-0000-0000-000000000003"
      ]
    },
    {
      "id": "40000000-0000-0000-0000-000000000102",
      "level": 2,
      "role_id": "50000000-0000-0000-0000-000000000002",
      "approver_user_ids": [
        "10000000-0000-0000-0000-000000000002"
      ]
    }
  ],
  "created_at": "2026-04-01T08:00:00Z",
  "updated_at": "2026-04-01T08:00:00Z"
}
```

| Field | Type | Keterangan |
|---|---|---|
| `id` | UUID | ID config |
| `company_id` | UUID | Scope company |
| `branch_id` | UUID | Scope cabang (wajib, unique per feature) |
| `feature_key` | string | Jenis dokumen yang di-gate |
| `is_active` | bool | Kalau `false`, submit akan gagal dengan "not configured" |
| `levels` | array | Ordered, level 1..N |
| `levels[].level` | int | 1..N, contiguous |
| `levels[].role_id` | UUID | Role yang memegang level ini (referensi `core.roles`) |
| `levels[].approver_user_ids` | UUID[] | Subset user di role tsb yang dipilih admin |

> Level 0 (empty `levels`) = auto-approve: dokumen langsung jadi `posted` saat submit.

---

## Approval Request Object

```json
{
  "id": "60000000-0000-0000-0000-000000000001",
  "company_id": "20000000-0000-0000-0000-000000000001",
  "branch_id": "30000000-0000-0000-0000-000000000001",
  "feature_key": "journal_entry",
  "reff_type": "finance.journal_entries",
  "reff_id": "70000000-0000-0000-0000-000000000010",
  "status": "waiting",
  "current_level": 2,
  "total_levels": 2,
  "submitted_at": "2026-04-08T09:00:00Z",
  "submitted_by": "10000000-0000-0000-0000-000000000005",
  "finalized_at": null,
  "finalized_by": null,
  "reject_reason": null,
  "levels": [
    {
      "id": "60000000-0000-0000-0000-000000000101",
      "level": 1,
      "role_id": "50000000-0000-0000-0000-000000000003",
      "role_name": "Finance Head",
      "eligible_approver_ids": ["10000000-0000-0000-0000-000000000003"],
      "status": "approved",
      "action_by": "10000000-0000-0000-0000-000000000003",
      "action_at": "2026-04-08T10:00:00Z",
      "comment": "OK"
    },
    {
      "id": "60000000-0000-0000-0000-000000000102",
      "level": 2,
      "role_id": "50000000-0000-0000-0000-000000000002",
      "role_name": "Treasurer",
      "eligible_approver_ids": ["10000000-0000-0000-0000-000000000002"],
      "status": "pending",
      "action_by": null,
      "action_at": null,
      "comment": null
    }
  ]
}
```

| Field | Type | Keterangan |
|---|---|---|
| `id` | UUID | ID request |
| `company_id` | UUID | Scope company |
| `branch_id` | UUID | Snapshot dari dokumen (untuk filter cepat) |
| `feature_key` | string | Jenis dokumen |
| `reff_type` | string | Tabel target, mis. `finance.journal_entries` |
| `reff_id` | UUID | PK row target |
| `status` | enum | `waiting` \| `approved` \| `rejected` \| `cancelled` |
| `current_level` | int \| null | Level yang sedang menunggu; `null` kalau final |
| `total_levels` | int | Snapshot N saat submit |
| `submitted_at` | timestamp | Saat submit |
| `submitted_by` | UUID | User submitter |
| `finalized_at` | timestamp \| null | Saat approved/rejected/cancelled |
| `finalized_by` | UUID \| null | User yang melakukan aksi final |
| `reject_reason` | string \| null | Alasan reject/cancel |
| `levels` | array | Audit trail per level |
| `levels[].role_name` | string | Snapshot nama role saat submit |
| `levels[].eligible_approver_ids` | UUID[] | Snapshot daftar approver saat submit |
| `levels[].status` | enum | `pending` \| `approved` \| `rejected` \| `skipped` |
| `levels[].action_by` | UUID \| null | User yang action |
| `levels[].action_at` | timestamp \| null | Waktu action |
| `levels[].comment` | string \| null | Comment / reject reason |

---

# Part 1 — Approval Configs

## Endpoints

### 1.1 List Approval Configs

```
GET /core/v1/approval-configs
```

**Auth:** Bearer token + CompanyContext

**Query Parameters:**

| Param | Type | Default | Keterangan |
|---|---|---|---|
| `branch_id` | UUID | - | Filter per cabang |
| `feature_key` | string | - | Filter per feature |

**Response (200):**
```json
{
  "data": [
    {
      "id": "40000000-0000-0000-0000-000000000001",
      "company_id": "20000000-0000-0000-0000-000000000001",
      "branch_id": "30000000-0000-0000-0000-000000000001",
      "feature_key": "journal_entry",
      "is_active": true,
      "levels": [
        {
          "id": "40000000-0000-0000-0000-000000000101",
          "level": 1,
          "role_id": "50000000-0000-0000-0000-000000000003",
          "approver_user_ids": ["10000000-0000-0000-0000-000000000003"]
        }
      ],
      "created_at": "2026-04-01T08:00:00Z",
      "updated_at": "2026-04-01T08:00:00Z"
    }
  ],
  "message": "Approval configs retrieved successfully"
}
```

---

### 1.2 Get Config by ID

```
GET /core/v1/approval-configs/:id
```

**Auth:** Bearer token + CompanyContext

**Response (200):** Single config object.

**Errors:**

| Status | Message |
|---|---|
| 404 | Approval config not found |

---

### 1.3 Create Approval Config

```
POST /core/v1/approval-configs
```

**Auth:** Bearer token + CompanyContext
**Permission:** `approval_configs:create`

**Request:**
```json
{
  "branch_id": "30000000-0000-0000-0000-000000000001",
  "feature_key": "journal_entry",
  "is_active": true,
  "levels": [
    {
      "level": 1,
      "role_id": "50000000-0000-0000-0000-000000000003",
      "approver_user_ids": [
        "10000000-0000-0000-0000-000000000003"
      ]
    },
    {
      "level": 2,
      "role_id": "50000000-0000-0000-0000-000000000002",
      "approver_user_ids": [
        "10000000-0000-0000-0000-000000000002"
      ]
    }
  ]
}
```

| Field | Type | Required | Validasi |
|---|---|---|---|
| `branch_id` | UUID | Ya | Harus valid UUID |
| `feature_key` | string | Ya | Harus dari whitelist |
| `is_active` | bool | Tidak | Default `true` |
| `levels` | array | Ya | Minimal 0 entry (0 = auto-approve) |
| `levels[].level` | int | Ya | Sequential mulai dari 1 |
| `levels[].role_id` | UUID | Ya | - |
| `levels[].approver_user_ids` | UUID[] | Ya | Minimal 1 user |

> `company_id` otomatis dari JWT.
> Kombinasi `(company_id, branch_id, feature_key)` harus unique (partial index, soft delete aware).

**Response (201):** Config object lengkap dengan `levels` yang sudah dibuat.

**Errors:**

| Status | Message |
|---|---|
| 400 | Unknown feature_key |
| 400 | Invalid levels (duplicate level numbers / levels must be sequential starting from 1) |
| 400 | Invalid request payload |
| 409 | (Postgres) `uniq_approval_configs_scope` — config untuk kombinasi ini sudah ada |

---

### 1.4 Update Approval Config

```
PUT /core/v1/approval-configs/:id
```

**Auth:** Bearer token + CompanyContext
**Permission:** `approval_configs:update`

**Request:** (semua field opsional)
```json
{
  "is_active": false,
  "levels": [
    {
      "level": 1,
      "role_id": "50000000-0000-0000-0000-000000000002",
      "approver_user_ids": [
        "10000000-0000-0000-0000-000000000002"
      ]
    }
  ]
}
```

| Field | Type | Keterangan |
|---|---|---|
| `is_active` | bool | Toggle aktif |
| `levels` | array | Kalau dikirim, akan **replace penuh** level list lama (delete + insert). Kalau `null`/tidak dikirim, level tidak disentuh. |

> **Penting:** update level bersifat replace set. Tidak ada patch level individual.
> `branch_id` dan `feature_key` **tidak bisa diubah** setelah create.

**Response (200):** Updated config object dengan level terbaru.

**Errors:**

| Status | Message |
|---|---|
| 400 | Invalid levels |
| 404 | Approval config not found |

---

### 1.5 Delete Approval Config

```
DELETE /core/v1/approval-configs/:id
```

**Auth:** Bearer token + CompanyContext
**Permission:** `approval_configs:delete`

Soft delete. Config yang sudah dihapus tidak ikut resolution, tapi `approval_requests` lama yang mereferensi config ini tetap utuh (audit trail tidak rusak).

**Response (200):**
```json
{
  "data": null,
  "message": "Approval config deleted successfully"
}
```

**Errors:**

| Status | Message |
|---|---|
| 404 | Approval config not found |

---

# Part 2 — Approval Requests

## Endpoints

### 2.1 List Approval Requests

```
GET /core/v1/approval-requests
```

**Auth:** Bearer token + CompanyContext

**Query Parameters:**

| Param | Type | Default | Keterangan |
|---|---|---|---|
| `page` | int | 1 | - |
| `limit` | int | 20 | Max 100 |
| `feature_key` | string | - | Filter per feature |
| `branch_id` | UUID | - | Filter per cabang |
| `status` | enum | - | `waiting` \| `approved` \| `rejected` \| `cancelled` |
| `only_mine` | bool | false | Kalau `true`, hanya request yang menunggu action user saat ini (level `current_level` + `eligible_approver_ids` include `user_id`) |

**Response (200):**
```json
{
  "data": [
    {
      "id": "60000000-0000-0000-0000-000000000001",
      "company_id": "20000000-0000-0000-0000-000000000001",
      "branch_id": "30000000-0000-0000-0000-000000000001",
      "feature_key": "journal_entry",
      "reff_type": "finance.journal_entries",
      "reff_id": "70000000-0000-0000-0000-000000000010",
      "status": "waiting",
      "current_level": 2,
      "total_levels": 2,
      "submitted_at": "2026-04-08T09:00:00Z",
      "submitted_by": "10000000-0000-0000-0000-000000000005",
      "finalized_at": null,
      "finalized_by": null,
      "reject_reason": null,
      "levels": [ /* ... */ ]
    }
  ],
  "message": "Approval requests retrieved successfully",
  "meta": {
    "pagination": {
      "page": 1,
      "limit": 20,
      "total": 1,
      "total_pages": 1
    }
  }
}
```

> Sorting default: `submitted_at DESC`.

---

### 2.2 Inbox (Waiting for Me)

```
GET /core/v1/approval-requests/inbox
```

**Auth:** Bearer token + CompanyContext

Shortcut endpoint: sama dengan List + paksa `only_mine=true` dan `status=waiting`. FE biasa memanggil ini untuk widget "Tugas approval saya".

**Query Parameters:** `page`, `limit`, `feature_key`, `branch_id` (status & only_mine di-override).

**Response (200):** Paginated list. Sama format dengan 2.1.

---

### 2.3 Get Request by ID

```
GET /core/v1/approval-requests/:id
```

**Auth:** Bearer token + CompanyContext

**Response (200):** Single request object (dengan `levels`).

**Errors:**

| Status | Message |
|---|---|
| 404 | Approval request not found |

---

### 2.4 Get Latest Request by Document

```
GET /core/v1/approval-requests/by-doc
```

**Auth:** Bearer token + CompanyContext

Mengembalikan approval request **paling terbaru** untuk satu dokumen target (`reff_type` + `reff_id`), **apapun statusnya**. Dipakai FE untuk dialog "siapa yang harus approve / siapa yang sudah approve kapan" pada baris list transaksi — bahkan setelah dokumen `posted` atau `rejected`.

Bila dokumen pernah di-reject lalu di-submit ulang, endpoint ini mengembalikan request terbaru (submitted_at DESC). Request lama tetap tersimpan di DB sebagai history dan tetap dapat diakses lewat `/approval-requests/:id` kalau ID-nya diketahui.

**Query Parameters:**

| Param | Type | Required | Keterangan |
|---|---|---|---|
| `reff_type` | string | Ya | Nama tabel target, mis. `finance.cash_transactions`, `finance.journal_entries`, `finance.fund_transfers`, `finance.a1_budget_projections`, `finance.a2_budget_arcae`, `finance.a3_operating_fund_curia` |
| `reff_id` | UUID | Ya | PK dokumen target |

> Scope company otomatis dari JWT — tidak bisa melihat request lintas tenant meski UUID ditebak benar.

**Response (200):** Single approval request object (dengan `levels` lengkap — sama persis dengan 2.3).

```json
{
  "data": {
    "id": "60000000-0000-0000-0000-000000000001",
    "company_id": "20000000-0000-0000-0000-000000000001",
    "branch_id": "30000000-0000-0000-0000-000000000001",
    "feature_key": "cash_in",
    "reff_type": "finance.cash_transactions",
    "reff_id": "70000000-0000-0000-0000-000000000010",
    "status": "waiting",
    "current_level": 2,
    "total_levels": 2,
    "submitted_at": "2026-04-17T09:00:00Z",
    "submitted_by": "10000000-0000-0000-0000-000000000005",
    "finalized_at": null,
    "finalized_by": null,
    "reject_reason": null,
    "levels": [
      {
        "id": "60000000-0000-0000-0000-000000000101",
        "level": 1,
        "role_id": "50000000-0000-0000-0000-000000000003",
        "role_name": "Finance Head",
        "eligible_approver_ids": ["10000000-0000-0000-0000-000000000003"],
        "status": "approved",
        "action_by": "10000000-0000-0000-0000-000000000003",
        "action_at": "2026-04-17T10:00:00Z",
        "comment": "OK"
      },
      {
        "id": "60000000-0000-0000-0000-000000000102",
        "level": 2,
        "role_id": "50000000-0000-0000-0000-000000000002",
        "role_name": "Treasurer",
        "eligible_approver_ids": ["10000000-0000-0000-0000-000000000002"],
        "status": "pending",
        "action_by": null,
        "action_at": null,
        "comment": null
      }
    ]
  },
  "message": "Approval request retrieved successfully"
}
```

**Contoh kasus pakai FE:**

```
GET /core/v1/approval-requests/by-doc?reff_type=finance.cash_transactions&reff_id=70000000-0000-0000-0000-000000000010
```

Dari response, FE bisa menampilkan:

- **"Siapa yang harus approve"** → loop `levels[]` dengan `status == "pending"`, pakai `eligible_approver_ids` + `role_name`.
- **"Sudah diapprove oleh"** → loop `levels[]` dengan `status == "approved"`, tampilkan `action_by` (user name di-resolve di FE) + `action_at` + `comment`.
- **"Ditolak oleh"** → cari level dengan `status == "rejected"`, tampilkan `action_by`, `action_at`, dan `comment` (= alasan reject).
- **Progress bar** → `current_level` / `total_levels`.

**Errors:**

| Status | Message |
|---|---|
| 400 | Invalid query parameters (reff_type / reff_id wajib, reff_id harus UUID) |
| 404 | No approval request found for this document |

> 404 berarti dokumen belum pernah di-submit. Dokumen masih `draft` tidak punya approval request sama sekali.

---

### 2.5 Approve

```
POST /core/v1/approval-requests/:id/approve
```

**Auth:** Bearer token + CompanyContext
**Permission:** `approval_requests:approve`

User harus termasuk `eligible_approver_ids` di level saat ini. Kalau ya, level saat ini ditandai `approved`:

- Kalau masih ada level berikutnya → `current_level++`, request tetap `waiting`
- Kalau sudah level terakhir → request jadi `approved`, `finalized_at`/`finalized_by` diisi

**Request:**
```json
{
  "comment": "Looks good"
}
```

| Field | Type | Required |
|---|---|---|
| `comment` | string | Tidak (max 500) |

**Response (200):** Updated request object.

**Errors:**

| Status | Message |
|---|---|
| 400 | Approval request is not in waiting state |
| 403 | You are not an eligible approver at the current level |
| 404 | Approval request not found |

---

### 2.6 Reject

```
POST /core/v1/approval-requests/:id/reject
```

**Auth:** Bearer token + CompanyContext
**Permission:** `approval_requests:approve`

Level saat ini ditandai `rejected`, sisa level yang masih `pending` ditandai `skipped`, request jadi `rejected`.

**Request:**
```json
{
  "reason": "Missing supporting docs"
}
```

| Field | Type | Required |
|---|---|---|
| `reason` | string | Ya (1-500) |

**Response (200):** Updated request object.

**Errors:**

| Status | Message |
|---|---|
| 400 | Approval request is not in waiting state |
| 400 | Invalid request payload (reason required) |
| 403 | You are not an eligible approver at the current level |
| 404 | Approval request not found |

---

### 2.7 Cancel (Submitter Only)

```
POST /core/v1/approval-requests/:id/cancel
```

**Auth:** Bearer token + CompanyContext

Hanya user yang menjadi submitter original (`submitted_by`) yang boleh cancel. Hanya boleh saat status masih `waiting`. Level pending akan ditandai `skipped`, request jadi `cancelled`.

**Request:**
```json
{
  "reason": "Salah entry, mau diperbaiki dulu"
}
```

| Field | Type | Required |
|---|---|---|
| `reason` | string | Tidak (max 500) |

**Response (200):** Updated request object.

**Errors:**

| Status | Message |
|---|---|
| 400 | Approval request is not in waiting state |
| 403 | Only the submitter can cancel a waiting request |
| 404 | Approval request not found |

---

## Business Rules

| Rule | Keterangan |
|---|---|
| Config wajib per cabang | Setiap cabang wajib setup approval config untuk tiap feature yang akan dipakai. Submit akan gagal kalau belum ada (`Approval config not configured for this branch & feature`). |
| Level sequential | Levels harus mulai dari 1 dan contiguous (1, 2, 3, ...). Gap/duplikat akan ditolak. |
| 0 levels = auto-approve | Config dengan `levels: []` bisa dipakai untuk feature yang tidak perlu approval. Submit akan langsung menghasilkan request `status=approved`. |
| Snapshot at submit | `role_name` dan `eligible_approver_ids` di `approval_request_levels` di-freeze saat submit. Perubahan config kemudian tidak mengubah history request lama. |
| 1 active request per doc | Satu dokumen hanya boleh punya 1 request dengan status `waiting`. Submit ulang saat masih waiting akan gagal dengan `409 An active approval request already exists for this document`. |
| Resubmit setelah reject | Service pemanggil (mis. journal_entries) boleh submit ulang dokumen yang status-nya `rejected`. Request baru akan jadi instance terpisah (request lama tetap tersimpan sebagai history). |
| Reject = skip remaining | Saat reject di level X, semua level X+1..N yang masih `pending` otomatis ditandai `skipped` supaya audit trail eksplisit. |
| Status vocabulary consumer | Engine selalu pakai `waiting` \| `approved` \| `rejected` \| `cancelled`. Beberapa modul consumer memilih vocabulary sendiri di kolom status mereka (mis. cash book pakai `draft`/`waiting`/`posted`/`rejected`, A1 budget pakai `draft`/`in_review`/`approved`/`rejected`). Service layer consumer bertugas map antara status dokumen dan status engine. Response `approval_requests` sendiri tetap memakai vocabulary engine. |

---

## Permission Requirements Summary

| Endpoint | Permission |
|---|---|
| `GET /approval-configs` | Login + CompanyContext |
| `GET /approval-configs/:id` | Login + CompanyContext |
| `POST /approval-configs` | `approval_configs:create` |
| `PUT /approval-configs/:id` | `approval_configs:update` |
| `DELETE /approval-configs/:id` | `approval_configs:delete` |
| `GET /approval-requests` | Login + CompanyContext |
| `GET /approval-requests/inbox` | Login + CompanyContext |
| `GET /approval-requests/by-doc` | Login + CompanyContext |
| `GET /approval-requests/:id` | Login + CompanyContext |
| `POST /approval-requests/:id/approve` | `approval_requests:approve` |
| `POST /approval-requests/:id/reject` | `approval_requests:approve` |
| `POST /approval-requests/:id/cancel` | Login + CompanyContext (submitter check di service) |

> Role `administrator` default sudah include `approval_configs: admin` dan `approval_requests: admin` (via seeder). Action `approve` otomatis terbongkar karena ditambahkan ke `LevelAdmin` action list.

---

## Integration Guide (untuk developer modul baru)

Modul feature (mis. `cash_in`, `expense`, `transfer`) yang ingin pakai approval system tinggal ikuti 3 langkah:

### 1. Daftarkan feature_key

Tambah constant di `internal/modules/core/approval/domain/feature_keys.go`:

```go
const (
    FeatureCashIn   = "cash_in"
    FeatureA1Budget = "a1_budget"
    // ...
)

var knownFeatureKeys = map[string]struct{}{
    FeatureCashIn:   {},
    FeatureA1Budget: {},
    // ...
}
```

> Snake_case lowercase wajib — mengikuti CHECK regex `^[a-z][a-z0-9_]*$` pada kolom `core.approval_configs.feature_key`.

### 2. Inject `ApprovalRequestService` ke modul Anda

Di `main.{modul}.go`:

```go
func Initialize(db *pgxpool.Pool, approvalSvc *approvalService.ApprovalRequestService) *Module {
    // ...
}
```

Lalu di [router.go](../../../internal/router/router.go) pass `approvalModule.RequestService` saat Initialize.

### 3. Panggil `SubmitTx` di dalam transaksi service Anda

```go
submitParams := approvalDto.SubmitParams{
    CompanyID:   companyID,
    BranchID:    doc.BranchID,
    FeatureKey:  approvalDomain.FeatureCashIn,
    ReffType:    "finance.cash_in",  // nama tabel target
    ReffID:      doc.ID,
    SubmittedBy: userID,
}
result, err := s.approvalSvc.SubmitTx(ctx, tx, submitParams)
if err != nil { return err }

var newStatus string
if result.AutoApproved {
    newStatus = "posted"
} else {
    newStatus = "waiting"
}
// update denormalized status di tabel dokumen Anda
```

Untuk Approve/Reject, service Anda menjadi orchestrator:

```go
reqResp, _ := s.approvalSvc.GetActiveByDoc(ctx, "finance.cash_in", docID)
result, err := s.approvalSvc.ApproveTx(ctx, tx, reqResp.ID, userID, comment)
if result.Finalized {
    // update dokumen → status='posted'
}
```

Pola wiring-nya: panggil `approvalRequestService.Submit(ctx, dto.SubmitParams{...})` dari service feature saat user menekan "Submit for Approval", lalu `Process(...)` saat approver men-setujui/menolak, dan update status dokumen ke `posted` bila `result.Finalized == true`.

### 4. Seed permission

Tambahkan resource baru di `internal/database/seeders/core/001_roles.sql` ke role `administrator` supaya default admin langsung bisa pakai:

```diff
     "finance.chart_of_accounts": "admin",
     "finance.journal_entries": "admin",
+    "finance.a1_budget": "admin",
     "approval_configs": "admin",
```

> Level `admin` expand ke actions: `create`, `read`, `update`, `delete`, `export`, `import`, `restore`, `approve`. Jadi cukup satu entry per resource.

---

**Last Updated:** 2026-04-18
