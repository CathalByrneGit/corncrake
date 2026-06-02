# Corncrake

[![CI](https://github.com/CathalByrneGit/corncrake/actions/workflows/ci.yml/badge.svg)](https://github.com/CathalByrneGit/corncrake/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![OpenAPI 3.0](https://img.shields.io/badge/OpenAPI-3.0-green)](docs/openapi.yaml)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8)](go.mod)

A configurable REST API for statutory survey submissions to national statistical institutes.

Ships with a built-in tenant for the **CSO EHECS** (Earnings, Hours and Employment Costs Survey) — add your own scheme by implementing two small interfaces. Modelled on Revenue Ireland's PAYE Modernisation REST API architecture.

> This is an independent open-source project. It is not affiliated with or endorsed by the Central Statistics Office, Ireland.

Part of the [Corncrake toolchain](#ecosystem).

---

## Why Corncrake

The existing EHECS submission process requires generating a schema-specific XML file and uploading it manually through a web portal. There is no direct API integration path — every system, regardless of how sophisticated, ends up exporting a file and clicking upload.

Corncrake provides the API layer that removes the manual step — payroll software submits directly in real time, the same way Revenue's PAYE Modernisation works. The tenant architecture means the same infrastructure serves multiple survey types without duplicating code.

---

## Adding a new survey tenant

Implement two interfaces and register in `init()`:

```go
func init() {
    tenant.Register(&tenant.Config{
        ID:          "my-survey",
        Name:        "My Statistical Survey",
        BaseURL:     "https://api.example.ie/survey/v1",
        FieldSchema: myFields,
        Validator:   &MySurveyValidator{},
        Formatter:   &MySurveyFormatter{},
    })
}
```

The API, validation pipeline, authentication, and database layer work unchanged for every registered tenant.

---

## Quick start

```bash
git clone https://github.com/CathalByrneGit/corncrake.git
cd corncrake
go run .
```

- Swagger UI: [http://localhost:3000/api-docs](http://localhost:3000/api-docs)
- Health check: [http://localhost:3000/corncrake/v1/health](http://localhost:3000/corncrake/v1/health)

---

## Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET`  | `/corncrake/v1/health` | None | Health check |
| `POST` | `/corncrake/v1/submissions/{holdingNumber}/{taxYear}/{quarter}/{runReference}/{submissionId}` | JWT | Create submission |
| `GET`  | `/corncrake/v1/submissions/{holdingNumber}/{taxYear}/{quarter}/{runReference}/{submissionId}` | JWT | Retrieve submission |
| `GET`  | `/corncrake/v1/submissions/{holdingNumber}/{taxYear}/{quarter}/{runReference}` | JWT | Run status |
| `GET`  | `/corncrake/v1/submissions/{holdingNumber}` | JWT | List submissions |
| `GET`  | `/corncrake/v1/lookups/occupation-codes` | JWT | CSO SOC-2010 codes |
| `GET`  | `/corncrake/v1/lookups/periods/{holdingNumber}` | JWT | Open reporting periods |
| `GET`  | `/corncrake/v1/lookups/schema-version` | JWT | Schema version |

Full specification: [`docs/openapi.yaml`](docs/openapi.yaml)

---

## Authentication

Bearer JWT in the `Authorization` header. At minimum the token must carry:

```json
{ "holdingNumber": "CSO123456" }
```

**Using `corncrake-cli` or the web formatter?** A token is issued directly to your organisation. You do not need to think about software identifiers.

**Integrating a payroll software product?** Also include:

```json
{
  "holdingNumber":   "CSO123456",
  "softwareUsed":    "YourProductName",
  "softwareVersion": "2026.1"
}
```

---

## Configurable database backend

| `DB_DRIVER` | Backend | Build tag |
|-------------|---------|-----------|
| `memory` | In-memory (default) | none |
| `postgres` | PostgreSQL | `-tags postgres` |
| `sqlserver` | Microsoft SQL Server | `-tags sqlserver` |

```bash
go build                    # memory only — no DB deps, works offline
go build -tags postgres     # include PostgreSQL driver
go build -tags sqlserver    # include SQL Server driver
go test ./...               # always runs against memory store
```

```bash
# Runtime
DB_DRIVER=sqlserver
DATABASE_URL=sqlserver://user:pass@host:1433?database=corncrake&encrypt=true
```

---

## Validation (CSO EHECS tenant)

Source: *CSO Notes for Payroll Software Providers on EHECS Requirements v4.0*

**HTTP 400 — Schema:** PPSN format, occupation codes 1–9, employment type enum, all numeric fields ≥ 0.

**HTTP 422 — Logic:** `overtimePay > 0` requires `overtimeHours > 0` · `grossEarnings >= overtimePay` · `basicHours` ≤ 2,184 per quarter.

---

## Credential storage

- Plain-text secret generated once, emailed to the employer, **never stored**
- PBKDF2-HMAC-SHA256 hash + unique salt persisted per credential
- SQL Server deployments: hash and salt columns use **Always Encrypted**
- Append-only audit log for every AUTH_SUCCESS and AUTH_FAIL

---

## Cryptography roadmap

| Timeline | Action |
|----------|--------|
| Now | Enable hybrid post-quantum TLS at the load balancer (AWS ALB, Azure, Cloudflare — no code changes needed) |
| 12–18 months | Migrate JWT signing RS256 → ML-DSA (NIST FIPS 204) once Go ecosystem has stable support |
| No action needed | PBKDF2 hashing and AES-256 Always Encrypted are not meaningfully vulnerable to quantum attack |

---

## Project structure

```
corncrake/
├── main.go
├── internal/
│   ├── models/          Domain types
│   ├── validators/      Schema and logic validation
│   ├── services/        SubmissionStore interface + Memory/Postgres/SQLServer
│   ├── middleware/      JWT auth
│   └── handlers/        HTTP handlers + tests
├── docs/openapi.yaml    OpenAPI 3.0 spec
└── .env.example
```

---

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `3000` | Listen port |
| `APP_ENV` | `development` | `json` logging in production |
| `JWT_SECRET` | dev secret | **Change in production** |
| `DB_DRIVER` | `memory` | `memory` \| `postgres` \| `sqlserver` |
| `DATABASE_URL` | — | Required for `postgres` / `sqlserver` |

---

## Testing

```bash
go test ./...        # 17 tests, no database needed
go test -race ./...  # with race detector
```

---

## Ecosystem

| Repo | Description |
|------|-------------|
| [`corncrake`](https://github.com/CathalByrneGit/corncrake) | This service |
| [`corncrake-sdk`](https://github.com/CathalByrneGit/corncrake-sdk) | Go library — format and submit from any application |
| [`corncrake-cli`](https://github.com/CathalByrneGit/corncrake-cli) | CLI tool — map, validate, submit, get XML from the terminal |

---

## Licence

MIT — Independent open-source project, not affiliated with the CSO.
