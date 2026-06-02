module github.com/CathalByrneGit/corncrake

go 1.22.2

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/golang-jwt/jwt/v5 v5.2.1
)

// Database drivers — included only when built with the matching build tag.
// Default build (no tags) uses the in-memory store and needs no DB drivers.
//
//   go build                      → memory store (no external deps)
//   go build -tags postgres       → PostgreSQL via lib/pq
//   go build -tags sqlserver      → SQL Server via go-mssqldb
require (
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect; sqlserver
	github.com/golang-sql/sqlexp v0.1.0 // indirect; sqlserver
	github.com/lib/pq v1.10.9 // postgres
	github.com/microsoft/go-mssqldb v1.7.2 // sqlserver
	golang.org/x/crypto v0.18.0 // indirect; sqlserver
	golang.org/x/text v0.14.0 // indirect; sqlserver
)
