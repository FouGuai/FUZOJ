package db

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

// PostgreSQLConfig holds the configuration for PostgreSQL connection pool
type PostgreSQLConfig struct {
	// DSN is the data source name
	// Format: "user=postgres password=password host=localhost port=5432 dbname=dbname sslmode=disable"
	DSN string

	// MaxOpenConnections is the maximum number of open connections to the database
	// Default: 25
	MaxOpenConnections int

	// MaxIdleConnections is the maximum number of connections in the idle connection pool
	// Default: 5
	MaxIdleConnections int

	// ConnMaxLifetime is the maximum amount of time a connection may be reused
	// Default: 5 minutes
	ConnMaxLifetime time.Duration

	// ConnMaxIdleTime is the maximum amount of time a connection may be idle
	// Default: 10 minutes
	ConnMaxIdleTime time.Duration
}

// DefaultPostgreSQLConfig returns the default PostgreSQL configuration
func DefaultPostgreSQLConfig() *PostgreSQLConfig {
	return &PostgreSQLConfig{
		MaxOpenConnections: 25,
		MaxIdleConnections: 5,
		ConnMaxLifetime:    5 * time.Minute,
		ConnMaxIdleTime:    10 * time.Minute,
	}
}

// PostgreSQL implements the Database interface using PostgreSQL driver with connection pooling
// Each PostgreSQL instance manages its own connection pool
type PostgreSQL struct {
	db     *sql.DB
	config *PostgreSQLConfig
	mu     sync.RWMutex
}

// NewPostgreSQL creates a new PostgreSQL database connection with connection pool
// DSN format: "user=postgres password=password host=localhost port=5432 dbname=dbname sslmode=disable"
func NewPostgreSQL(dsn string) (*PostgreSQL, error) {
	config := DefaultPostgreSQLConfig()
	config.DSN = dsn
	return NewPostgreSQLWithConfig(config)
}

// NewPostgreSQLWithConfig creates a new PostgreSQL database connection with custom configuration
func NewPostgreSQLWithConfig(config *PostgreSQLConfig) (*PostgreSQL, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if config.DSN == "" {
		return nil, fmt.Errorf("DSN cannot be empty")
	}

	// Set defaults if not specified
	if config.MaxOpenConnections == 0 {
		config.MaxOpenConnections = 25
	}
	if config.MaxIdleConnections == 0 {
		config.MaxIdleConnections = 5
	}
	if config.ConnMaxLifetime == 0 {
		config.ConnMaxLifetime = 5 * time.Minute
	}
	if config.ConnMaxIdleTime == 0 {
		config.ConnMaxIdleTime = 10 * time.Minute
	}

	db, err := sql.Open("postgres", config.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConnections)
	db.SetMaxIdleConns(config.MaxIdleConnections)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	// Verify the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgreSQL{db: db, config: config}, nil
}

// NewPostgreSQLWithDB creates a PostgreSQL instance from an existing sql.DB
func NewPostgreSQLWithDB(db *sql.DB) (*PostgreSQL, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	return &PostgreSQL{db: db, config: DefaultPostgreSQLConfig()}, nil
}

// GetConfig returns the current PostgreSQL configuration
func (p *PostgreSQL) GetConfig() *PostgreSQLConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// SetMaxOpenConnections sets the maximum number of open connections
func (p *PostgreSQL) SetMaxOpenConnections(n int) {
	if n > 0 {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.db.SetMaxOpenConns(n)
		p.config.MaxOpenConnections = n
	}
}

// SetMaxIdleConnections sets the maximum number of idle connections
func (p *PostgreSQL) SetMaxIdleConnections(n int) {
	if n >= 0 {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.db.SetMaxIdleConns(n)
		p.config.MaxIdleConnections = n
	}
}

// SetConnMaxLifetime sets the maximum lifetime of a connection
func (p *PostgreSQL) SetConnMaxLifetime(d time.Duration) {
	if d >= 0 {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.db.SetConnMaxLifetime(d)
		p.config.ConnMaxLifetime = d
	}
}

