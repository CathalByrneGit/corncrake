//go:build sqlserver

// This file registers the Microsoft SQL Server driver.
// Compile with: go build -tags sqlserver
package services

import _ "github.com/microsoft/go-mssqldb"
