//go:build postgres

// This file registers the PostgreSQL driver.
// Compile with: go build -tags postgres
// Or for SQL Server: go build -tags sqlserver
// Default (no tags): memory store only — no external DB dependencies.
package services

import _ "github.com/lib/pq"