// SetConnMaxIdleTime sets the maximum idle time of a connection
func (p *PostgreSQL) SetConnMaxIdleTime(d time.Duration) {
	if d >= 0 {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.db.SetConnMaxIdleTime(d)
		p.config.ConnMaxIdleTime = d
	}
}

// Query executes a query that returns rows
func (p *PostgreSQL) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	return &PostgreSQLRows{rows: rows}, nil
}

// QueryRow executes a query that returns at most one row
func (p *PostgreSQL) QueryRow(ctx context.Context, query string, args ...interface{}) Row {
	return &PostgreSQLRow{row: p.db.QueryRowContext(ctx, query, args...)}
}

// Exec executes a query that doesn't return rows
func (p *PostgreSQL) Exec(ctx context.Context, query string, args ...interface{}) (Result, error) {
	result, err := p.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("exec failed: %w", err)
	}
	return &PostgreSQLResult{result: result}, nil
}

// Transaction executes a function within a database transaction
func (p *PostgreSQL) Transaction(ctx context.Context, fn func(tx Transaction) error) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}

	pgTx := &PostgreSQLTransaction{tx: tx}
	if err := fn(pgTx); err != nil {
		_ = pgTx.Rollback()
		return err
	}

	return pgTx.Commit()
}

// BeginTx starts a new transaction with the given options
func (p *PostgreSQL) BeginTx(ctx context.Context, opts *TxOptions) (Transaction, error) {
	sqlOpts := ConvertTxOptions(opts)
	tx, err := p.db.BeginTx(ctx, sqlOpts)
	if err != nil {
		return nil, fmt.Errorf("begin transaction failed: %w", err)
	}
	return &PostgreSQLTransaction{tx: tx}, nil
}

// Prepare creates a prepared statement for later queries or executions
func (p *PostgreSQL) Prepare(ctx context.Context, query string) (Stmt, error) {
	stmt, err := p.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("prepare statement failed: %w", err)
	}
	return &PostgreSQLStmt{stmt: stmt}, nil
}

// Ping verifies a connection to the database is still alive
func (p *PostgreSQL) Ping(ctx context.Context) error {
	if err := p.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	return nil
}

// Close closes the database connection
func (p *PostgreSQL) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.db.Close(); err != nil {
		return fmt.Errorf("close failed: %w", err)
	}
	return nil
}

// Stats returns database statistics
func (p *PostgreSQL) Stats() Stats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return ConvertSQLStats(p.db.Stats())
}

// GetDB returns the underlying database instance
func (p *PostgreSQL) GetDB() interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.db
}

// PostgreSQLRows implements the Rows interface
type PostgreSQLRows struct {
	rows *sql.Rows
}

// Next prepares the next result row
func (r *PostgreSQLRows) Next() bool {
	return r.rows.Next()
}

// Scan copies the columns from the current row into the values
func (r *PostgreSQLRows) Scan(dest ...interface{}) error {
	if err := r.rows.Scan(dest...); err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}
	return nil
}

// Close closes the Rows
func (r *PostgreSQLRows) Close() error {
	if err := r.rows.Close(); err != nil {
		return fmt.Errorf("close rows failed: %w", err)
	}
	return nil
}

// Err returns the error encountered during iteration
func (r *PostgreSQLRows) Err() error {
	return r.rows.Err()
}

// Columns returns the column names
func (r *PostgreSQLRows) Columns() ([]string, error) {
	cols, err := r.rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get columns failed: %w", err)
	}
	return cols, nil
}

// ColumnTypes returns column type information
func (r *PostgreSQLRows) ColumnTypes() ([]ColumnType, error) {
	types, err := r.rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("get column types failed: %w", err)
	}
	result := make([]ColumnType, len(types))
	for i, t := range types {
		result[i] = &PostgreSQLColumnType{ct: t}
	}
	return result, nil
}

// NextResultSet advances to the next result set
func (r *PostgreSQLRows) NextResultSet() bool {
	return r.rows.NextResultSet()
}

// PostgreSQLRow implements the Row interface
type PostgreSQLRow struct {
	row *sql.Row
}

// Scan copies the columns from the matched row
func (r *PostgreSQLRow) Scan(dest ...interface{}) error {
	if err := r.row.Scan(dest...); err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}
	return nil
}

