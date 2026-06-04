package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/CathalByrneGit/corncrake/internal/models"
)

// PostgresStore is a PostgreSQL-backed implementation of SubmissionStore.
// Uses standard database/sql — no ORM, no code generation.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates the store and ensures the schema exists.
func NewPostgresStore(db *sql.DB) (*PostgresStore, error) {
	s := &PostgresStore{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// migrate creates tables if they do not exist.
// Idempotent — safe to run on every startup.
func (s *PostgresStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS submissions (
			submission_id    TEXT        PRIMARY KEY,
			tenant_id        TEXT        NOT NULL,
			run_reference    TEXT        NOT NULL,
			holding_number   TEXT        NOT NULL,
			tax_year         INTEGER     NOT NULL,
			quarter          INTEGER     NOT NULL,
			return_type      TEXT        NOT NULL DEFAULT 'ORIGINAL',
			status           TEXT        NOT NULL DEFAULT 'RECEIVED',
			received_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			software_used    TEXT,
			software_version TEXT,
			item_count       INTEGER     NOT NULL,
			body             JSONB       NOT NULL,
			warnings         JSONB,
			contact_email    TEXT,
			contact_phone    TEXT,
			comments         TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_submissions_holding
			ON submissions (holding_number);

		CREATE INDEX IF NOT EXISTS idx_submissions_run
			ON submissions (run_reference);

		CREATE INDEX IF NOT EXISTS idx_submissions_period
			ON submissions (holding_number, tax_year, quarter);
	`)
	return err
}

func (s *PostgresStore) Create(p CreateParams) (*CreateResult, error) {
	warningsJSON, err := json.Marshal(p.Warnings)
	if err != nil {
		return nil, fmt.Errorf("marshalling warnings: %w", err)
	}

	receivedAt := time.Now().UTC()

	_, err = s.db.Exec(`
		INSERT INTO submissions (
			submission_id, tenant_id, run_reference, holding_number, tax_year, quarter,
			return_type, status, received_at, software_used, software_version,
			item_count, body, warnings,
			contact_email, contact_phone, comments
		) VALUES ($1,$2,$3,$4,$5,$6,$7,'RECEIVED',$8,$9,$10,$11,$12,$13,$14,$15,$16)
		ON CONFLICT (submission_id) DO NOTHING`,
		p.SubmissionID, p.TenantID, p.RunReference, p.HoldingNumber,
		p.TaxYear, p.Quarter, p.ReturnType,
		receivedAt, p.Client.SoftwareUsed, p.Client.SoftwareVersion,
		p.ItemCount, p.Body, warningsJSON,
		p.ContactEmail, p.ContactPhone, p.Comments,
	)
	if err != nil {
		return nil, fmt.Errorf("insert submission: %w", err)
	}

	return &CreateResult{
		SubmissionID: p.SubmissionID,
		RunReference: p.RunReference,
		Status:       "RECEIVED",
		ReceivedAt:   receivedAt,
		ItemCount:    p.ItemCount,
		Warnings:     p.Warnings,
	}, nil
}

func (s *PostgresStore) GetByID(submissionID string) (*models.SubmissionRecord, error) {
	row := s.db.QueryRow(`
		SELECT submission_id, tenant_id, run_reference, holding_number, tax_year, quarter,
		       return_type, status, received_at, software_used, software_version,
		       item_count, body, warnings,
		       contact_email, contact_phone, comments
		FROM submissions
		WHERE submission_id = $1`, submissionID)

	return scanSubmission(row)
}

func (s *PostgresStore) GetRun(runReference string) (*models.RunStatus, error) {
	rows, err := s.db.Query(`
		SELECT submission_id, tenant_id, run_reference, quarter, tax_year,
		       return_type, status, received_at, item_count
		FROM submissions
		WHERE run_reference = $1
		ORDER BY received_at DESC`, runReference)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRunStatus(rows, runReference)
}

func (s *PostgresStore) List(holdingNumber string, taxYear, quarter int) ([]models.SubmissionSummary, error) {
	query := `
		SELECT submission_id, tenant_id, run_reference, quarter, tax_year,
		       return_type, status, received_at, item_count
		FROM submissions
		WHERE holding_number = $1`
	args := []any{holdingNumber}

	if taxYear > 0 {
		args = append(args, taxYear)
		query += fmt.Sprintf(" AND tax_year = $%d", len(args))
	}
	if quarter > 0 {
		args = append(args, quarter)
		query += fmt.Sprintf(" AND quarter = $%d", len(args))
	}
	query += " ORDER BY received_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSummaries(rows)
}

// ── SQL Server store ──────────────────────────────────────────────────────────

// SQLServerStore is a Microsoft SQL Server-backed implementation.
// Uses the same schema as PostgresStore with MSSQL-compatible DDL.
type SQLServerStore struct {
	db *sql.DB
}

// NewSQLServerStore creates the store and ensures the schema exists.
func NewSQLServerStore(db *sql.DB) (*SQLServerStore, error) {
	s := &SQLServerStore{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *SQLServerStore) migrate() error {
	// SQL Server uses IF NOT EXISTS via object catalog check
	// NVARCHAR(MAX) for JSON columns — SQL Server 2016+ has native JSON support
	_, err := s.db.Exec(`
		IF NOT EXISTS (
			SELECT 1 FROM sys.tables WHERE name = 'submissions'
		)
		CREATE TABLE submissions (
			submission_id    NVARCHAR(36)     NOT NULL PRIMARY KEY,
			tenant_id        NVARCHAR(100)    NOT NULL,
			run_reference    NVARCHAR(100)    NOT NULL,
			holding_number   NVARCHAR(15)     NOT NULL,
			tax_year         INT              NOT NULL,
			quarter          INT              NOT NULL,
			return_type      NVARCHAR(10)     NOT NULL DEFAULT 'ORIGINAL',
			status           NVARCHAR(20)     NOT NULL DEFAULT 'RECEIVED',
			received_at      DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
			software_used    NVARCHAR(100)    NULL,
			software_version NVARCHAR(20)     NULL,
			item_count       INT              NOT NULL,
			body             NVARCHAR(MAX)    NOT NULL,
			warnings         NVARCHAR(MAX)    NULL,
			contact_email    NVARCHAR(255)    NULL,
			contact_phone    NVARCHAR(30)     NULL,
			comments         NVARCHAR(500)    NULL
		);

		IF NOT EXISTS (
			SELECT 1 FROM sys.indexes
			WHERE name = 'idx_submissions_holding'
		)
		CREATE INDEX idx_submissions_holding
			ON submissions (holding_number);

		IF NOT EXISTS (
			SELECT 1 FROM sys.indexes
			WHERE name = 'idx_submissions_run'
		)
		CREATE INDEX idx_submissions_run
			ON submissions (run_reference);
	`)
	return err
}

// SQL Server uses @p1, @p2... positional parameters
func (s *SQLServerStore) Create(p CreateParams) (*CreateResult, error) {
	warningsJSON, _ := json.Marshal(p.Warnings)
	receivedAt := time.Now().UTC()

	_, err := s.db.Exec(`
		IF NOT EXISTS (SELECT 1 FROM submissions WHERE submission_id = @p1)
		INSERT INTO submissions (
			submission_id, tenant_id, run_reference, holding_number, tax_year, quarter,
			return_type, status, received_at, software_used, software_version,
			item_count, body, warnings,
			contact_email, contact_phone, comments
		) VALUES (@p1,@p2,@p3,@p4,@p5,@p6,@p7,'RECEIVED',@p8,@p9,@p10,@p11,@p12,@p13,@p14,@p15,@p16)`,
		p.SubmissionID, p.TenantID, p.RunReference, p.HoldingNumber,
		p.TaxYear, p.Quarter, p.ReturnType,
		receivedAt, p.Client.SoftwareUsed, p.Client.SoftwareVersion,
		p.ItemCount, string(p.Body), string(warningsJSON),
		p.ContactEmail, p.ContactPhone, p.Comments,
	)
	if err != nil {
		return nil, fmt.Errorf("insert submission: %w", err)
	}

	return &CreateResult{
		SubmissionID: p.SubmissionID,
		RunReference: p.RunReference,
		Status:       "RECEIVED",
		ReceivedAt:   receivedAt,
		ItemCount:    p.ItemCount,
		Warnings:     p.Warnings,
	}, nil
}

func (s *SQLServerStore) GetByID(submissionID string) (*models.SubmissionRecord, error) {
	row := s.db.QueryRow(`
		SELECT submission_id, tenant_id, run_reference, holding_number, tax_year, quarter,
		       return_type, status, received_at, software_used, software_version,
		       item_count, body, warnings,
		       contact_email, contact_phone, comments
		FROM submissions
		WHERE submission_id = @p1`, submissionID)

	return scanSubmission(row)
}

func (s *SQLServerStore) GetRun(runReference string) (*models.RunStatus, error) {
	rows, err := s.db.Query(`
		SELECT submission_id, tenant_id, run_reference, quarter, tax_year,
		       return_type, status, received_at, item_count
		FROM submissions
		WHERE run_reference = @p1
		ORDER BY received_at DESC`, runReference)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRunStatus(rows, runReference)
}

func (s *SQLServerStore) List(holdingNumber string, taxYear, quarter int) ([]models.SubmissionSummary, error) {
	// SQL Server doesn't support dynamic $N params — build with numbered @pN
	query := `
		SELECT submission_id, tenant_id, run_reference, quarter, tax_year,
		       return_type, status, received_at, item_count
		FROM submissions
		WHERE holding_number = @p1`
	args := []any{holdingNumber}

	if taxYear > 0 {
		args = append(args, taxYear)
		query += fmt.Sprintf(" AND tax_year = @p%d", len(args))
	}
	if quarter > 0 {
		args = append(args, quarter)
		query += fmt.Sprintf(" AND quarter = @p%d", len(args))
	}
	query += " ORDER BY received_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSummaries(rows)
}

// ── Shared scan helpers ───────────────────────────────────────────────────────
// Both stores return the same Go types so scanning is identical.

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSubmission(row rowScanner) (*models.SubmissionRecord, error) {
	var r models.SubmissionRecord
	var bodyJSON, warningsJSON []byte
	var softwareUsed, softwareVersion sql.NullString
	var contactEmail, contactPhone, comments sql.NullString

	err := row.Scan(
		&r.SubmissionID, &r.TenantID, &r.RunReference, &r.HoldingNumber,
		&r.TaxYear, &r.Quarter, &r.ReturnType, &r.Status, &r.ReceivedAt,
		&softwareUsed, &softwareVersion,
		&r.ItemCount, &bodyJSON, &warningsJSON,
		&contactEmail, &contactPhone, &comments,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	r.SoftwareUsed = softwareUsed.String
	r.SoftwareVersion = softwareVersion.String
	r.ContactEmail = contactEmail.String
	r.ContactPhone = contactPhone.String
	r.Comments = comments.String

	r.Body = json.RawMessage(bodyJSON)
	if len(warningsJSON) > 0 && string(warningsJSON) != "null" {
		if err := json.Unmarshal(warningsJSON, &r.Warnings); err != nil {
			return nil, fmt.Errorf("unmarshal warnings: %w", err)
		}
	}

	return &r, nil
}

func scanRunStatus(rows *sql.Rows, runReference string) (*models.RunStatus, error) {
	run := &models.RunStatus{RunReference: runReference}
	for rows.Next() {
		var s models.SubmissionSummary
		if err := rows.Scan(
			&s.SubmissionID, &s.TenantID, &s.RunReference, &s.Quarter, &s.TaxYear,
			&s.ReturnType, &s.Status, &s.ReceivedAt, &s.ItemCount,
		); err != nil {
			return nil, err
		}
		run.TotalItems += s.ItemCount
		run.Submissions = append(run.Submissions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(run.Submissions) == 0 {
		return nil, ErrNotFound
	}
	run.TotalSubmissions = len(run.Submissions)
	return run, nil
}

func scanSummaries(rows *sql.Rows) ([]models.SubmissionSummary, error) {
	var results []models.SubmissionSummary
	for rows.Next() {
		var s models.SubmissionSummary
		if err := rows.Scan(
			&s.SubmissionID, &s.TenantID, &s.RunReference, &s.Quarter, &s.TaxYear,
			&s.ReturnType, &s.Status, &s.ReceivedAt, &s.ItemCount,
		); err != nil {
			return nil, err
		}
		results = append(results, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].ReceivedAt.After(results[j].ReceivedAt)
	})
	return results, nil
}
