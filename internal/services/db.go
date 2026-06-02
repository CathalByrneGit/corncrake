// Package services — database configuration and store factory.
//
// The store backend is selected at startup via the DB_DRIVER environment
// variable. No code changes are needed to switch databases.
//
// Supported drivers:
//
//	memory   — in-memory (default, for development and testing)
//	postgres — PostgreSQL (open source, recommended for cloud deployments)
//	sqlserver — Microsoft SQL Server (in-house default for CSO infrastructure)
//
// Connection string format per driver:
//
//	postgres:   DATABASE_URL=postgres://user:pass@host:5432/ehecs?sslmode=require
//	sqlserver:  DATABASE_URL=sqlserver://user:pass@host:1433?database=ehecs&encrypt=true
package services

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
)

// DBConfig holds the parsed database configuration.
type DBConfig struct {
	Driver string // "memory" | "postgres" | "sqlserver"
	DSN    string // connection string
}

// DBConfigFromEnv reads DB_DRIVER and DATABASE_URL from the environment.
func DBConfigFromEnv() DBConfig {
	return DBConfig{
		Driver: envOrDefaultSvc("DB_DRIVER", "memory"),
		DSN:    os.Getenv("DATABASE_URL"),
	}
}

// NewStore creates the appropriate SubmissionStore based on config.
// Called once from main() at startup.
func NewStore(cfg DBConfig) (SubmissionStore, error) {
	switch cfg.Driver {
	case "memory", "":
		slog.Info("store: using in-memory backend (data not persisted)")
		return NewMemoryStore(), nil

	case "postgres":
		if cfg.DSN == "" {
			return nil, fmt.Errorf("DATABASE_URL is required for DB_DRIVER=postgres")
		}
		slog.Info("store: connecting to PostgreSQL")
		db, err := sql.Open("postgres", cfg.DSN)
		if err != nil {
			return nil, fmt.Errorf("postgres: open: %w", err)
		}
		if err := db.Ping(); err != nil {
			return nil, fmt.Errorf("postgres: ping: %w", err)
		}
		store, err := NewPostgresStore(db)
		if err != nil {
			return nil, fmt.Errorf("postgres: init: %w", err)
		}
		slog.Info("store: PostgreSQL ready")
		return store, nil

	case "sqlserver", "mssql":
		if cfg.DSN == "" {
			return nil, fmt.Errorf("DATABASE_URL is required for DB_DRIVER=sqlserver")
		}
		slog.Info("store: connecting to SQL Server")
		db, err := sql.Open("sqlserver", cfg.DSN)
		if err != nil {
			return nil, fmt.Errorf("sqlserver: open: %w", err)
		}
		if err := db.Ping(); err != nil {
			return nil, fmt.Errorf("sqlserver: ping: %w", err)
		}
		store, err := NewSQLServerStore(db)
		if err != nil {
			return nil, fmt.Errorf("sqlserver: init: %w", err)
		}
		slog.Info("store: SQL Server ready")
		return store, nil

	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER %q — must be memory, postgres, or sqlserver", cfg.Driver)
	}
}

func envOrDefaultSvc(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