// PostgreSQLResult implements the Result interface
type PostgreSQLResult struct {
	result sql.Result
}

// LastInsertId returns the last inserted ID
func (r *PostgreSQLResult) LastInsertId() (int64, error) {
	id, err := r.result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get last insert id failed: %w", err)
	}
	return id, nil
}

// RowsAffected returns the number of rows affected
func (r *PostgreSQLResult) RowsAffected() (int64, error) {
	affected, err := r.result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("get rows affected failed: %w", err)
	}
	return affected, nil
}

// PostgreSQLTransaction implements the Transaction interface
type PostgreSQLTransaction struct {
	tx *sql.Tx
}

// Query executes a query within the transaction
func (t *PostgreSQLTransaction) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	rows, err := t.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("transaction query failed: %w", err)
	}
	return &PostgreSQLRows{rows: rows}, nil
}

// QueryRow executes a query that returns at most one row
func (t *PostgreSQLTransaction) QueryRow(ctx context.Context, query string, args ...interface{}) Row {
	return &PostgreSQLRow{row: t.tx.QueryRowContext(ctx, query, args...)}
}

// Exec executes a query within the transaction
func (t *PostgreSQLTransaction) Exec(ctx context.Context, query string, args ...interface{}) (Result, error) {
	result, err := t.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("transaction exec failed: %w", err)
	}
	return &PostgreSQLResult{result: result}, nil
}

// Prepare creates a prepared statement within the transaction
func (t *PostgreSQLTransaction) Prepare(ctx context.Context, query string) (Stmt, error) {
	stmt, err := t.tx.PrepareContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("transaction prepare failed: %w", err)
	}
	return &PostgreSQLStmt{stmt: stmt}, nil
}

// Commit commits the transaction
func (t *PostgreSQLTransaction) Commit() error {
	if err := t.tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}
	return nil
}

// Rollback rolls back the transaction
func (t *PostgreSQLTransaction) Rollback() error {
	if err := t.tx.Rollback(); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}
	return nil
}

// PostgreSQLStmt implements the Stmt interface
type PostgreSQLStmt struct {
	stmt *sql.Stmt
}

// Exec executes a prepared statement
func (s *PostgreSQLStmt) Exec(ctx context.Context, args ...interface{}) (Result, error) {
	result, err := s.stmt.ExecContext(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("statement exec failed: %w", err)
	}
	return &PostgreSQLResult{result: result}, nil
}

// Query executes a prepared query statement
func (s *PostgreSQLStmt) Query(ctx context.Context, args ...interface{}) (Rows, error) {
	rows, err := s.stmt.QueryContext(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("statement query failed: %w", err)
	}
	return &PostgreSQLRows{rows: rows}, nil
}

// QueryRow executes a prepared query that returns at most one row
func (s *PostgreSQLStmt) QueryRow(ctx context.Context, args ...interface{}) Row {
	return &PostgreSQLRow{row: s.stmt.QueryRowContext(ctx, args...)}
}

// Close closes the statement
func (s *PostgreSQLStmt) Close() error {
	if err := s.stmt.Close(); err != nil {
		return fmt.Errorf("close statement failed: %w", err)
	}
	return nil
}

// PostgreSQLColumnType implements the ColumnType interface
type PostgreSQLColumnType struct {
	ct *sql.ColumnType
}

// Name returns the column name
func (c *PostgreSQLColumnType) Name() string {
	return c.ct.Name()
}

// DatabaseTypeName returns the database system type name
func (c *PostgreSQLColumnType) DatabaseTypeName() string {
	return c.ct.DatabaseTypeName()
}

// Length returns the column length
func (c *PostgreSQLColumnType) Length() (int64, bool) {
	return c.ct.Length()
}

// Nullable returns whether the column may be null
func (c *PostgreSQLColumnType) Nullable() (bool, bool) {
	return c.ct.Nullable()
}

// DecimalSize returns the scale and precision
func (c *PostgreSQLColumnType) DecimalSize() (int64, int64, bool) {
	return c.ct.DecimalSize()
}

// ScanType returns a Go type suitable for scanning
func (c *PostgreSQLColumnType) ScanType() interface{} {
	return c.ct.ScanType()
}
